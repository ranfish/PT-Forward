// Package publish 新发布执行器（§59.156 切片 2，2026-08-31）。
//
// §59.147 配置中心消费流：发布链 DB 优先——切读 sites.publish_form_config，
// 替代 §59.146 灰度 flag（tagConfigFromFlag/settings tag_applier_sites 随本切片退役）。
// 复用组件（§59.148 保留清单）：renderDescription / dedupByPiecesHash /
// CheckPublishEligibility / adapter UploadTorrent / pusher 加种链 / ResourceResolver。
//
// 铁律对齐：零拉取（源 .torrent 走下载器 ExportTorrent 本地导出；元数据走
// torrent_metadata 落库数据 + PTGen 缓存兜底）；pre-audit=推数据拿判定（发布动作前置组成）。
package publish

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/ptgen"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// joinNotes 多段备注拼接（预检摘要+结果消息）。
func joinNotes(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "；" + b
	}
}

// TechTextOf §59.319 P2: 原盘 tech 文本源统一——BDInfo 优先，普通种子
// MediaInfo（SourceMediaInfo 兜底由调用方语境决定，发布链只消费本地列——
// 与 FetchAndStore 落库语义一致）。表单 techinfo 域/BuildTechProfile/标签
// 推断/审计 body/描述 quote 五消费点同源。ExtractMediaInfo 格式分流
// （QUICK SUMMARY 锚）保证 BDInfo 文本自动走 BD 解析器。
// §59.324: 源站采集的 BDInfo/MI 文本常裹 [code] BBCode 块（True.Grit
// MinBD 案——站方 MEDIAINFO_CONTAINS_BBCODE 拒收）——发布链输出剥
// code 标记对（描述域的 [code] 是合法 BBCode 不受影响，仅此处剥）。
func TechTextOf(meta *model.TorrentMetadata) string {
	raw := ""
	if meta.BDInfo != "" {
		raw = meta.BDInfo
	} else {
		raw = meta.MediaInfo
	}
	if strings.Contains(raw, "[code]") || strings.Contains(raw, "[/code]") {
		raw = strings.ReplaceAll(raw, "[code]", "")
		raw = strings.ReplaceAll(raw, "[/code]", "")
	}
	return raw
}

// parseScreenshotsCol screenshots 列解析（§59.47 四消费点同款）。
// §59.159 修复：列存储形态为 JSON 数组（["url1","url2"]）——FieldsFunc 硬拆
// 会保留引号 → [img]"url"[/img] 坏格式 → LuckAudit 审核拒绝（实战 86 分 1 错误）。
// JSON 解析优先，剥引号兜底。
func parseScreenshotsCol(s string) []string {
	if s == "" {
		return nil
	}
	var urls []string
	if err := json.Unmarshal([]byte(s), &urls); err == nil {
		return urls
	}
	// fallback：逗号/换行分隔 + 剥引号
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "\"")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// chineseTitleOf 提取标题中文段（§59.164 修道院 cnname——"阴风阵阵.Suspiria.2018"→"阴风阵阵"）。
// 取最长连续中文段（含·间隔与内部空格）；无中文返回空（字段不投递）。
var chineseTitleRe = regexp.MustCompile(`\p{Han}+(?:[\s·・]\p{Han}+)*`)

// §59.399: IMDb/豆瓣编号提取（学校站 imdbid/doubanid 编号形态字段）。
// IMDb ID=tt+数字（URL 任意位置匹配——尾部锚定不必要，tt 前缀即无歧义锚）；
// 豆瓣 ID=subject/ 后数字（路径锚定防误匹配非条目页）。DB 值已是纯编号
// 形态（Tab4 用户手输 §59.341）直出兼容；形态不匹配返回空串（setForm
// skip 不填——站方非必填不阻断）。
var reIMDbID = regexp.MustCompile(`tt[0-9]+`)
var reDoubanID = regexp.MustCompile(`subject/([0-9]+)`)

func extractIMDbID(url string) string {
	url = strings.TrimSpace(url)
	if m := reIMDbID.FindString(url); m != "" {
		return m
	}
	return ""
}

func extractDoubanID(url string) string {
	url = strings.TrimSpace(url)
	if m := reDoubanID.FindStringSubmatch(url); len(m) > 1 {
		return m[1]
	}
	// 纯数字直出（用户手输纯编号形态——无 URL 壳）
	if url != "" {
		for _, r := range url {
			if r < '0' || r > '9' {
				return ""
			}
		}
		return url
	}
	return ""
}

