package publish

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/setting"
	"gorm.io/gorm"
)

const deletedGroupsKey = "deleted_group_mappings"

type SourceSiteDetector struct {
	db       *gorm.DB
	settings *setting.Repository
	cache    map[string]string // group_name → site_name
	deleted  map[string]bool   // 用户主动删除的 group_name
	mu       sync.RWMutex
}

func NewSourceSiteDetector(db *gorm.DB) *SourceSiteDetector {
	return &SourceSiteDetector{
		db:       db,
		settings: setting.NewRepository(db),
		cache:    make(map[string]string),
		deleted:  make(map[string]bool),
	}
}

func (d *SourceSiteDetector) loadDeletedSet(ctx context.Context) {
	val, err := d.settings.Get(ctx, deletedGroupsKey)
	if err != nil || val == "" {
		return
	}
	var list []string
	if json.Unmarshal([]byte(val), &list) == nil {
		d.mu.Lock()
		for _, g := range list {
			d.deleted[g] = true
		}
		d.mu.Unlock()
	}
}

func (d *SourceSiteDetector) saveDeletedSet(ctx context.Context) {
	d.mu.RLock()
	list := make([]string, 0, len(d.deleted))
	for g := range d.deleted {
		list = append(list, g)
	}
	d.mu.RUnlock()
	data, _ := json.Marshal(list)
	_ = d.settings.Set(ctx, deletedGroupsKey, string(data))
}

func (d *SourceSiteDetector) MarkDeleted(ctx context.Context, groupName string) {
	d.mu.Lock()
	d.deleted[groupName] = true
	d.mu.Unlock()
	d.saveDeletedSet(ctx)
}

func (d *SourceSiteDetector) UnmarkDeleted(ctx context.Context, groupName string) {
	d.mu.Lock()
	delete(d.deleted, groupName)
	d.mu.Unlock()
	d.saveDeletedSet(ctx)
}

