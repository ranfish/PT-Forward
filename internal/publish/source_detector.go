package publish

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

var groupSuffixRe = regexp.MustCompile(`-([A-Za-z0-9]+)$`)

type SourceSiteDetector struct {
	db    *gorm.DB
	cache map[string]string
	mu    sync.RWMutex
}

func NewSourceSiteDetector(db *gorm.DB) *SourceSiteDetector {
	return &SourceSiteDetector{db: db, cache: make(map[string]string)}
}

// ExtractGroupName 从标题末尾提取制作组名
// 例: "House.of.the.Dragon.S03E03...-CMCTV" → "CMCTV"
func ExtractGroupName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	idx := strings.LastIndex(title, "-")
	if idx <= 0 || idx >= len(title)-1 {
		return ""
	}
	raw := title[idx+1:]
	raw = strings.TrimSpace(raw)
	upper := strings.ToUpper(raw)
	ignore := map[string]bool{
		"NOGROUP": true, "N/A": true, "NONE": true, "UNKNOWN": true,
	}
	if ignore[upper] {
		return ""
	}
	if len(raw) < 2 || len(raw) > 30 {
		return ""
	}
	return raw
}

type SourceDetectResult struct {
	SourceSite    string
	SourceSiteID  uint
	GroupName     string
	TorrentID     string
	AutoDetected  bool
}

// Detect 从标题和覆盖数据判断源头站
func (d *SourceSiteDetector) Detect(ctx context.Context, title, infoHash string, coverageSites []model.SiteCoverageCache) SourceDetectResult {
	result := SourceDetectResult{}

	// 1. 制作组 → 站点映射
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
				// 从覆盖数据中找 torrent_id
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

	// 2. 降级：从覆盖数据中找第一个有 cookie 的站
	siteMap := make(map[string]model.SiteCoverageCache)
	for _, c := range coverageSites {
		if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
			siteMap[c.SiteName] = c
		}
	}
	if len(siteMap) > 0 {
		var sites []model.Site
		d.db.WithContext(ctx).Where("enabled = ? AND is_source = ?", true, true).Find(&sites)
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
		// 没有 is_source 的站有覆盖，取第一个有覆盖的
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

	// 大小写不敏感查询
	var mapping model.ReleaseGroupMapping
	if err := d.db.WithContext(ctx).
		Where("LOWER(group_name) = LOWER(?)", groupName).
		First(&mapping).Error; err == nil {
		d.mu.Lock()
		d.cache[groupName] = mapping.SiteName
		d.mu.Unlock()
		return mapping.SiteName
	}

	return ""
}

func (d *SourceSiteDetector) RefreshCache(ctx context.Context) {
	var mappings []model.ReleaseGroupMapping
	d.db.WithContext(ctx).Find(&mappings)

	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]string, len(mappings))
	for _, m := range mappings {
		d.cache[m.GroupName] = m.SiteName
	}
}
