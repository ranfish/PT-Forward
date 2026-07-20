package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	engine        *extract.Engine
}

func NewFetcher(db *gorm.DB, logger *zap.Logger, siteProvider SiteAdapterProvider) *Fetcher {
	return &Fetcher{
		db:           db,
		logger:       logger.With(zap.String("component", "metadata")),
		siteProvider: siteProvider,
	}
}

// SetEngine 注入提取器路由（§56.13 接线点）。
// 启动时由 main.go 调用：metadataFetcher.SetEngine(engine)。
// engine == nil 时退化为老 adapter 字段（向后兼容）。
func (f *Fetcher) SetEngine(engine *extract.Engine) { f.engine = engine }

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

	// §56.13 接线: 拿到 rawHTML 后调 Engine.Extract（站点 hook + PublicExtractor）
	engineMeta := extract.Meta{ExtractorName: "adapter.GetTorrentDetail"}
	var engineSeed extract.SeedData // 保留 Engine 输出，buildMetadata 时用于补全 detail 中缺失的字段（如 Source 产地）
	if f.engine != nil && detail.RawHTML != "" {
		input := extract.Input{
			SiteCode:      deriveSiteCode(siteCfg.Domain),
			SiteNickname:  siteCfg.SiteName,
			BaseURL:       siteCfg.BaseURL,
			TorrentID:     torrentID,
			PageHTML:      detail.RawHTML,
			FallbackTitle: detail.Title,
		}
		seed, meta := f.engine.Extract(input)
		if meta.ExtractorName != "" {
			engineMeta = meta
		}
		if seed.IsMeaningful() {
			engineSeed = seed
			mergeSeedIntoDetail(detail, seed)
			f.logger.Debug("engine extracted",
				zap.String("extractor", meta.ExtractorName),
				zap.Bool("used_fallback", meta.UsedFallback),
				zap.Int("mediainfo_len", len(seed.MediaInfo)),
				zap.Int("body_len", len(seed.Intro.Body)),
				zap.Int("screenshots_len", len(seed.Intro.ScreenshotURLs())),
				zap.Int("flags", len(seed.Flags)))
		}
	}

	// §56.13 方案 1: 用 titleparser 从 h1/标题补全 detail 中空的基本字段。
	// titleparser 已能解析 resolution/video_codec/audio_codec/release_group/medium。
	// Engine 提取的字段优先（更精准），titleparser 仅补全空字段。
	// 补全的字段经 LookupStandardKey 标准化为 code（与 Engine 输出格式一致）。
	fillDetailFromTitle(detail)

	meta := f.buildMetadata(infoHash, siteName, torrentID, detail, engineMeta, engineSeed)
	meta.FetchSource = fetchSource
	return meta, nil
}

// fillDetailFromTitle 用 titleparser 从详情页标题解析基本字段，补全 detail 中的空字段。
// 仅补全空字段（不覆盖 Engine 已提取的）。所有补全的字段经 LookupStandardKey 标准化。
//
// 适用场景：
//   - PTer 详情页"基本信息"聚合文本不含 codec/resolution（只有 type/size）
//   - 但 h1 标题含完整 scene 命名（"Ride or Die S01 2026 1080p AMZN WEB-DL H.264 DDP 5.1 Atmos-CMCTV"）
//   - titleparser 能从标题解析出 resolution/video_codec/audio_codec/release_group
//
// 字段语义映射（注意 detail.Source 实际语义是"媒介"）：
//   - detail.Resolution ← titleparser components.Resolution
//   - detail.Codec ← components.VideoCodec
//   - detail.AudioCodec ← components.AudioCodec
//   - detail.ReleaseGroup ← components.ReleaseGroup
//   - detail.Source ← components.Medium（detail.Source 在 model 中语义=媒介）
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
}

// deriveSiteCode 从 site.Domain 推导 site_code（如 pterclub.net → "pterclub"）。
// Engine.specialByCode 用此 key 匹配站点特殊提取器。
// 推导失败返回空串（Engine 会自动 fallback 到 public）。
func deriveSiteCode(domain string) string {
	if domain == "" {
		return ""
	}
	host := domain
	if u, err := url.Parse(domain); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	// 去掉端口
	if i := strings.IndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	parts := strings.Split(host, ".")
	if len(parts) == 0 {
		return ""
	}
	// 取第一段（如 pterclub.net → "pterclub"）
	return strings.ToLower(parts[0])
}

// mergeSeedIntoDetail 把 Engine 输出的 SeedData 字段合并到 detail（仅覆盖非空字段）。
// Engine 优先于 adapter（更精准）。
func mergeSeedIntoDetail(detail *model.TorrentDetail, seed extract.SeedData) {
	if seed.Title != "" {
		detail.Title = seed.Title
	}
	if seed.Subtitle != "" {
		detail.Subtitle = seed.Subtitle
	}
	if seed.Intro.Body != "" {
		detail.Description = seed.Intro.Body
	}
	if seed.Intro.Poster != "" {
		detail.PosterURL = seed.Intro.Poster
	}
	if urls := seed.Intro.ScreenshotURLs(); len(urls) > 0 {
		detail.Screenshots = urls
	}
	if seed.MediaInfo != "" {
		detail.MediaInfo = seed.MediaInfo
	}
	if seed.BDInfo != "" {
		detail.BDInfo = seed.BDInfo
	}
	if seed.Type != "" {
		detail.Category = seed.Type
	}
	if seed.Medium != "" {
		detail.Source = seed.Medium
	}
	if seed.Resolution != "" {
		detail.Resolution = seed.Resolution
	}
	if seed.VideoCodec != "" {
		detail.Codec = seed.VideoCodec
	}
	if seed.AudioCodec != "" {
		detail.AudioCodec = seed.AudioCodec
	}
	if seed.ReleaseGroup != "" {
		detail.ReleaseGroup = seed.ReleaseGroup
	}
	if len(seed.Tags) > 0 {
		detail.Tags = seed.Tags
	}
	// flags 始终用 Engine 输出覆盖（即使空），因为老 adapter 的 extractFlagsFromText
	// 扫描整页 HTML 会把站点级"禁转声明"区块/评论区/相关推荐误识别为种子 flags。
	// Engine 的 extractFlags 只扫 title+subtitle+descrBBCode，更精准。
	detail.Flags = seed.Flags
	if seed.IMDbLink != "" {
		detail.IMDbURL = seed.IMDbLink
	}
	if seed.DoubanLink != "" {
		detail.DoubanURL = seed.DoubanLink
	}
	if seed.TMDbLink != "" {
		detail.TMDbURL = seed.TMDbLink
	}
}

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

func (f *Fetcher) buildMetadata(infoHash, siteName, torrentID string, detail *model.TorrentDetail, engineMeta extract.Meta, engineSeed extract.SeedData) *model.TorrentMetadata {
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

	// §56.13 + §56.14: 构建 detail_source_json（三源之一，/merge 接口使用）
	// 优先用 Engine 输出（更精准），fallback 用 adapter 字段
	seed := extract.DetailToSeed(detail)
	// §56.13 v0.0.241: detail 没有"产地"字段（model.TorrentDetail.Source 语义=媒介），
	// 但 Engine 提取了 seed.Source（产地，如"欧美"），需要保留到 detail_source_json。
	// 这里用 Engine seed.Source 补全（mergeSeedIntoDetail 因 detail 无对应字段会丢弃）。
	if seed.Source == "" && engineSeed.Source != "" {
		seed.Source = engineSeed.Source
	}
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
