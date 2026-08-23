package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SiteAdapterProvider interface {
	GetAdapter(ctx context.Context, siteName string) (model.SiteAdapter, error)
	GetSiteConfig(ctx context.Context, siteName string) (*model.SiteConfig, error)
}

type Fetcher struct {
	db           *gorm.DB
	logger       *zap.Logger
	siteProvider SiteAdapterProvider
}

func NewFetcher(db *gorm.DB, logger *zap.Logger, siteProvider SiteAdapterProvider) *Fetcher {
	return &Fetcher{
		db:           db,
		logger:       logger.With(zap.String("component", "metadata")),
		siteProvider: siteProvider,
	}
}

// SetEngine 已弃用（§56.13 方案 B：Engine 注入到 adapter.Factory，不再需要 fetcher 持有）。
// 保留为空操作避免破坏外部调用。
func (f *Fetcher) SetEngine(_ *extract.Engine) {}

// FetchAndStoreDirect §59.61: comment 直达入口——拉详情后 D3-C 标题轻校验
// （不过则报错，调用方降级下一 tid 来源/搜索）。sourceName = 本地种子名。
func (f *Fetcher) FetchAndStoreDirect(ctx context.Context, infoHash, siteName, torrentID, sourceName string) (*model.TorrentMetadata, error) {
	meta, err := f.FetchAndStore(ctx, infoHash, siteName, torrentID)
	if err != nil {
		return nil, err
	}
	// D3=C 轻校验：标题相关性（不校验 size/分辨率——元数据版本不敏感）
	if meta != nil && meta.Title != "" && sourceName != "" {
		if !reseed.TitleRelevant(sourceName, meta.Title) {
			f.logger.Warn("direct fetch title mismatch (D3 reject)",
				zap.String("site", siteName),
				zap.String("torrent_id", torrentID),
				zap.String("source", sourceName[:min(len(sourceName), 50)]),
				zap.String("detail", meta.Title[:min(len(meta.Title), 50)]))
			return nil, fmt.Errorf("直达标题不相关（D3 拒绝, tid=%s）", torrentID)
		}
	}
	return meta, nil
}

func (f *Fetcher) FetchAndStore(ctx context.Context, infoHash, siteName, torrentID string) (*model.TorrentMetadata, error) {
	if infoHash == "" || siteName == "" || torrentID == "" {
		return nil, fmt.Errorf("info_hash, site_name, torrent_id are required")
	}

	meta, err := f.fetchFromSite(ctx, infoHash, siteName, torrentID, "rss_detail")
	if err != nil {
		f.logger.Debug("primary site fetch failed, trying IYUU fallback",
			zap.String("site", siteName),
			zap.String("info_hash", infoHash),
			zap.Error(err))

		meta = f.fetchWithIYUUFallback(ctx, infoHash, siteName)
		if meta == nil {
			return nil, fmt.Errorf("all fetch attempts failed: primary=%w", err)
		}
	}

	if err := f.store(ctx, meta); err != nil {
		return nil, fmt.Errorf("store metadata: %w", err)
	}

	f.logger.Debug("metadata fetched and stored",
		zap.String("info_hash", infoHash),
		zap.String("site_name", siteName),
		zap.String("standard_type", meta.StandardType),
		zap.String("fetch_source", meta.FetchSource))

	return meta, nil
}

