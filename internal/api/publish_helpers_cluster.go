package api

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/site"
)

// buildClusterTargets §59.61: 簇副本 comment → 直达候选。
// 数据源管道 b（snapshots.comment，syncer 10min 同步）；tracker 首域取自本副本
// 下载器实时数据不可得时回退缩省形态（仅 URL/ob_tid/HDHx 方言，纯数字方言依赖
// tracker 上下文自动跳过——降级语义安全）。
func (h *PublishTorrentsHandler) buildClusterTargets(ctx context.Context, clientID, savePath, name, selfHash string) []comment.DirectTarget {
	if h.db == nil || clientID == "" || savePath == "" || name == "" {
		return nil
	}
	members := publish.GetClusterComments(ctx, h.db, clientID, savePath, name)
	if len(members) == 0 {
		return nil
	}
	// 域名解析器：全量站点四列域名（domain/alt/tracker/base_url）
	var sites []model.Site
	h.db.WithContext(ctx).Find(&sites)
	resolver := publish.HostResolverFromSites(sites)
	// tracker 首域：本副本 hash 的 TrackerMatcher 解析（快照无 tracker 列，
	// 用覆盖缓存 tracker 来源域近似——仅纯数字/相对路径方言需要，缺失则跳过）
	trackerHost := h.lookupTrackerHost(ctx, selfHash)
	hosts := map[string]string{selfHash: trackerHost}
	return publish.BuildClusterTargets(members, resolver, hosts)
}

// lookupTrackerHost 从覆盖缓存取该 hash 已知 tracker 域（近似上下文）。
func (h *PublishTorrentsHandler) lookupTrackerHost(ctx context.Context, hash string) string {
	if h.coverage == nil {
		return ""
	}
	cs, err := h.coverage.GetCachedCoverage(ctx, hash)
	if err != nil {
		return ""
	}
	for _, c := range cs {
		if c.Source == model.CoverageSourceTracker && c.SiteName != "" {
			var s model.Site
			if err := h.db.WithContext(ctx).Where("name = ?", c.SiteName).First(&s).Error; err == nil {
				return s.Domain
			}
		}
	}
	return ""
}

var _ = site.NewTrackerMatcher // 保持 site 包引用（HostResolverFromSites 内部使用）

// §59.61 第 4 步: 簇共享三方法——批内跳过 / 元数据传播 / 截图二次传播

// hasCompleteMetadata 该 hash 是否已有完整数据行（poster/description 任一非空）
// ——runBatchFetch 批内跳过判据（簇内先前副本已获取或传播）。
func (h *PublishTorrentsHandler) hasCompleteMetadata(ctx context.Context, hash string) bool {
	if hash == "" {
		return false
	}
	var n int64
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND (poster != '' OR description != '')", hash).
		Count(&n)
	return n > 0
}

// propagateClusterReviewed §59.93: 审核状态簇同步——PUT reviewed 写行后把
// 簇内全部行同步（同簇数据相同，审核一份数据=审核全簇）。仙履奇缘 54 副本实锤：
// 编辑行 reviewed=1 其余 53 行=0，列表逐快照行展示时其他快照挂的兄弟行仍 pending。
func (h *PublishTorrentsHandler) propagateClusterReviewed(ctx context.Context, clientID, savePath, name, selfHash string, reviewed bool) {
	if h.db == nil || clientID == "" || savePath == "" || name == "" {
		return
	}
	var siblingHashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = 0 AND hash != ?",
			clientID, savePath, name, selfHash).
		Pluck("hash", &siblingHashes)
	if len(siblingHashes) == 0 {
		return
	}
	res := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ?", siblingHashes).
		Update("reviewed", reviewed)
	if res.Error != nil {
		h.logger.Error("cluster reviewed propagate failed", zap.Error(res.Error))
		return
	}
	if res.RowsAffected > 0 {
		h.logger.Info("cluster reviewed propagated",
			zap.String("hash", selfHash[:min(10, len(selfHash))]),
			zap.Bool("reviewed", reviewed),
			zap.Int64("rows", res.RowsAffected))
	}
}

