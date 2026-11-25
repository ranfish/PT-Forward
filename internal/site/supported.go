package site

import (
	"encoding/json"
	"sort"
	"sync"
)

// SupportedSite 描述系统内置的"白名单站点"。
// 数据来源：internal/site/supported_sites.json（go:embed 只读 seed）。
// 用户在 UI 添加站点时，必须从该列表选择，禁止自由输入 domain。
type SupportedSite struct {
	Domain              string `json:"domain"`
	NameCN              string `json:"name_cn"`
	Framework           string `json:"framework"`
	AuthType            string `json:"auth_type"`
	CookiecloudDomain   string `json:"cookiecloud_domain"`
	DownloadURLTemplate string `json:"download_url_template"`
	// §59.478: 详情页 URL 模板（空=框架默认 /details.php?id={id}·
	// 皇后 plugin_details.php 老种新版面案）
	DetailsURLTemplate string            `json:"details_url_template,omitempty"`
	PasskeyLabel       string            `json:"passkey_label,omitempty"`
	PasskeyHint        string            `json:"passkey_hint,omitempty"`
	APIKeyLabel        string            `json:"api_key_label,omitempty"`
	APIKeyHint         string            `json:"api_key_hint,omitempty"`
	RSSKeyLabel        string            `json:"rss_key_label,omitempty"`
	RSSKeyHint         string            `json:"rss_key_hint,omitempty"`
	ShowCookie         *bool             `json:"show_cookie,omitempty"`
	ShowPasskey        *bool             `json:"show_passkey,omitempty"`
	ShowAPIKey         *bool             `json:"show_apikey,omitempty"`
	ShowRSSKey         *bool             `json:"show_rsskey,omitempty"`
	ShowBearerToken    *bool             `json:"show_bearer_token,omitempty"`
	ShowAuthKey        *bool             `json:"show_authkey,omitempty"`
	ShowAuthHash       *bool             `json:"show_authhash,omitempty"`
	ShowUserID         *bool             `json:"show_userid,omitempty"`
	PublishFormFields  map[string]string `json:"publish_form_fields,omitempty"`
	Paths              map[string]string `json:"paths,omitempty"`
	VerificationStatus string            `json:"verification_status"`
	SpecialNotes       string            `json:"special_notes,omitempty"`
	// §59.318: 发布适配支持（开发者权威位——逐站适配完成随版本置 true；
	// 用户侧 is_target(发布开关) 的门槛。当前：修道院/幸运）。
	PublishSupported bool `json:"publish_supported,omitempty"`
}

var (
	supportedSitesOnce sync.Once
	supportedSitesMap  map[string]*SupportedSite
	supportedSitesList []*SupportedSite
	supportedSitesErr  error
)

// loadSupportedSites 单次解析 embedded JSON 到内存索引。
// 失败时 supportedSitesErr 非 nil，后续调用不会重试（sync.Once 语义）。
func loadSupportedSites() {
	supportedSitesOnce.Do(func() {
		var m map[string]SupportedSite
		if err := json.Unmarshal(SupportedSitesJSON, &m); err != nil {
			supportedSitesErr = err
			return
		}
		supportedSitesMap = make(map[string]*SupportedSite, len(m))
		supportedSitesList = make([]*SupportedSite, 0, len(m))
		for k := range m {
			s := m[k]
			supportedSitesMap[k] = &s
			supportedSitesList = append(supportedSitesList, &s)
		}
		sort.Slice(supportedSitesList, func(i, j int) bool {
			return supportedSitesList[i].Domain < supportedSitesList[j].Domain
		})
	})
}

// GetSupportedSite 按 domain 查询；找不到返回 (nil, false)。
func GetSupportedSite(domain string) (*SupportedSite, bool) {
	loadSupportedSites()
	if supportedSitesErr != nil {
		return nil, false
	}
	s, ok := supportedSitesMap[domain]
	return s, ok
}

// IsPublishSupported §59.318→§59.571 双源: 站点是否已适配发布。
// 源① supported_sites.json publish_supported=true（静态声明·兼容存量 69 站）
// 源② sites 表 form_config enabled=true（migration 交付即生效——适配完成
//
//	的交付物就是 form_config·它的存在就是证明）
//
// 满足任一即 true——json 标记降级为「预声明」而非「门卫」。
// §59.571 定案：单环境用户 OTA → migration → form_config → 直接可开目标站
// （手工 json 同步遗漏不再阻断用户——乌托邦/朱雀/海豹/肉丝/野马五站案驱动）。
// 不在白名单的站（理论不存在——强白名单体系）恒 false。
func IsPublishSupported(domain string) bool {
	s, ok := GetSupportedSite(domain)
	if !ok {
		return false // §59.318: 白名单外恒 false（强白名单前置门）
	}
	if s.PublishSupported {
		return true // 源一：json 静态声明
	}
	// §59.571: 源二——form_config 派生（白名单内站·migration 交付即生效）
	return siteFormConfigEnabled(domain)
}

// siteFormConfigEnabled §59.571: DB sites 表 form_config enabled=true 判定。
// 闭包 DB 访问（site 包不直接依赖 gorm——由 SetFormConfigChecker 注入）。
var formConfigChecker func(domain string) bool

// SetFormConfigChecker §59.571: main 启动时注入 DB 查询闭包。
// site 包保持零 DB 依赖（包边界不破）。
func SetFormConfigChecker(fn func(domain string) bool) {
	formConfigChecker = fn
}

func siteFormConfigEnabled(domain string) bool {
	if formConfigChecker == nil {
		return false
	}
	return formConfigChecker(domain)
}

// ListSupportedSites 返回按 domain 升序的全部白名单站点。
func ListSupportedSites() []*SupportedSite {
	loadSupportedSites()
	return supportedSitesList
}

// SupportedSitesErr 返回 embedded JSON 解析错误（如有）。
func SupportedSitesErr() error {
	loadSupportedSites()
	return supportedSitesErr
}