// FetchAndStoreBySearch 无 tid 时通过 L2 搜索反查 tid，再调 FetchAndStore。
// §56.33 决策 A（方案 A）：复用 reseed.SearchAndVerifyMatch（辅种 L2 搜索能力）。
// 调用时机（D1 tid 链的最后兜底）：前端传值 / coverage / torrent_metadata.TorrentID 都没拿到 tid 时。
//
// 参数：
//   - torrentName: 下载器种子名（用于提取搜索关键词 + 制作组）
//   - size: 种子大小（字节，用于 L2 搜索过滤，0 表示不按 size 过滤）
//   - sourceLocalMI: 源侧本地文件 MediaInfo（§59.36 修订，可选）——音频 token
//     冲突时做 MI 仲裁；空则降级 skipAudio 放行（v0.0.644 行为）。
//
// 返回：反查到 tid 并采集成功 → *TorrentMetadata；反查失败 → nil, error
func (f *Fetcher) FetchAndStoreBySearch(ctx context.Context, infoHash, siteName, torrentName string, size int64, sourceLocalMI ...string) (*model.TorrentMetadata, error) {
	localMI := ""
	if len(sourceLocalMI) > 0 {
		localMI = sourceLocalMI[0]
	}
	if infoHash == "" || siteName == "" || torrentName == "" {
		return nil, fmt.Errorf("info_hash, site_name, torrent_name are required")
	}
	if f.siteProvider == nil {
		return nil, fmt.Errorf("site provider not configured")
	}

	config, err := f.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || config == nil {
		return nil, fmt.Errorf("get site config for %s: %w", siteName, err)
	}
	adapter, err := f.siteProvider.GetAdapter(ctx, siteName)
	if err != nil || adapter == nil {
		return nil, fmt.Errorf("get adapter for %s: %w", siteName, err)
	}

	keyword := reseed.ExtractSearchKeyword(torrentName)
	groupName := reseed.ExtractGroupName(torrentName)
	if keyword == "" {
		return nil, fmt.Errorf("cannot extract search keyword from title: %s", torrentName)
	}

	match, allResults, err := reseed.SearchAndVerifyMatchWithResults(ctx, adapter, config, keyword, groupName, size, torrentName)
	if err != nil {
		return nil, fmt.Errorf("L2 search failed on %s: %w", siteName, err)
	}

	// §59.36 修订: 主轮 + loose 轮的音频冲突候选收集（组名/相关性/其余字段全过，
	// 仅音频 token 冲突）→ MI 仲裁（源 local MI 可得时）或降级放行。
	// 源 local MI 可得且仲裁返回 nil = 全员 MI 不一致确定性出局（站内真无此
	// 音频版本）——此时不进 loose 轮（loose 的 skipAudio 豁免会推翻确定性判定，
	// 拿错版本元数据），直接失败。
	arbitratedOut := false
	if match == nil {
		var ambiguous []*reseed.L2MatchResult
		ambiguous = append(ambiguous, reseed.AudioConflictCandidates(allResults, groupName, size, torrentName)...)
		if len(ambiguous) > 0 {
			m := f.resolveAudioConflict(ctx, adapter, config, siteName, ambiguous, localMI)
			if m != nil {
				match = m
				f.logger.Info("FetchAndStoreBySearch: audio conflict resolved",
					zap.String("site", siteName),
					zap.String("torrent_id", m.TorrentID),
					zap.String("matched_title", m.Title),
					zap.Bool("mi_arbitrated", localMI != ""))
			} else if localMI != "" {
				arbitratedOut = true
				f.logger.Info("FetchAndStoreBySearch: audio conflict arbitrated out (MI mismatch)",
					zap.String("site", siteName),
					zap.Int("candidates", len(ambiguous)))
			}
		}
	}

	// §59.30 元数据获取宽松降级：size 校验失败（站内 REPACK 重发替换原版等
	// 同资源不同版本场景）时，仅按 标题相关性+组名+TechProfile 验证重试一轮。
	// 元数据（海报/声明/简介）与版本无关，可安全获取；注入校验由
	// ValidateInjection 独立兜底（§59.25），不受此放宽影响。
	// §59.36 修订: 仲裁确定性出局（arbitratedOut）时跳过——loose 的 skipAudio
	// 豁免（降级②路径）只服务"源 MI 不可得"场景，不得推翻 MI 仲裁判定。
	if match == nil && size > 0 && !arbitratedOut {
		if m := reseed.SearchAndVerifyLoose(ctx, adapter, config, keyword, groupName, torrentName); m != nil {
			match = m
			f.logger.Info("FetchAndStoreBySearch: loose match (size skipped)",
				zap.String("site", siteName),
				zap.String("torrent_id", m.TorrentID),
				zap.String("matched_title", m.Title))
		}
	}

	if match == nil {
		// §59.59 审计: 附搜索结果统计（区分"站内无结果"与"结果被验证拒绝"）
		return nil, fmt.Errorf("L2 search no match on %s (keyword=%s group=%s results=%d)", siteName, keyword, groupName, len(allResults))
	}

	f.logger.Debug("FetchAndStoreBySearch: tid resolved",
		zap.String("site", siteName),
		zap.String("torrent_id", match.TorrentID),
		zap.String("matched_title", match.Title))

	return f.FetchAndStore(ctx, infoHash, siteName, match.TorrentID)
}