// upsertClusterScreenshotCache §59.63: 截图链接缓存写穿（自动链成功点 + 手动捕获
// 完成点共用）。ttlDays<=0 视为功能关闭。UPSERT——同簇双 leader 后写赢（第二个
// 本就是缓存命中，链接一致）。
func upsertClusterScreenshotCache(db *gorm.DB, logger *zap.Logger, ttlDays int, clientID, savePath, name string, shots []string) {
	if db == nil || logger == nil || ttlDays <= 0 || clientID == "" || savePath == "" || name == "" || len(shots) == 0 {
		return
	}
	data, err := json.Marshal(shots)
	if err != nil {
		return
	}
	row := model.ClusterScreenshotCache{
		ClientID: clientID, SavePath: savePath, Name: name,
		Screenshots: string(data), UpdatedAt: time.Now(),
	}
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_id"}, {Name: "save_path"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"screenshots", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		logger.Warn("screenshot cache upsert failed", zap.Error(err))
	}
}

// lookupScreenshotCache §59.63: 观察期判定（惰性，Q5——无后台扫描，获取事件时判）。
// 命中且 updated_at + ttlDays > now → 返回缓存链接；否则 miss。
func (h *PublishTorrentsHandler) lookupScreenshotCache(clientID, savePath, name string) ([]string, bool) {
	if h.db == nil || h.screenshotCacheDays <= 0 {
		return nil, false
	}
	var row model.ClusterScreenshotCache
	if err := h.db.Where("client_id = ? AND save_path = ? AND name = ?", clientID, savePath, name).
		First(&row).Error; err != nil {
		return nil, false
	}
	if time.Since(row.UpdatedAt) > time.Duration(h.screenshotCacheDays)*24*time.Hour {
		return nil, false
	}
	shots := model.ParseScreenshotColumn(row.Screenshots)
	if len(shots) == 0 {
		return nil, false
	}
	return shots, true
}

// finalizeClusterPropagation §59.61 附5: fetch 尾部终局传播。竞态实锤（疯狂动物城2
// BluRay 27/54 行残留站点态）: propagateClusterMetadata 的 INSERT 循环（54 行/178ms，
// src 只读一次）与异步 applyPosterFallback 的回传 UPDATE 并发——循环尾段 INSERT 的行
// 携带过期站点态落地，"已有行不覆盖"语义保证无人再修。修复: 等 fallback 终局后
// INSERT（携带终态），再显式回传一次（幂等兜底，覆盖既有传播行）。
func (h *PublishTorrentsHandler) finalizeClusterPropagation(ctx context.Context, fallbackWg *sync.WaitGroup, clientID, savePath, name, hash, siteName string) {
	if fallbackWg != nil {
		fallbackWg.Wait()
	}
	h.propagateClusterMetadata(ctx, clientID, savePath, name, hash, siteName)
	h.propagateClusterPosters(ctx, clientID, savePath, name, hash)
}

