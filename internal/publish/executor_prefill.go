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
	"errors"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// hdcityPrefill §59.454: 城市站（hdcity.city）jobs 段特化注入
// （标签无关项）——两步上传站的表单特化（用户逐点讨论定案）。
//
// ① bigname/small_descr 双填副标题（站方实态——t-68751/t-68754 双行
// 同值实证；空态拼装退化"译名 | MainTitle | Year"——PT 圈副标题
// 共识 0% 场景纯防御）
// ② tag1ing 题材注入（hdcityTag1ingValue——◎类别 行 100% 必有
// （29 库 315/315 实证），财神 csptGenreTagValue 增强版——27 项
// 题材映射表+行内多题材按序取首个命中）
// ③ processing_sel 3D 特化（hdcityProcessingOption——HOU→1/
// HSBS→2/全幅→5 Alt/空→不投递；交织(3)/红蓝(4)无提取词形预览人工）
//
// 标签依赖项（chnize/sub_hard/sub_soft/sub_incl）在 tags 产出后
// hdcityTagDependentFill 处理。
// yemaRegionID §59.459: 野马 region PTGen 产区中文→数字值全表
// （wiki 枚举权威 8 项——大陆/港/台分立/欧洲聚合/其它兜底）。
var yemaRegionID = map[string]string{
	"大陆": "1", "中国大陆": "1", "香港": "2", "中国香港": "2",
	"台湾": "3", "中国台湾": "3", "美国": "4", "日本": "6", "韩国": "7",
	// 欧洲聚合（英法德意西俄等→EU=5）
	"英国": "5", "法国": "5", "德国": "5", "意大利": "5", "西班牙": "5",
	"俄罗斯": "5", "新西兰": "5", "加拿大": "5", "澳大利亚": "5",
}

// yemaPublishInject §59.459-②③: 野马 attributes 数据源注入——
// regionList（PTGen 产区合拍多产区全投 ≤3——野马数组语义比肉丝单选
// 表达力强；miss 不投）+translated（中文译名 shortDesc 兜底）+poster。
func yemaPublishInject(form map[string]string, meta *model.TorrentMetadata) {
	regions := []string{}
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		for _, raw := range src.Region {
			for _, part := range strings.Split(raw, "/") {
				w := strings.TrimSpace(part)
				if id, ok := yemaRegionID[w]; ok {
					dup := false
					for _, e := range regions {
						if e == id {
							dup = true
							break
						}
					}
					if !dup {
						regions = append(regions, id)
					}
				}
				if len(regions) >= 3 {
					break
				}
			}
			if len(regions) >= 3 {
				break
			}
		}
	}
	if len(regions) > 0 {
		if b, err := json.Marshal(regions); err == nil {
			form["yema_regions"] = string(b)
		}
	}
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil && len(src.TranslatedTitles) > 0 {
		form["yema_translated"] = string(src.TranslatedTitles)
	}
	form["yema_poster"] = meta.Poster
}