// resolveAudioConflict §59.36 修订: 音频冲突候选 MI 仲裁。
// 候选 MI 现场抓详情页（tid 已知 +1 请求，音频冲突低频触发）；
// 源 local MI 可得走①仲裁，否则②盲放行（ResolveAudioConflict 内部处理）。
func (f *Fetcher) resolveAudioConflict(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, siteName string, candidates []*reseed.L2MatchResult, sourceLocalMI string) *reseed.L2MatchResult {
	// 仅源 local MI 可得时才需要候选 MI（②盲放行不抓详情页，省请求）
	candidateMI := map[string]string{}
	if sourceLocalMI != "" {
		for _, c := range candidates {
			detail, err := adapter.GetTorrentDetail(ctx, config, c.TorrentID)
			if err != nil || detail == nil {
				continue
			}
			if detail.MediaInfo != "" {
				candidateMI[c.TorrentID] = detail.MediaInfo
			}
		}
	}
	return reseed.ResolveAudioConflict(candidates, sourceLocalMI, candidateMI)
}

func (f *Fetcher) fetchFromSite(ctx context.Context, infoHash, siteName, torrentID, fetchSource string) (*model.TorrentMetadata, error) {
	if f.siteProvider == nil {
		return nil, fmt.Errorf("site provider not configured")
	}

	siteCfg, err := f.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || siteCfg == nil {
		return nil, fmt.Errorf("get site config for %s: %w", siteName, err)
	}

	adapter, err := f.siteProvider.GetAdapter(ctx, siteName)
	if err != nil || adapter == nil {
		return nil, fmt.Errorf("get adapter for %s: %w", siteName, err)
	}

	detail, err := adapter.GetTorrentDetail(ctx, siteCfg, torrentID)
	if err != nil {
		return nil, fmt.Errorf("get torrent detail from %s: %w", siteName, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("nil detail returned from %s", siteName)
	}

	// §59.26 方案 A: keepfrds（朋友站）title/subtitle 互换
	// 朋友站部分种子 title=中文格式化标题，subtitle=英文 v1.05 发种名。
	// 副标题非空时无条件互换（PTNexus 同款逻辑）。
	if strings.Contains(siteCfg.Domain, "keepfrds") && detail.Subtitle != "" {
		detail.Title, detail.Subtitle = detail.Subtitle, detail.Title
	}

	// §56.13 方案 B: adapter 内部已用 Engine 提取（如 NexusPHPAdapter）。
	// fetcher 不再调 Engine，仅做 titleparser 补全（fallback，针对 Engine 没提取到的字段）。
	fillDetailFromTitle(detail)

	// engineMeta 用于诊断（detail_source_json.extractor_info）
	engineMeta := extract.Meta{ExtractorName: "adapter.GetTorrentDetail"}
	if detail.EngineExtractorName != "" {
		engineMeta.ExtractorName = detail.EngineExtractorName
	}

	meta := f.buildMetadata(infoHash, siteName, torrentID, detail, engineMeta)
	meta.FetchSource = fetchSource
	return meta, nil
}

// fillDetailFromTitle 用 titleparser 从详情页标题解析基本字段，补全 detail 中的空字段。
// 仅补全空字段（不覆盖 Engine 已提取的）。所有补全的字段经 LookupStandardKey 标准化。
//
// 两层提取（v0.0.244 新增第二层）：
//  1. titleparser.ParseTitle: 严格 scene 命名解析（resolution/codec/...）
//  2. titleparser.ExtractXXX: 宽松正则匹配（移植 auto_feed_js，能从任意文本提取）
//
// 适用场景：
//   - PTer 详情页"基本信息"聚合文本不含 codec/resolution（只有 type/size）
//   - 但 h1 标题含完整 scene 命名（"Ride or Die S01 2026 1080p AMZN WEB-DL H.264 DDP 5.1 Atmos-CMCTV"）
//   - titleparser 能从标题解析出 resolution/video_codec/audio_codec/release_group
//
// 字段语义映射（注意 detail.Source 实际语义是"媒介"）：
//   - detail.Resolution ← titleparser components.Resolution / ExtractResolution
//   - detail.Codec ← components.VideoCodec / ExtractCodec
//   - detail.AudioCodec ← components.AudioCodec / ExtractAudioCodec
//   - detail.ReleaseGroup ← components.ReleaseGroup
//   - detail.Source ← components.Medium / ExtractMedium（detail.Source 在 model 中语义=媒介）
func fillDetailFromTitle(detail *model.TorrentDetail) {
	title := strings.TrimSpace(detail.Title)
	if title == "" {
		return
	}
	c := titleparser.ParseTitle(title)
	if detail.Resolution == "" && c.Resolution != "" {
		if std := extract.LookupStandardKey("resolution", c.Resolution); std != "" {
			detail.Resolution = std
		} else {
			detail.Resolution = c.Resolution
		}
	}
	if detail.Codec == "" && c.VideoCodec != "" {
		if std := extract.LookupStandardKey("video_codec", c.VideoCodec); std != "" {
			detail.Codec = std
		} else {
			detail.Codec = c.VideoCodec
		}
	}
	if detail.AudioCodec == "" && c.AudioCodec != "" {
		if std := extract.LookupStandardKey("audio_codec", c.AudioCodec); std != "" {
			detail.AudioCodec = std
		} else {
			detail.AudioCodec = c.AudioCodec
		}
	}
	if detail.ReleaseGroup == "" && c.ReleaseGroup != "" {
		if std := extract.LookupStandardKey("team", c.ReleaseGroup); std != "" {
			detail.ReleaseGroup = std
		} else {
			detail.ReleaseGroup = c.ReleaseGroup
		}
	}
	// detail.Source 语义=媒介，titleparser components.Medium 是媒介
	if detail.Source == "" && c.Medium != "" {
		if std := extract.LookupStandardKey("medium", c.Medium); std != "" {
			detail.Source = std
		} else {
			detail.Source = c.Medium
		}
	}

	// 第二层 fallback（v0.0.244）: titleparser.ExtractXXX 从 title 宽松正则匹配。
	// 仅当 ParseTitle 没提取到时尝试（互补关系）。
	// 这些正则来自 auto_feed_js 实战验证，与 ParseTitle 的严格解析互补。
	if detail.Resolution == "" {
		if v := titleparser.ExtractResolution(title); v != "" {
			if std := extract.LookupStandardKey("resolution", v); std != "" {
				detail.Resolution = std
			} else {
				detail.Resolution = v
			}
		}
	}
	if detail.Codec == "" {
		if v := titleparser.ExtractCodec(title); v != "" {
			if std := extract.LookupStandardKey("video_codec", v); std != "" {
				detail.Codec = std
			} else {
				detail.Codec = v
			}
		}
	}
	if detail.AudioCodec == "" {
		if v := titleparser.ExtractAudioCodec(title); v != "" {
			if std := extract.LookupStandardKey("audio_codec", v); std != "" {
				detail.AudioCodec = std
			} else {
				detail.AudioCodec = v
			}
		}
	}
	if detail.Source == "" {
		if v := titleparser.ExtractMedium(title, title); v != "" {
			if std := extract.LookupStandardKey("medium", v); std != "" {
				detail.Source = std
			} else {
				detail.Source = v
			}
		}
	}
}

// deriveSiteCode 从 site.Domain 推导 site_code（如 pterclub.net → "pterclub"）。
// Engine.specialByCode 用此 key 匹配站点特殊提取器。
// §56.13 方案 B: deriveSiteCode 和 mergeSeedIntoDetail 已移到 adapter 包
// （adapter 内部调 Engine 时用）。

func (f *Fetcher) fetchWithIYUUFallback(ctx context.Context, infoHash, primarySite string) *model.TorrentMetadata {
	var caches []model.SiteCoverageCache
	if err := f.db.WithContext(ctx).
		Where("info_hash = ? AND site_name != ? AND status = ? AND torrent_id != ''",
			infoHash, primarySite, model.CoverageConfirmedHas).
		Order("confidence DESC").
		Limit(5).
		Find(&caches).Error; err != nil || len(caches) == 0 {
		return nil
	}

	for _, c := range caches {
		f.logger.Debug("trying IYUU fallback site",
			zap.String("fallback_site", c.SiteName),
			zap.String("torrent_id", c.TorrentID),
			zap.String("info_hash", infoHash))

		meta, err := f.fetchFromSite(ctx, infoHash, c.SiteName, c.TorrentID, "iyuu_cache")
		if err != nil {
			f.logger.Debug("IYUU fallback site failed",
				zap.String("site", c.SiteName),
				zap.Error(err))
			continue
		}
		return meta
	}
	return nil
}

func (f *Fetcher) buildMetadata(infoHash, siteName, torrentID string, detail *model.TorrentDetail, engineMeta extract.Meta) *model.TorrentMetadata {
	now := time.Now()
	meta := &model.TorrentMetadata{
		InfoHash:          infoHash,
		SiteName:          siteName,
		TorrentID:         torrentID,
		Title:             detail.Title,
		Subtitle:          detail.Subtitle,
		SourceCategory:    detail.Category,
		SourceDescription: detail.Description,
		Description:       detail.Description,
		Statement:         detail.Statement,
		Poster:            detail.PosterURL,
		IMDbURL:           detail.IMDbURL,
		DoubanURL:         detail.DoubanURL,
		TMDbURL:           detail.TMDbURL,
		FetchSource:       "rss_detail",
		FetchedAt:         now,
	}

	meta.StandardType = f.normalizeCategory(detail.Category)

	if len(detail.Tags) > 0 {
		if data, err := json.Marshal(detail.Tags); err == nil {
			meta.Tags = string(data)
		}
	}

	if len(detail.Flags) > 0 {
		if data, err := json.Marshal(detail.Flags); err == nil {
			meta.Flags = string(data)
		}
	}

	if len(detail.Screenshots) > 0 {
		if data, err := json.Marshal(detail.Screenshots); err == nil {
			meta.Screenshots = string(data)
		}
	}

	if detail.MediaInfo != "" {
		meta.SourceMediaInfo = detail.MediaInfo
		meta.MediaInfoSource = "source_site"
	}

	// §56.13 + §56.14 方案 B: 构建 detail_source_json。
	// detail 已经是 adapter 内部 Engine 提取的结果（含 SourceRegion 等），
	// DetailToSeed 会自动处理 detail.SourceRegion → seed.Source 的映射。
	seed := extract.DetailToSeed(detail)
	detailSource := SeedToDetailSource(seed, now, engineMeta)
	if data, err := json.Marshal(detailSource); err == nil {
		meta.DetailSourceJSON = string(data)
	}

	return meta
}

func (f *Fetcher) normalizeCategory(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "movie") || strings.Contains(raw, "电影"):
		return "category.movie"
	case strings.Contains(lower, "tv") || strings.Contains(lower, "series") || strings.Contains(raw, "电视剧") || strings.Contains(raw, "剧集"):
		return "category.tv_series"
	case strings.Contains(lower, "anim") || strings.Contains(raw, "动漫") || strings.Contains(raw, "动画"):
		return "category.animation"
	case strings.Contains(lower, "doc") || strings.Contains(raw, "纪录"):
		return "category.documentaries"
	case strings.Contains(lower, "variety") || strings.Contains(lower, "show") || strings.Contains(raw, "综艺"):
		return "category.tv_shows"
	case strings.Contains(lower, "music") || strings.Contains(raw, "音乐"):
		return "category.music"
	case strings.Contains(lower, "sport") || strings.Contains(raw, "体育"):
		return "category.sports"
	default:
		if strings.HasPrefix(raw, "category.") {
			return raw
		}
		return ""
	}
}

