package publish

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
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

// ExtractGroupName 从标题末尾提取制作组名
func ExtractGroupName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	idx := strings.LastIndex(title, "-")
	if idx > 0 && idx < len(title)-1 {
		raw := strings.TrimSpace(title[idx+1:])
		upper := strings.ToUpper(raw)
		ignore := map[string]bool{
			"NOGROUP": true, "N/A": true, "NONE": true, "UNKNOWN": true,
		}
		if !ignore[upper] && len(raw) >= 2 && len(raw) <= 30 {
			return raw
		}
	}

	// ￡ 分隔符（SSD 特有格式 ￡CMCT发布者，取连续英文 = 组名）
	if pIdx := strings.LastIndex(title, "￡"); pIdx >= 0 {
		rest := title[pIdx+len("￡"):]
		var b strings.Builder
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				b.WriteRune(r)
			} else {
				break
			}
		}
		if b.Len() >= 2 {
			return b.String()
		}
	}

	return ""
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
		siteName := d.lookupGroup(ctx, groupName)
		if siteName != "" {
			var site model.Site
			if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ?", siteName, true).First(&site).Error; err == nil {
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

func (d *SourceSiteDetector) lookupGroup(ctx context.Context, groupName string) string {
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
