package publish

import (
	"context"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

type SourceSiteDetector struct {
	db    *gorm.DB
	cache map[string]string // group_name → site_name
	mu    sync.RWMutex
}

func NewSourceSiteDetector(db *gorm.DB) *SourceSiteDetector {
	return &SourceSiteDetector{db: db, cache: make(map[string]string)}
}

// ExtractGroupName 从标题末尾提取制作组名
func ExtractGroupName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	idx := strings.LastIndex(title, "-")
	if idx <= 0 || idx >= len(title)-1 {
		return ""
	}
	raw := strings.TrimSpace(title[idx+1:])
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

func (d *SourceSiteDetector) Detect(ctx context.Context, title, infoHash string, coverageSites []model.SiteCoverageCache) SourceDetectResult {
	result := SourceDetectResult{}

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

	// 按 domain 模糊匹配站点表
	if mapping.Domain != "" {
		var site model.Site
		pattern := "%" + strings.ToLower(mapping.Domain) + "%"
		if err := d.db.WithContext(ctx).
			Where("LOWER(domain) LIKE ? OR LOWER(base_url) LIKE ?", pattern, pattern).
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
	d.db.WithContext(ctx).Find(&mappings)

	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]string, len(mappings))
	for _, m := range mappings {
		if m.SiteName != "" {
			d.cache[m.GroupName] = m.SiteName
		}
	}
}

// groupDomainSeed: 制作组名 → 站点域名（数据源: examples/auto_feed_js）
// site_name 留空，运行时按 domain 匹配站点表，用户可在 Web 页面修正
var groupDomainSeed = map[string]string{
	// springsunday.net
	"CMCT":  "springsunday.net",
	"CMCTV": "springsunday.net",
	// m-team
	"MTeam": "m-team",
	// hdhome.org
	"HDHome": "hdhome.org",
	// ptchdbits.co
	"CHDBits": "ptchdbits.co",
	"CHD":     "ptchdbits.co",
	"CHDPAD":  "ptchdbits.co",
	"CHDTV":   "ptchdbits.co",
	"CHDWEB":  "ptchdbits.co",
	// hdsky.me
	"HDSky":  "hdsky.me",
	"HDSPAD": "hdsky.me",
	"HDSWEB": "hdsky.me",
	"HDSTV":  "hdsky.me",
	"HDS":    "hdsky.me",
	// pthome.net
	"PTHome": "pthome.net",
	"PTH":    "pthome.net",
	// totheglory.im
	"TTG": "totheglory.im",
	// ourbits.club
	"OurBits": "ourbits.club",
	// hdroute.org
	"HDRoute": "hdroute.org",
	// hddolby.com
	"HDDolby": "hddolby.com",
	// hdarea.club
	"HDArea": "hdarea.club",
	// hdtime.org
	"HDTime": "hdtime.org",
	// hdvideo.top
	"HDVideo": "hdvideo.top",
	// joyhd.net
	"JoyHD": "joyhd.net",
	// btschool
	"BTSchool": "btschool.club",
	// tjupt.org
	"TJUPT": "tjupt.org",
	// sjtu
	"PuTao": "sjtu.edu.cn",
	// hdchina
	"HDChina": "hdchina",
	"HDC":     "hdchina",
	// agsvpt.com
	"AGSV": "agsvpt.com",
	// cyanbug.net
	"CyanBug": "cyanbug.net",
	// yemapt.org
	"YemaPT": "yemapt.org",
	// pterclub.net
	"PTer": "pterclub.net",
	// piggo.me
	"PigGo": "piggo.me",
	// open.cd
	"OpenCD": "open.cd",
	// hdchina official group
	"DONATELLA": "hdchina",
	// 52movie
	"52MOVIE": "52movie.top",
	// u2
	"U2": "u2.dmhy.org",
	// DarkLand
	"DarkLand": "darkland.top",
	// SoulVoice
	"SoulVoice": "soulvoice.club",
	// ICC
	"ICC": "icc2022.com",
	// TCCF / et8
	"TCCF": "et8.org",
	// HDU
	"HDU": "upxin.net",
	// HitPT
	"HITPT": "hitpt.com",
	// haidan
	"HaiDan": "haidan.video",
	// DiscFan
	"DiscFan": "discfan.net",
	// FreeFarm
	"FreeFarm": "0ff.cc",
}

func (d *SourceSiteDetector) SeedDefaultMappings(ctx context.Context) error {
	var count int64
	d.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).Count(&count)
	if count > 0 {
		return nil
	}

	for group, domain := range groupDomainSeed {
		d.db.WithContext(ctx).Where("group_name = ?", group).
			FirstOrCreate(&model.ReleaseGroupMapping{
				GroupName:  group,
				Domain:     domain,
				IsOfficial: true,
			})
	}

	d.RefreshCache(ctx)
	return nil
}
