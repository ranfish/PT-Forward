package api

// §59.539 禁转标记定时复检——标记跟随体系（用户定案）：
//
// 核心语义「标记优先·忠实跟随站点标记」：
//   复检标记消失 → 直接放行清 flags（不看 until——站方标记唯一权威）
//   标记还在+until 已过（站方延长窗）→ 滚动续期 until=now+24h（每轮复检在则续）
//   标记变化 → 跟随站方（限时→永久如实更新）
//   失败（网络/CF/404/数据不足）→ 原样·不升级不降级·下周期再来
//   until 降级为「两次复检之间的列表缓存判定」（§59.162 原语义保留）
//
// 站点分层路径：
//   keepfrds=列表行路径（GetTorrentDetail 内 enrichKeepfrdsTransferFlags 天然
//     带 RawListRow+AddedAtText → ClassifyNoTransfer 重判——与采集链同源同判据；
//     RawListRow 空=数据不足=fail 保持——防详情页无标记误判消失）
//   其它站=通用详情页路径（ExtractFlagsFromText(title+subtitle+tags+description)
//     ——与采集 extractFlags 同域两层检测）
//
// 复检对象：flags 含禁转族七词（禁转/限时禁转/限转/谢绝转载/独占/严禁转载/
// 谢决搬运）；分集/adult 系统级禁发不进名单不触碰（检了也不能转·名单+回写
// 子集替换双重保险）。回写=子集替换（只动七词·保留其它 flags）。

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"github.com/robfig/cron/v3"
)

// noTransferForbiddenWords 禁转族八词（§59.539 定案+回归审核 P7 补
// 「禁止转载」——extract 词表全集对齐去分集；注意「禁止转载」不含相邻
// 「禁转」子串·子串匹配兜不住必须显式列名·否则该 flag 种子永卡禁转列表
// 无复检——与限转原缺口同型）。
var noTransferForbiddenWords = []string{
	"禁转", "禁止转载", "限时禁转", "限转", "谢绝转载", "独占", "严禁转载", "谢决搬运",
}

// isNoTransferWord 是否禁转族词（含子串匹配——flags 里存的是精确词，
// 子串防御「禁止转载」等变体漏网）。
func isNoTransferWord(f string) bool {
	for _, w := range noTransferForbiddenWords {
		if f == w || strings.Contains(f, w) || strings.Contains(w, f) {
			return true
		}
	}
	return false
}

// noTransferRecheckState 任务状态（running 防重入·无停止按钮——量小自然完成）。
type noTransferRecheckState struct {
	mu        sync.Mutex
	running   bool
	lastRun   string
	lastStats string
}

// noTransferRecheckConfig 读 publish_settings 两键（默认关+cron）。
func (h *PublishTorrentsHandler) noTransferRecheckConfig(ctx context.Context) (bool, string) {
	enabled := false
	schedule := ""
	var rows []model.PublishSetting
	h.db.WithContext(ctx).Where("key IN ?", []string{"no_transfer_recheck_enabled", "no_transfer_recheck_schedule"}).Find(&rows)
	for _, r := range rows {
		switch r.Key {
		case "no_transfer_recheck_enabled":
			enabled = r.Value == "true" || r.Value == "1"
		case "no_transfer_recheck_schedule":
			schedule = r.Value
		}
	}
	return enabled, schedule
}