func chineseTitleOf(title string) string {
	best := ""
	for _, seg := range chineseTitleRe.FindAllString(title, -1) {
		seg = strings.TrimSpace(seg)
		if len([]rune(seg)) > len([]rune(best)) {
			best = seg
		}
	}
	return best
}

// fingerprintPiecesHash 种子数据 pieces hash（dedup 本地记忆键——空数据/解析失败返回空）。
func fingerprintPiecesHash(torrentData []byte) string {
	if len(torrentData) == 0 {
		return ""
	}
	if m, err := fingerprint.ComputeFromTorrent(torrentData); err == nil && m != nil {
		return m.PiecesHash
	}
	return ""
}

// detailURLOf 站内种详情 URL（拦截记录直达链接——tid 空返回空）。
func detailURLOf(cfg *model.SiteConfig, tid string) string {
	if tid == "" {
		return ""
	}
	// §59.491: 协议根修——裸 Domain 拼接产出无协议 URL（publish_url 落库
	// "www.musopia.vip/..."）·前端 href 相对路径拼系统地址。PT 站全 https·
	// 已带协议的 Domain 原样（防御双形态）。
	d := strings.TrimRight(cfg.Domain, "/")
	if !strings.HasPrefix(d, "http://") && !strings.HasPrefix(d, "https://") {
		d = "https://" + d
	}
	return d + "/details.php?id=" + tid
}

// preAuditTitle §59.166 B2 补：预检 name 与上传标题同源（tp 重组 MI 纠错终态
// ——Arco 案预检层旧标题与表单 DDP 对照错位残留）。
// §59.254 项4：重组空返回 ok=false（DryRun 同拒——预检正是发现问题的工具）。
func (e *PublishExecutor) preAuditTitle(meta *model.TorrentMetadata) (string, bool) {
	// §59.514 附八: 优先消费重组标题资产列（§59.304 双链分叉防——主管线
	// 锚定列·兜底链同锚定·列空才降级现算）
	if meta.ReassembledTitle != "" {
		return meta.ReassembledTitle, true
	}
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(meta.DetailSourceJSON)
	tp := titleparser.BuildTechProfile(meta.Title, TechTextOf(meta), domMedium, domRes, domVideo, domAudio)
	e.normalizeSeasonPackOf(meta, &tp) // §59.308: 预检 name 与上传标题同归一（双链分叉防）
	if rt := titleparser.ReassembleFromTechProfile(tp, titleparser.V105TitleFormat()); strings.TrimSpace(rt) != "" {
		return rt, true
	}
	return "", false
}

// parseTagsAssets §59.311: meta.Tags JSON 数组 → canonical 键列表（Tab1 ③ 同源）。
func parseTagsAssets(raw string) []string {
	if raw == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	return arr
}