// propagateClusterMetadata 获取成功后（fetchSingleTorrent 末尾）：站点侧数据 + MI
// 复制到簇内无数据副本（已有行不覆盖——尊重显式获取/编辑）。fetch_source=cluster 审计可辨。
func (h *PublishTorrentsHandler) propagateClusterMetadata(ctx context.Context, clientID, savePath, name, selfHash, siteName string) {
	if h.db == nil || clientID == "" || savePath == "" || name == "" {
		return
	}
	// 源行（本站数据最全——TechProfile/tags/MI 均已后处理完）
	var src model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", selfHash, siteName).
		First(&src).Error; err != nil {
		return
	}
	// 簇内缺行副本
	var siblingHashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = 0 AND hash != ?",
			clientID, savePath, name, selfHash).
		Pluck("hash", &siblingHashes)
	if len(siblingHashes) == 0 {
		return
	}
	var have []string
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ?", siblingHashes).Pluck("info_hash", &have)
	haveSet := map[string]bool{}
	for _, hh := range have {
		haveSet[hh] = true
	}
	n := 0
	for _, hh := range siblingHashes {
		if haveSet[hh] {
			continue
		}
		cpy := src
		cpy.ID = 0
		cpy.InfoHash = hh
		cpy.FetchSource = "cluster"
		if err := h.db.WithContext(ctx).Create(&cpy).Error; err == nil {
			n++
		}
	}
	if n > 0 {
		h.logger.Info("cluster metadata propagated",
			zap.String("hash", selfHash[:min(10, len(selfHash))]),
			zap.Int("replicas", n))
	}
}

// propagateClusterScreenshots 截图策略完成后：补簇内空截图行
// （策略是分钟级异步任务，元数据传播时截图未就绪——此为二次传播）。
func (h *PublishTorrentsHandler) propagateClusterScreenshots(ctx context.Context, clientID, savePath, name, selfHash, screenshotsJSON string) {
	if h.db == nil || clientID == "" || savePath == "" || name == "" || screenshotsJSON == "" || screenshotsJSON == "[]" {
		return
	}
	var siblingHashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = 0 AND hash != ?",
			clientID, savePath, name, selfHash).
		Pluck("hash", &siblingHashes)
	if len(siblingHashes) == 0 {
		return
	}
	// §59.61 附: 覆盖条件含 cluster 行——站内 2 张部分截图须被策略终态（5 张）覆盖
	// （原仅补空行——243 墓碑镇实锤：簇行残留部分截图不回传）
	res := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ? AND (screenshots = '' OR screenshots = '[]' OR fetch_source = 'cluster')", siblingHashes).
		Update("screenshots", screenshotsJSON)
	if res.RowsAffected > 0 {
		h.logger.Info("cluster screenshots propagated",
			zap.String("hash", selfHash[:min(10, len(selfHash))]),
			zap.Int64("rows", res.RowsAffected))
	}
}

// propagateClusterPosters §59.61 附（传播竞态修复）：异步修复（PTGen 海报/简介）
// 完成后把首副本终态回传簇行——仅覆盖 fetch_source='cluster' 的传播行
// （独立获取行 rss_detail 不动，尊重显式数据）。243 墓碑镇 57 副本裂图根因：
// t1 传播不完整态 → t2 PTGen 只修首副本 → 簇行永久残缺。
func (h *PublishTorrentsHandler) propagateClusterPosters(ctx context.Context, clientID, savePath, name, selfHash string) {
	if h.db == nil || clientID == "" || savePath == "" || name == "" {
		return
	}
	var src model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ?", selfHash).First(&src).Error; err != nil {
		return
	}
	if src.Poster == "" && src.Description == "" {
		return
	}
	var siblingHashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = 0 AND hash != ?",
			clientID, savePath, name, selfHash).
		Pluck("hash", &siblingHashes)
	if len(siblingHashes) == 0 {
		return
	}
	updates := map[string]interface{}{}
	if src.Poster != "" {
		updates["poster"] = src.Poster
	}
	if src.Description != "" {
		updates["description"] = src.Description
	}
	// 仅传播行（cluster）——首副本的致谢声明等后处理也会带上（簇共享语义）
	if src.Statement != "" {
		updates["statement"] = src.Statement
	}
	res := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ? AND fetch_source = 'cluster'", siblingHashes).
		Updates(updates)
	if res.Error != nil {
		h.logger.Error("cluster poster propagate failed", zap.Error(res.Error))
		return
	}
	if res.RowsAffected > 0 {
		h.logger.Info("cluster posters propagated",
			zap.String("hash", selfHash[:min(10, len(selfHash))]),
			zap.Int64("rows", res.RowsAffected))
	}
}