// RunNoTransferRecheck 任务主体：全量禁转族行逐颗复检（定时+手动共用）。
func (h *PublishTorrentsHandler) RunNoTransferRecheck(ctx context.Context) {
	h.noTransferRecheck.mu.Lock()
	if h.noTransferRecheck.running {
		h.noTransferRecheck.mu.Unlock()
		h.logger.Info("no-transfer recheck already running")
		return
	}
	h.noTransferRecheck.running = true
	h.noTransferRecheck.mu.Unlock()
	defer func() {
		if p := recover(); p != nil {
			h.logger.Error("no-transfer recheck panic", zap.Any("panic", p))
		}
		h.noTransferRecheck.mu.Lock()
		h.noTransferRecheck.running = false
		h.noTransferRecheck.mu.Unlock()
	}()

	// 查禁转族行（LIKE 超集——flags JSON 数组串含任一禁转词；子集替换回写
	// 兜底分集/adult 不被触碰）。
	var metas []model.TorrentMetadata
	h.db.WithContext(ctx).
		Where("flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ?",
			`%"禁转"%`, `%"禁止转载"%`, `%"限时禁转"%`, `%"限转"%`, `%"谢决转载"%`, `%"独占"%`, `%"严禁转载"%`, `%"谢决搬运"%`).
		Where("torrent_id != ''").
		Find(&metas)

	cleared, kept, renewed, changed, failed := 0, 0, 0, 0, 0
	// §59.542 追记十一: (site,tid) 去重——同簇多行常共享同一站方 tid
	// （243 实证：16 hash 行同 tid 2782403）·逐行独立请求=重复拉详情+
	// 限流风险。同组只判定一次·结果逐行应用。
	type groupKey struct{ site, tid string }
	type resolveResult struct {
		forbidden []string
		until     *time.Time
		ok        bool
	}
	resolved := make(map[groupKey]*resolveResult)
	for i := range metas {
		select {
		case <-ctx.Done():
			h.logger.Info("no-transfer recheck ctx done, stop", zap.Int("processed", i))
			goto done
		default:
		}
		key := groupKey{metas[i].SiteName, metas[i].TorrentID}
		rr, seen := resolved[key]
		if !seen {
			nf, nu, ok := h.resolveNoTransferState(ctx, metas[i].SiteName, metas[i].TorrentID)
			rr = &resolveResult{forbidden: nf, until: nu, ok: ok}
			resolved[key] = rr
		}
		if !rr.ok {
			failed++
			continue
		}
		switch h.applyNoTransferResult(ctx, &metas[i], rr.forbidden, rr.until) {
		case "cleared":
			cleared++
		case "kept":
			kept++
		case "renewed":
			renewed++
		case "changed":
			changed++
		default:
			failed++
		}
	}
done:
	// §59.542 追记十一（用户定案：复检与簇脱钩）: 复检对象=源站+该种子
	// （站方对某 tid 的标记）·簇副本行不是独立复检对象——同 (site,tid)
	// 一组一次判定·组内行统一应用；计数按组（16 副本行=「检测 1 颗」）。
	stats := time.Now().Format("01-02 15:04:05") + " 检测 " + itoa(len(resolved)) +
		" 颗（行 " + itoa(len(metas)) + "）：放行 " + itoa(cleared) + " · 保留 " + itoa(kept) +
		" · 续期 " + itoa(renewed) + " · 变化 " + itoa(changed) + " · 失败 " + itoa(failed)
	h.noTransferRecheck.mu.Lock()
	h.noTransferRecheck.lastRun = time.Now().Format("2006-01-02 15:04:05")
	h.noTransferRecheck.lastStats = stats
	h.noTransferRecheck.mu.Unlock()
	// §59.196 附二教训：任务终态必落日志（前端轮询假象防线）。
	h.logger.Info("no-transfer recheck finished",
		zap.Int("total", len(metas)), zap.Int("cleared", cleared), zap.Int("kept", kept),
		zap.Int("renewed", renewed), zap.Int("changed", changed), zap.Int("failed", failed))
}