func (f *Fetcher) store(ctx context.Context, meta *model.TorrentMetadata) error {
	// §56.33: 分离 attrs 和 dest。
	// 原 Assign(meta).FirstOrCreate(meta) 对已存在记录失效：FirstOrCreate 读 DB 旧值
	// 到 dest（meta）时，Assign 的 attrs（同一 meta 指针）也被旧值覆盖，导致用旧值
	// 更新=无效。修复：attrs 用 meta 的值副本，FirstOrCreate 的 dest 用 meta 本体。
	attrs := *meta
	return f.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
		Assign(&attrs).
		FirstOrCreate(meta).Error
}

func (f *Fetcher) GetMetadata(ctx context.Context, infoHash, siteName string) (*model.TorrentMetadata, bool) {
	var meta model.TorrentMetadata
	if err := f.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&meta).Error; err != nil {
		return nil, false
	}
	return &meta, true
}

func (f *Fetcher) GetMetadataByHash(ctx context.Context, infoHash string) (*model.TorrentMetadata, bool) {
	var meta model.TorrentMetadata
	if err := f.db.WithContext(ctx).
		Where("info_hash = ?", infoHash).
		Order("updated_at DESC").
		First(&meta).Error; err != nil {
		return nil, false
	}
	return &meta, true
}
