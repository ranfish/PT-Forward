package publish

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SourceSiteDetector struct {
	db     *gorm.DB
	logger *zap.Logger
	cache  map[string]string // group_name → site_name
	mu     sync.RWMutex
}

func NewSourceSiteDetector(db *gorm.DB, logger *zap.Logger) *SourceSiteDetector {
	return &SourceSiteDetector{db: db, logger: logger, cache: make(map[string]string)}
}

// ExtractGroupName 从标题末尾提取制作组名（委托至 util.ExtractGroupName）
func ExtractGroupName(title string) string {
	return util.ExtractGroupName(title)
}

type SourceDetectResult struct {
	SourceSite    string
	SourceSiteID  uint
	GroupName     string
	TorrentID     string
	AutoDetected  bool
}

func (d *SourceSiteDetector) Detect(ctx context.Context, title, infoHash string, coverageSites []model.SiteCoverageCache) SourceDetectResult {
	result := SourceDetectResult{}

	// §56.40: 用户配置的优先级列表优先（用户配置 > 自动检测）
	if priority := d.getSourcePriority(ctx); len(priority) > 0 && len(coverageSites) > 0 {
		siteMap := make(map[string]model.SiteCoverageCache)
		for _, c := range coverageSites {
			if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
				siteMap[c.SiteName] = c
			}
		}
		for _, siteName := range priority {
			if c, ok := siteMap[siteName]; ok {
				var site model.Site
				if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
					result.SourceSite = site.Name
					result.SourceSiteID = site.ID
					result.TorrentID = c.TorrentID
					result.AutoDetected = false
					return result
				}
			}
		}
	}

	groupName := ExtractGroupName(title)
	result.GroupName = groupName

	if groupName != "" {
		siteName := d.LookupGroup(ctx, groupName)
		if siteName != "" {
			var site model.Site
			// §59.20: Step 2 加 cookie 检查——源站无 cookie 时降级到 Step 3
			if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
				result.SourceSite = site.Name
				result.SourceSiteID = site.ID
				result.AutoDetected = true
				for _, c := range coverageSites {
					if c.SiteName == site.Name {
						result.TorrentID = c.TorrentID
						break
					}
				}
				return result
			}
		}
	}

	// 降级：覆盖数据中找第一个有 cookie 的源站
	siteMap := make(map[string]model.SiteCoverageCache)
	for _, c := range coverageSites {
		if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
			siteMap[c.SiteName] = c
		}
	}
	if len(siteMap) > 0 {
		var sites []model.Site
		if err := d.db.WithContext(ctx).Where("enabled = ? AND is_source = ?", true, true).Find(&sites).Error; err != nil {
			return result
		}
		for _, site := range sites {
			if c, ok := siteMap[site.Name]; ok {
				if site.Cookie != "" {
					result.SourceSite = site.Name
					result.SourceSiteID = site.ID
					result.TorrentID = c.TorrentID
					result.AutoDetected = false
					return result
				}
			}
		}
		for siteName, c := range siteMap {
			var site model.Site
			if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ?", siteName, true).First(&site).Error; err == nil {
				if site.Cookie != "" {
					result.SourceSite = site.Name
					result.SourceSiteID = site.ID
					result.TorrentID = c.TorrentID
					result.AutoDetected = false
					return result
				}
			}
		}
	}

	return result
}

func (d *SourceSiteDetector) LookupGroup(ctx context.Context, groupName string) string {
	if groupName == "" {
		return ""
	}
	d.mu.RLock()
	if siteName, ok := d.cache[groupName]; ok {
		d.mu.RUnlock()
		return siteName
	}
	d.mu.RUnlock()

	var mapping model.ReleaseGroupMapping
	if err := d.db.WithContext(ctx).
		Where("LOWER(group_name) = LOWER(?)", groupName).
		First(&mapping).Error; err != nil {
		// §59.29: 缓存 miss 结果——大列表（2w+ 行）循环中未映射组名
		// 会反复触发 DB 查询（不同 title 同组名），miss 缓存消除重复查询
		d.mu.Lock()
		d.cache[groupName] = ""
		d.mu.Unlock()
		return ""
	}

	// 优先用用户修正的 site_name
	if mapping.SiteName != "" {
		d.mu.Lock()
		d.cache[groupName] = mapping.SiteName
		d.mu.Unlock()
		return mapping.SiteName
	}

	// 按 domain 模糊匹配站点表（含 alternative_domains，自动去除 www.）
	if mapping.Domain != "" {
		bareDomain := strings.TrimPrefix(strings.ToLower(mapping.Domain), "www.")
		var site model.Site
		pattern := "%" + bareDomain + "%"
		if err := d.db.WithContext(ctx).
			Where("LOWER(REPLACE(domain, 'www.', '')) LIKE ? OR LOWER(REPLACE(base_url, 'www.', '')) LIKE ? OR LOWER(alternative_domains) LIKE ?", pattern, pattern, pattern).
			First(&site).Error; err == nil {
			resolved := site.Name
			d.mu.Lock()
			d.cache[groupName] = resolved
			d.mu.Unlock()
			return resolved
		}
	}

	return ""
}