// resolveNoTransferState 网络判定段（一组一次）：站名→域名解析 + 拉详情 +
// 站方当前禁转集提取。返回 (禁转集, until, ok)。
// §59.542 追记十一根因修复：meta.SiteName 是站名（「朋友」）而
// GetSiteConfig/GetAdapter 按 domain 查——原实现直接传站名必然
// GetByDomain miss（243 实证：16/16 全 failed·「站点不存在: 朋友」·
// 15 秒快速失败零请求）。此处先经 sites 表解析站名→domain。
func (h *PublishTorrentsHandler) resolveNoTransferState(ctx context.Context, siteName, torrentID string) ([]string, *time.Time, bool) {
	if h.siteProvider == nil {
		h.logger.Debug("no-transfer recheck skip: site provider nil")
		return nil, nil, false
	}
	var site model.Site
	if err := h.db.WithContext(ctx).Where("name = ?", siteName).First(&site).Error; err != nil {
		h.logger.Debug("no-transfer recheck: site name not found",
			zap.String("site", siteName), zap.Error(err))
		return nil, nil, false
	}
	config, err := h.siteProvider.GetSiteConfig(ctx, site.Domain)
	if err != nil || config == nil {
		h.logger.Debug("no-transfer recheck: get site config failed",
			zap.String("site", siteName), zap.String("domain", site.Domain), zap.Error(err))
		return nil, nil, false
	}
	adapter, err := h.siteProvider.GetAdapter(ctx, site.Domain)
	if err != nil || adapter == nil {
		h.logger.Debug("no-transfer recheck: get adapter failed",
			zap.String("site", siteName), zap.String("domain", site.Domain), zap.Error(err))
		return nil, nil, false
	}
	detail, err := adapter.GetTorrentDetail(ctx, config, torrentID)
	if err != nil || detail == nil {
		h.logger.Debug("no-transfer recheck: get detail failed",
			zap.String("site", siteName), zap.String("tid", torrentID), zap.Error(err))
		return nil, nil, false
	}
	now := time.Now()
	var newForbidden []string
	var newUntil *time.Time
	if strings.Contains(config.Domain, "keepfrds") {
		// 列表行路径：RawListRow 空=数据不足（列表抓取失败）→ fail 保持
		// （防详情页无标记误判消失——§59.539 站点分层定案）
		if detail.RawListRow == "" {
			h.logger.Debug("no-transfer recheck: keepfrds raw list row empty",
				zap.String("site", siteName), zap.String("tid", torrentID))
			return nil, nil, false
		}
		limited, permanent, until := metadata.ClassifyNoTransfer(detail.RawListRow, detail.AddedAtText, now)
		switch {
		case limited:
			newForbidden = []string{"限时禁转"}
			u := until
			newUntil = &u
		case permanent:
			newForbidden = []string{"禁转"}
		}
	} else {
		// 通用路径：与采集 extractFlags 同域（title+subtitle+tags+description）
		text := detail.Title + " " + detail.Subtitle + " " + strings.Join(detail.Tags, " ") + " " + detail.Description
		for _, f := range extract.ExtractFlagsFromText(text) {
			if isNoTransferWord(f) {
				newForbidden = append(newForbidden, f)
			}
		}
	}
	return newForbidden, newUntil, true
}

// applyNoTransferResult 本地对比回写段（一行一次）：滚动续期 + 动作判定 +
// 子集替换回写。返回 cleared/kept/renewed/changed/failed。
func (h *PublishTorrentsHandler) applyNoTransferResult(ctx context.Context, meta *model.TorrentMetadata, newForbidden []string, newUntil *time.Time) string {
	var oldFlags []string
	if meta.Flags != "" {
		_ = json.Unmarshal([]byte(meta.Flags), &oldFlags)
	}
	// 保留集=非禁转族（分集/adult 等·子集替换保护）
	keep := make([]string, 0, len(oldFlags))
	for _, f := range oldFlags {
		if !isNoTransferWord(f) {
			keep = append(keep, f)
		}
	}

	// 滚动续期：限时禁转在 + until 缺失/已过（站方延长窗）→ now+24h
	// （ClassifyNoTransfer 公式重算不出延长窗——发布时间不变重算恒过去；
	// 每轮复检标记在就续 24h·站方摘标记后下轮自动放——§59.539 定案 A）
	hasLimited := false
	for _, f := range newForbidden {
		if f == "限时禁转" {
			hasLimited = true
			break
		}
	}
	action := "kept"
	if hasLimited {
		if newUntil == nil || newUntil.Before(time.Now()) {
			u := time.Now().Add(24 * time.Hour)
			newUntil = &u
			action = "renewed"
		}
	} else {
		newUntil = nil
	}

	// 组装新 flags（去重）
	seen := map[string]bool{}
	newFlags := make([]string, 0, len(keep)+len(newForbidden))
	for _, f := range keep {
		if !seen[f] {
			seen[f] = true
			newFlags = append(newFlags, f)
		}
	}
	for _, f := range newForbidden {
		if !seen[f] {
			seen[f] = true
			newFlags = append(newFlags, f)
		}
	}

	// 对比旧禁转集判定动作
	oldForbidden := make([]string, 0, len(oldFlags))
	for _, f := range oldFlags {
		if isNoTransferWord(f) {
			oldForbidden = append(oldForbidden, f)
		}
	}
	if !strSlicesEqual(oldForbidden, newForbidden) && len(newForbidden) == 0 {
		action = "cleared" // 站方标记消失→直接放行（不看 until·标记优先）
	} else if !strSlicesEqual(oldForbidden, newForbidden) {
		action = "changed" // 标记变化（如限时→永久）跟随站方
	}
	if action == "kept" && !untilEqual(meta.NoTransferUntil, newUntil) {
		action = "renewed" // 集合没变但 until 变了（滚动续期已置）
	}

	// 回写（子集替换：keep+newForbidden；until 同步）
	flagJSON, _ := json.Marshal(newFlags)
	updates := map[string]interface{}{"flags": string(flagJSON)}
	if newUntil != nil {
		updates["no_transfer_until"] = *newUntil
	} else {
		updates["no_transfer_until"] = nil
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
		Updates(updates).Error; err != nil {
		h.logger.Warn("no-transfer recheck write failed",
			zap.String("hash", safeHashPrefix(meta.InfoHash)), zap.Error(err))
		return "failed"
	}
	if action != "kept" {
		h.logger.Info("no-transfer recheck "+action,
			zap.String("site", meta.SiteName), zap.String("hash", safeHashPrefix(meta.InfoHash)),
			zap.Strings("old", oldForbidden), zap.Strings("new", newForbidden))
	}
	return action
}

func strSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ca := append([]string(nil), a...)
	cb := append([]string(nil), b...)
	for i := 0; i < len(ca); i++ {
		for j := i + 1; j < len(ca); j++ {
			if ca[i] > ca[j] {
				ca[i], ca[j] = ca[j], ca[i]
			}
			if cb[i] > cb[j] {
				cb[i], cb[j] = cb[j], cb[i]
			}
		}
	}
	for i := range ca {
		if ca[i] != cb[i] {
			return false
		}
	}
	return true
}

func untilEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func safeHashPrefix(h string) string {
	if len(h) > 10 {
		return h[:10]
	}
	return h
}

// NoTransferRecheckTaskConfig §59.539: main 注册入口（读配置）。
func (h *PublishTorrentsHandler) NoTransferRecheckTaskConfig(ctx context.Context) (bool, string) {
	return h.noTransferRecheckConfig(ctx)
}

// SetNoTransferRecheckSync §59.539: 配置变更即时重注册回调注入（main）。
func (h *PublishTorrentsHandler) SetNoTransferRecheckSync(f func()) { h.noTransferRecheckSync = f }

// handleNoTransferRecheckRun 手动触发（幂等：running 中直接返回）。
func (h *PublishTorrentsHandler) handleNoTransferRecheckRun(w http.ResponseWriter, r *http.Request) {
	h.noTransferRecheck.mu.Lock()
	running := h.noTransferRecheck.running
	h.noTransferRecheck.mu.Unlock()
	if running {
		Success(w, map[string]interface{}{"started": false, "running": true})
		return
	}
	go h.RunNoTransferRecheck(context.Background())
	Success(w, map[string]interface{}{"started": true, "running": true})
}

// handleNoTransferRecheckStatus 状态（GET）+配置（含空表默认分支——§59.536
// P1 教训：回读必须含全部字段防保存不生效假象）。
func (h *PublishTorrentsHandler) handleNoTransferRecheckStatus(w http.ResponseWriter, r *http.Request) {
	enabled, schedule := h.noTransferRecheckConfig(r.Context())
	h.noTransferRecheck.mu.Lock()
	defer h.noTransferRecheck.mu.Unlock()
	Success(w, map[string]interface{}{
		"running":     h.noTransferRecheck.running,
		"last_run":    h.noTransferRecheck.lastRun,
		"last_stats":  h.noTransferRecheck.lastStats,
		"enabled":     enabled,
		"schedule":    schedule,
	})
}

// handleNoTransferRecheckConfigSave 保存配置（PUT）+即时重注册 cron。
func (h *PublishTorrentsHandler) handleNoTransferRecheckConfigSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled  bool   `json:"enabled"`
		Schedule string `json:"schedule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Enabled && req.Schedule == "" {
		Error(w, http.StatusBadRequest, 40002, "启用时调度周期必填")
		return
	}
	if req.Schedule != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(req.Schedule); err != nil {
			Error(w, http.StatusBadRequest, 40003, "cron 表达式无效: "+err.Error())
			return
		}
	}
	ctx := r.Context()
	save := func(k, v string) {
		h.db.WithContext(ctx).Where("key = ?", k).Delete(&model.PublishSetting{})
		h.db.WithContext(ctx).Create(&model.PublishSetting{Key: k, Value: v})
	}
	save("no_transfer_recheck_enabled", boolStr(req.Enabled))
	save("no_transfer_recheck_schedule", req.Schedule)
	// 即时重注册（配置驱动——对齐 IYUU syncCoverageSchedule 模式）
	if h.noTransferRecheckSync != nil {
		go h.noTransferRecheckSync()
	}
	Success(w, map[string]interface{}{"saved": true})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