// rousiPublishInject §59.458-②③: 肉丝 attributes 数据源注入——
// genre（PTGen 题材数组剥 miss 后 JSON；全空兜"其它"——站方必填）
// +region（PTGen 产地 norm 三条+合拍首值优先（§59.391 葡萄先例同数据
// 源同策略）+label→slug 映射；miss 兜 other）+poster（封面 URL——
// adapter 窄例外下载 base64）。form 显示键（DryRun 可见）。
func rousiPublishInject(form map[string]string, meta *model.TorrentMetadata) {
	// genre
	genres := []string{}
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		for _, g := range src.Genre {
			g = strings.TrimSpace(g)
			if g != "" && rousiGenreSet[g] {
				genres = append(genres, g)
			}
		}
	}
	if len(genres) == 0 {
		genres = []string{"其它"}
	}
	if b, err := json.Marshal(genres); err == nil {
		form["rs_genre"] = string(b)
	}

	// region（PTGen Region=[]string 合拍串形态——元素内再按 / 切）
	region := "other"
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
	outer:
		for _, raw := range src.Region {
			for _, part := range strings.Split(raw, "/") {
				w := strings.TrimSpace(part)
				if w == "" {
					continue
				}
				// norm 三条：中国大陆→大陆/中国香港→香港/中国台湾→台湾
				switch {
				case strings.HasPrefix(w, "中国大陆"):
					w = "大陆"
				case strings.HasPrefix(w, "中国香港"):
					w = "香港"
				case strings.HasPrefix(w, "中国台湾"):
					w = "台湾"
				}
				if slug, ok := rousiRegionSlug[w]; ok {
					region = slug
					break outer // 首值优先——命中即止（§59.391 葡萄先例）
				}
			}
		}
	}
	form["rs_region"] = region

	// poster（封面 URL——adapter §59.458 窄例外下载）
	form["rs_poster"] = meta.Poster
}

func hdcityPrefill(form map[string]string, cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile) {
	if form == nil || cfg == nil || meta == nil {
		return
	}
	// ① bigname/small_descr 双填副标题（空态拼装退化）
	subtitle := strings.TrimSpace(meta.Subtitle)
	if subtitle == "" {
		subtitle = hdcitySubtitleFallback(meta, tp)
	}
	if subtitle != "" {
		form["bigname"] = subtitle
		form["small_descr"] = subtitle
	}
	// ② tag1ing 题材注入
	if v := hdcityTag1ingValue(cfg, meta); v != "" {
		form["tag1ing"] = v
	}
	// ③ processing 3D（城市裸名无域常量——好学 processing_sel[4] 先例）
	if m := hdcityProcessingOption(cfg, tp); m != nil {
		form["processing_sel"] = m.Value
	}
}

// isUploadTimeoutErr §59.470: 上传超时类错误判定——超时≠失败（站方可能
// 已完成入库·南洋案实证），触发站方复查通道。
func isUploadTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "Client.Timeout") ||
		strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "Timeout exceeded") ||
		strings.Contains(s, "timeout")
}

// repushConverge 补推成功后的原记录收敛（§59.159 设计意图·提方法可测）：
// uploaded 行 → status=pushed+seeded（上传未推种→已推种）；
// duplicate 行 → 仅置 seeded（v0.0.1455 Caddo Lake/Dead Man 双案：§59.509
// duplicate 成可补推主力后回写漏跟·status 保持「站上已有」事实不变·
// canRepush 判据 tid 在手且本地未做种——seeded 置位即按钮收敛）。
func (e *PublishExecutor) repushConverge(ctx context.Context, in ExecuteInput, now time.Time) {
	res2 := e.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("torrent_id = ? AND target_site = ? AND status = ?",
			in.TorrentID, in.TargetSite, "uploaded").
		Updates(map[string]interface{}{
			"status":     "pushed",
			"seeded":     true,
			"seeded_at":  now,
			"seed_error": "",
		})
	if res2.Error != nil {
		e.logger.Warn("repush 回写失败", zap.Error(res2.Error))
	} else if res2.RowsAffected > 0 {
		e.logger.Info("repush 回写收敛", zap.String("tid", in.TorrentID), zap.Int64("rows", res2.RowsAffected))
	}
	res3 := e.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("torrent_id = ? AND target_site = ? AND status = ? AND seeded = ?",
			in.TorrentID, in.TargetSite, "duplicate", false).
		Updates(map[string]interface{}{
			"seeded":    true,
			"seeded_at": now,
		})
	if res3.Error != nil {
		e.logger.Warn("repush duplicate 回写失败", zap.Error(res3.Error))
	} else if res3.RowsAffected > 0 {
		e.logger.Info("repush duplicate 收敛", zap.String("tid", in.TorrentID), zap.Int64("rows", res3.RowsAffected))
	}
}