func (d *SourceSiteDetector) RefreshCache(ctx context.Context) {
	var mappings []model.ReleaseGroupMapping
	if err := d.db.WithContext(ctx).Find(&mappings).Error; err != nil {
		d.logger.Warn("query group mappings failed", zap.Error(err))
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]string, len(mappings))
	for _, m := range mappings {
		if m.SiteName != "" {
			d.cache[m.GroupName] = m.SiteName
		}
	}
}

// HasGroupMappings 检查站点是否有官组映射
func (d *SourceSiteDetector) HasGroupMappings(ctx context.Context, site *model.Site) bool {
	if site == nil {
		return false
	}
	var count int64
	d.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
		Where("site_name = ? OR domain LIKE ?", site.Name, "%"+strings.ToLower(site.Domain)+"%").
		Count(&count)
	return count > 0
}

// getSourcePriority §56.40: 从 system_settings 读取用户配置的源站点优先级。
func (d *SourceSiteDetector) getSourcePriority(ctx context.Context) []string {
	var val string
	if err := d.db.WithContext(ctx).Raw("SELECT value FROM system_settings WHERE key = 'source_priority' LIMIT 1").Row().Scan(&val); err != nil {
		return nil
	}
	var priority []string
	if err := json.Unmarshal([]byte(val), &priority); err != nil {
		return nil
	}
	return priority
}

// getFetchPriority §59.20: 从 system_settings 读取用户配置的获取优先级（与 source_priority 独立）。
func (d *SourceSiteDetector) getFetchPriority(ctx context.Context) []string {
	var val string
	if err := d.db.WithContext(ctx).Raw("SELECT value FROM system_settings WHERE key = 'fetch_priority' LIMIT 1").Row().Scan(&val); err != nil {
		return nil
	}
	var priority []string
	if err := json.Unmarshal([]byte(val), &priority); err != nil {
		return nil
	}
	return priority
}

// SelectFetchSite §59.20: "获取数据"时的站点选择（制作组优先 → fetch_priority → 兜底）。
// 与 Detect()（发布流程，source_priority 优先）的区别：制作组映射优先于 fetch_priority。
func (d *SourceSiteDetector) SelectFetchSite(ctx context.Context, title string, coverageSites []model.SiteCoverageCache) SourceDetectResult {
	result := SourceDetectResult{}

	siteMap := make(map[string]model.SiteCoverageCache)
	for _, c := range coverageSites {
		if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
			siteMap[c.SiteName] = c
		}
	}

	// ① 制作组映射 → 源站（有 cookie AND 在覆盖列表中）
	groupName := ExtractGroupName(title)
	result.GroupName = groupName
	if groupName != "" {
		siteName := d.LookupGroup(ctx, groupName)
		if siteName != "" {
			// 优先：coverage 命中 + cookie → 有 tid 可直接获取
			if c, ok := siteMap[siteName]; ok {
				var site model.Site
				if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
					result.SourceSite = site.Name
					result.SourceSiteID = site.ID
					result.TorrentID = c.TorrentID
					result.AutoDetected = true
					return result
				}
			}
			// §59.25: 无 coverage 时也允许选站（batch-fetch 场景），用搜索方式获取
			var site model.Site
			if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
				result.SourceSite = site.Name
				result.SourceSiteID = site.ID
				result.AutoDetected = true
				return result
			}
		}
	}

	// ② fetch_priority ∩ coverage ∩ cookie
	if priority := d.getFetchPriority(ctx); len(priority) > 0 {
		for _, siteName := range priority {
			if c, ok := siteMap[siteName]; ok {
				var site model.Site
				if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
					result.SourceSite = site.Name
					result.SourceSiteID = site.ID
					result.TorrentID = c.TorrentID
					result.AutoDetected = false
					return result
				}
			}
		}
	}

	// ③ coverage ∩ cookie 兜底
	for siteName, c := range siteMap {
		var site model.Site
		if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
			result.SourceSite = site.Name
			result.SourceSiteID = site.ID
			result.TorrentID = c.TorrentID
			result.AutoDetected = false
			return result
		}
	}

	return result
}
