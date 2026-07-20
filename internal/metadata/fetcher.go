package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SiteAdapterProvider interface {
	GetAdapter(ctx context.Context, siteName string) (model.SiteAdapter, error)
	GetSiteConfig(ctx context.Context, siteName string) (*model.SiteConfig, error)
}

type Fetcher struct {
	db            *gorm.DB
	logger        *zap.Logger
	siteProvider  SiteAdapterProvider
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
		InfoHash:        infoHash,
		SiteName:        siteName,
		TorrentID:       torrentID,
		Title:           detail.Title,
		Subtitle:        detail.Subtitle,
		SourceCategory:  detail.Category,
		SourceDescription: detail.Description,
		Description:     detail.Description,
		Poster:          detail.PosterURL,
		IMDbURL:         detail.IMDbURL,
		DoubanURL:       detail.DoubanURL,
		TMDbURL:         detail.TMDbURL,
		FetchSource:     "rss_detail",
		FetchedAt:       now,
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
	return f.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
		Assign(meta).
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