// buildFormLabels §59.310: DryRun 表单组装可读化——字段名（站方 input name）反查
// 域中文名 + 值（站方枚举 value）反查 label（ValueMappings 全域扫）。
// "audiocodec_sel[4]"="12" → "音频编码 · DTS"；文本域（无枚举）只带域名。
func buildFormLabels(cfg *model.PublishFormConfig, form map[string]string) map[string]string {
	if cfg == nil || len(form) == 0 {
		return nil
	}
	// 域中文名（domain → 中文）
	domainNames := map[string]string{
		model.FieldDomainType: "类型", model.FieldDomainStandard: "规格",
		model.FieldDomainCodec: "编码", model.FieldDomainAudiocodec: "音频编码",
		model.FieldDomainTeam: "制作组", model.FieldDomainMedium: "媒介",
		model.FieldDomainSmallDescr: "副标题", model.FieldDomainDescription: "简介",
		model.FieldDomainTechInfo: "技术信息", model.FieldDomainIMDBURL: "IMDb",
		model.FieldDomainPTGen: "豆瓣链接", model.FieldDomainDoubanURL: "豆瓣链接",
		model.FieldDomainIMDbID: "IMDb编号", model.FieldDomainDoubanID: "豆瓣编号",
		model.FieldDomainCNName: "中文名", model.FieldDomainUplver: "匿名发布", model.FieldDomainOffer: "候选发布",
	}
	// 字段名 → 域名（FormFields 反查）
	fieldToDomain := map[string]string{}
	for domain, field := range cfg.FormFields {
		if field != "" {
			fieldToDomain[field] = domain
		}
	}
	out := make(map[string]string, len(form))
	for field, value := range form {
		domain, ok := fieldToDomain[field]
		if !ok {
			// 站方特有字段无域名（如 url_[r]/DryRun [gpw] 前缀预览键）——
			// 跳过不进 formLabels（Form 主体仍原键可见，此处无需重复）
			continue
		}
		dn := domainNames[domain]
		if dn == "" {
			dn = domain
		}
		// 值 → label（下拉域）
		if ms, ok := cfg.ValueMappings[domain]; ok {
			for _, m := range ms {
				if m.Value == value {
					out[field] = dn + " · " + m.Label
					break
				}
			}
		}
		if out[field] == "" {
			out[field] = dn
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// normalizeSeasonPackOf §59.308: 完结季包季集归一单点——A 通道（meta.Tags 含
// complete——副标题"全N集"/源站 tag 提取，完结声明权威在标签域）或 B 通道
// （PTGen episodes=区间末集）命中时 E01 起始集区间归一纯季号（v1.05 单季合集=S01；
// 海的开始案 TAGS_COMPLETE_NO_SERIES_ALLOWED 根治）。主管线与 preAuditTitle 共用。
func (e *PublishExecutor) normalizeSeasonPackOf(meta *model.TorrentMetadata, tp *titleparser.TechProfile) {
	if meta == nil || tp == nil {
		return
	}
	// §59.514 通道 C: 副标「全N集/N集全」并入 tagsComplete 同权（信号集
	// 不对称消除——副标词形此前只在 §59.424 门放行不进归一，Book 型
	// episode_split 误标场景 E01 形态漏网）
	tagsComplete := strings.Contains(meta.Tags, "\"complete\"") || subtitleCompleteSignal(meta.Subtitle)
	ptgenEps := 0
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		if v, perr := strconv.Atoi(strings.TrimSpace(src.Episodes)); perr == nil {
			ptgenEps = v
		}
	}
	tp.SeasonEpisode = titleparser.NormalizeSeasonPack(tp.SeasonEpisode, tagsComplete, ptgenEps)
}

// mergePTGenSource §59.254 项1: 第四源准备与合并（executor 侧）。
// 端点感知链：PTGenSourceJSON 内 aka 启发式（零请求）→ NeedsIMDbFallback
// 触发时 &imdb 二连（e.pipe.ptgen 独享豆影）→ MergePTGenInto。
// EnglishName 同时回写 meta.EnglishTitle（二连触发时权威值覆盖）。
func (e *PublishExecutor) mergePTGenSource(ctx context.Context, tp *titleparser.TechProfile, meta *model.TorrentMetadata) {
	if meta == nil {
		return
	}
	src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON)
	if err != nil || src == nil {
		return // 无 PTGen 数据（增强层缺失回退三源现状）
	}
	englishName := ptgen.ExtractEnglishName(&src.PTGenResult, src.Source)
	if ptgen.NeedsIMDbFallback(tp.MainTitle, englishName) && e.pipe != nil && e.pipe.ptgen != nil && meta.DoubanURL != "" {
		if en, _ := e.pipe.ptgen.QueryIMDbForEnglish(ctx, meta.DoubanURL); en != "" {
			englishName = en
			// 权威值覆盖落库（EnglishTitle 资产）
			if err := e.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ?", meta.InfoHash).
				Update("english_title", en).Error; err != nil {
				e.logger.Warn("english_title persist failed", zap.Error(err))
			}
		}
	}
	// 常规 EnglishTitle 尽力落库（aka 启发式——未触发二连时）
	if englishName != "" && meta.EnglishTitle == "" {
		_ = e.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", meta.InfoHash).
			Update("english_title", englishName).Error
	}
	titleparser.MergePTGenInto(tp, titleparser.PTGenMergeInput{
		EnglishName: englishName,
		Year:        src.Year,
	}, func(msg string, kv ...any) {
		// 可观测日志（info 级——year_corrected/main_title_backfilled/title_mismatch）
		fields := []zap.Field{zap.String("info_hash", meta.InfoHash)}
		for i := 0; i+1 < len(kv); i += 2 {
			if k, ok := kv[i].(string); ok {
				fields = append(fields, zap.Any(k, kv[i+1]))
			}
		}
		e.logger.Info(msg, fields...)
	})
}
