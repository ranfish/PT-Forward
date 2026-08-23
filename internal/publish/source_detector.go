package publish

import (
	"github.com/ranfish/pt-forward/internal/comment"
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
// SelectFetchSite §59.61: clusterTargets = 簇 comment 解析出的直达候选
// （(client_id, save_path, name) 群内各副本 comment 聚合，调用方经
// GetClusterComments + comment.Resolve 产出）。官组链消费指向源站的候选（J1/J1b），
// 非官组链消费任意候选（J2）。nil 表示无簇数据（旧调用方等价行为）。
func (d *SourceSiteDetector) SelectFetchSite(ctx context.Context, title string, coverageSites []model.SiteCoverageCache, clusterTargets []comment.DirectTarget) SourceDetectResult {
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
			// §59.61 ①c: 簇 comment tid（J1 源站副本自指 / J1b 溯源指向源站）——
			// 指向源站的直达候选即原发页凭证，优先于 coverage_tid（确定性证据：
			// 文件出生地声明 vs 系统历史缓存）
			for _, ct := range clusterTargets {
				if ct.SiteName != siteName || ct.TorrentID == "" {
					continue
				}
				var site2 model.Site
				if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site2).Error; err == nil {
					result.SourceSite = site2.Name
					result.SourceSiteID = site2.ID
					result.TorrentID = ct.TorrentID
					result.AutoDetected = true
					d.logSelect("①组映射+comment_tid", groupName, site2.Name, ct.TorrentID, "cluster")
					return result
				}
			}
			// 优先：coverage 命中 + cookie → 有 tid 可直接获取
			if c, ok := siteMap[siteName]; ok {
				var site model.Site
				if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
					result.SourceSite = site.Name
					result.SourceSiteID = site.ID
					result.TorrentID = c.TorrentID
					result.AutoDetected = true
					// §59.59: 层选可观测——审计"匹配源站"正确性的回放依据（永久保留）
					d.logSelect("①组映射+coverage_tid", groupName, site.Name, c.TorrentID, c.Source)
					return result
				}
			}
			// §59.25: 无 coverage 时也允许选站（batch-fetch 场景），用搜索方式获取
			var site model.Site
			if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
				result.SourceSite = site.Name
				result.SourceSiteID = site.ID
				result.AutoDetected = true
				d.logSelect("①b组映射_搜索", groupName, site.Name, "", "")
				return result
			}
		}
	}

	// §59.61 ②a: 非官组簇 comment 直达（J2）——无组映射种子，
	// 簇内任意副本 comment 可解析即直达该站（PT0 实测 99% 覆盖）
	for _, ct := range clusterTargets {
		if ct.TorrentID == "" {
			continue
		}
		var site2 model.Site
		if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", ct.SiteName, true).First(&site2).Error; err == nil {
			result.SourceSite = site2.Name
			result.SourceSiteID = site2.ID
			result.TorrentID = ct.TorrentID
			result.AutoDetected = false
			d.logSelect("②a簇comment_tid", groupName, site2.Name, ct.TorrentID, "cluster")
			return result
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
					d.logSelect("②priority+coverage_tid", groupName, site.Name, c.TorrentID, c.Source)
					return result
				}
			}
		}
	}

	// ③ coverage ∩ cookie 兜底（map 遍历无序——tid 来源为确定性证据时直达，
	// §59.59 Q3b 定案保持现状）
	for siteName, c := range siteMap {
		var site model.Site
		if err := d.db.WithContext(ctx).Where("name = ? AND enabled = ? AND cookie != ''", siteName, true).First(&site).Error; err == nil {
			result.SourceSite = site.Name
			result.SourceSiteID = site.ID
			result.TorrentID = c.TorrentID
			result.AutoDetected = false
			d.logSelect("③coverage兜底", groupName, site.Name, c.TorrentID, c.Source)
			return result
		}
	}

	return result
}

// logSelect §59.59: 选站层选日志——记录命中层级/组名/站点/tid 及其证据来源，
// "为什么从这站获取"的永久回放依据。layer 为空串时不记（防御）。
func (d *SourceSiteDetector) logSelect(layer, groupName, siteName, torrentID, evidence string) {
	if d.logger == nil || layer == "" {
		return
	}
	d.logger.Info("source site selected",
		zap.String("layer", layer),
		zap.String("group", groupName),
		zap.String("site", siteName),
		zap.String("tid", torrentID),
		zap.String("evidence", evidence))
}