func (d *SourceSiteDetector) isDeleted(groupName string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.deleted[groupName]
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

// groupDomainSeed: 制作组名 → 站点域名
// 数据源: examples/auto_feed_js default_site_info + reg_team_name
// 一个站点多个官组后缀 = 多行
var groupDomainSeed = map[string]string{
	// springsunday.net (CMCT 系列)
	"CMCT": "springsunday.net",
	"CMCTV": "springsunday.net",

	// m-team (馒头)
	"MTeam": "m-team",

	// hdhome.org (家园)
	"HDHome": "hdhome.org",

	// ptchdbits.co (彩虹岛)
	"CHDBits": "ptchdbits.co",
	"CHD":     "ptchdbits.co",
	"CHDPAD":  "ptchdbits.co",
	"CHDTV":   "ptchdbits.co",
	"CHDWEB":  "ptchdbits.co",

	// hdsky.me (天空)
	"HDSky":  "hdsky.me",
	"HDSPAD": "hdsky.me",
	"HDSWEB": "hdsky.me",
	"HDSTV":  "hdsky.me",
	"HDS":    "hdsky.me",

	// pthome.net (铂金家)
	"PTHome": "pthome.net",
	"PTH":    "pthome.net",

	// totheglory.im (套套哥)
	"TTG": "totheglory.im",

	// ourbits.club (我堡)
	"OurBits": "ourbits.club",

	// hdroute.org
	"HDRoute": "hdroute.org",

	// hddolby.com (多个官组)
	"HDDolby":   "hddolby.com",
	"DBTV":      "hddolby.com",
	"QHstudIo":  "hddolby.com",
	"Dream":     "hddolby.com",

	// hdarea.club
	"HDArea": "hdarea.club",

	// hdtime.org
	"HDTime": "hdtime.org",

	// hdvideo.top (多个官组)
	"HDVideo": "hdvideo.top",
	"HDVWEB":  "hdvideo.top",
	"HDVMV":   "hdvideo.top",

	// joyhd.net
	"JoyHD": "joyhd.net",

	// btschool.club
	"BTSchool": "btschool.club",

	// tjupt.org (北洋园)
	"TJUPT": "tjupt.org",

	// sjtu.edu.cn (葡萄)
	"PuTao": "sjtu.edu.cn",
	"putao": "sjtu.edu.cn",

	// hdchina (高清瓦堡)
	"HDChina":   "hdchina",
	"HDC":       "hdchina",

	// agsvpt.com (末日) — 9 个官组后缀
	"AGSV":      "agsvpt.com",
	"AGSVPT":    "agsvpt.com",
	"AGSVE":     "agsvpt.com",
	"AGSVWEB":   "agsvpt.com",
	"AGSVREMUX": "agsvpt.com",
	"AGSVRip":   "agsvpt.com",
	"AGSVTV":    "agsvpt.com",
	"AGSVDIY":   "agsvpt.com",
	"AGSVMUS":   "agsvpt.com",

	// cyanbug.net (青虫)
	"CyanBug": "cyanbug.net",

	// yemapt.org (野马)
	"YemaPT": "yemapt.org",

	// pterclub.net (大象)
	"PTer": "pterclub.net",

	// piggo.me (猪猪) — 3 个官组
	"PigGo":  "piggo.me",
	"PigoHD": "piggo.me",
	"PigoWeb": "piggo.me",
	"PiGoNF": "piggo.me",

	// open.cd (音乐站)
	"OpenCD": "open.cd",

	// nanyangpt.com (南洋)
	"NYTV":   "nanyangpt.com",

	// pt.eastgame.org (TLF)
	"TLF":    "eastgame.org",

	// darkland.top
	"DarkLand": "darkland.top",

	// soulvoice.club (灵魂之声)
	"SoulVoice": "soulvoice.club",

	// icc2022.com
	"ICC": "icc2022.com",

	// et8.org (高教)
	"TCCF": "et8.org",

	// pt.upxin.net
	"HDU": "upxin.net",

	// hitpt.com
	"HITPT": "hitpt.com",

	// haidan.video (海胆)
	"HaiDan": "haidan.video",

	// discfan.net
	"DiscFan": "discfan.net",

	// pt.0ff.cc
	"FreeFarm": "0ff.cc",

	// ptlgs.org
	"PTLGS": "ptlgs.org",

	// hdfans.org
	"HDFans": "hdfans.org",

	// 52movie.top
	"52MOVIE": "52movie.top",

	// u2.dmhy.org
	"U2": "u2.dmhy.org",

	// carpt.net
	"CarPT": "carpt.net",

	// wintersakura.net (樱花) — 3 个官组
	"SakuraWEB": "wintersakura.net",
	"SakuraSUB": "wintersakura.net",
	"WScode":    "wintersakura.net",

	// star-space.net (影) — 6 个官组
	"Ying":      "star-space.net",
	"YingWEB":   "star-space.net",
	"YingDIY":   "star-space.net",
	"YingTV":    "star-space.net",
	"YingMV":    "star-space.net",
	"YingMUSIC": "star-space.net",

	// hdkyl.in (麒麟) — 6 个官组
	"HDKWEB":   "hdkyl.in",
	"HDKTV":    "hdkyl.in",
	"HDKMV":    "hdkyl.in",
	"HDKGame":  "hdkyl.in",
	"HDKDIY":   "hdkyl.in",
	"HDKylin":  "hdkyl.in",

	// hares web
	"HaresWEB": "hares.club",
	"HaresTV":  "hares.club",

	// panda
	"AilMWeb": "pandapt.net",
	"PANDA":   "pandapt.net",

	// ubits
	"UBWEB": "ubits.club",

	// ptcafe
	"CafeWEB": "ptcafe.club",
	"CafeTV":  "ptcafe.club",

	// dj
	"DJWEB": "dajiao",
	"DJTV":  "dajiao",

	// okpt
	"OK":    "okpt.net",
	"OKWEB": "okpt.net",

	// filelist
	"PlayHD":  "filelist.io",
	"PlaySD":  "filelist.io",
	"PlayWEB": "filelist.io",
	"PlayTV":  "filelist.io",

	// crabpt
	"XHBWeb": "crabpt.vip",

	// 红叶
	"RLWEB":   "lemonhd.net",
	"RLeaves": "lemonhd.net",
	"RLTV":    "lemonhd.net",

	// 青蛙
	"FROG":    "qingwapt.com",
	"FROGE":   "qingwapt.com",
	"FROGWeb": "qingwapt.com",

	// zmpt
	"ZmWeb": "zmpt.cc",
	"ZmPT":  "zmpt.cc",

	// ptsbao
	"FFans": "ptsbao.club",
	"sBao":  "ptsbao.club",
	"FHDMV": "ptsbao.club",

	// 13city
	"13City": "13city.org",

	// FRDS
	"FRDS": "frds",

	// HHClub
	"HHWEB": "hhclub",

	// QingWa
	"QingWa": "qingwapt.com",
}

func (d *SourceSiteDetector) SeedDefaultMappings(ctx context.Context) error {
	d.loadDeletedSet(ctx)

	var existingNames []string
	d.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).Pluck("group_name", &existingNames)
	existingSet := make(map[string]bool, len(existingNames))
	for _, name := range existingNames {
		existingSet[name] = true
	}

	for group, domain := range groupDomainSeed {
		if existingSet[group] || d.isDeleted(group) {
			continue
		}
		d.db.WithContext(ctx).Create(&model.ReleaseGroupMapping{
			GroupName:  group,
			Domain:     domain,
			IsOfficial: true,
		})
	}

	d.RefreshCache(ctx)
	return nil
}

// SyncSiteGroups 站点添加/更新时，自动从 groupDomainSeed 匹配并创建该站的官组映射
func (d *SourceSiteDetector) SyncSiteGroups(ctx context.Context, site *model.Site) {
	if site == nil {
		return
	}
	siteDomain := strings.ToLower(site.Domain)
	siteBaseURL := strings.ToLower(site.BaseURL)

	for group, seedDomain := range groupDomainSeed {
		seedDomainLower := strings.ToLower(seedDomain)
		bareSeed := strings.TrimPrefix(seedDomainLower, "www.")
		bareSite := strings.TrimPrefix(siteDomain, "www.")

		matched := strings.Contains(bareSite, bareSeed) ||
			strings.Contains(siteBaseURL, seedDomainLower)

		if !matched || d.isDeleted(group) {
			continue
		}

		// 检查是否已存在（按 group_name + domain 去重）
		var count int64
		d.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
			Where("group_name = ?", group).Count(&count)
		if count > 0 {
			continue
		}

		d.db.WithContext(ctx).Create(&model.ReleaseGroupMapping{
			GroupName:  group,
			Domain:     site.Domain,
			SiteName:   site.Name,
			IsOfficial: true,
		})
	}

	d.RefreshCache(ctx)
}
