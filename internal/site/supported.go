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
	Domain              string            `json:"domain"`
	NameCN              string            `json:"name_cn"`
	Framework           string            `json:"framework"`
	AuthType            string            `json:"auth_type"`
	CookiecloudDomain   string            `json:"cookiecloud_domain"`
	DownloadURLTemplate string            `json:"download_url_template"`
	PasskeyLabel        string            `json:"passkey_label,omitempty"`
	PasskeyHint         string            `json:"passkey_hint,omitempty"`
	APIKeyLabel         string            `json:"api_key_label,omitempty"`
	APIKeyHint          string            `json:"api_key_hint,omitempty"`
	RSSKeyLabel         string            `json:"rss_key_label,omitempty"`
	RSSKeyHint          string            `json:"rss_key_hint,omitempty"`
	ShowCookie          *bool             `json:"show_cookie,omitempty"`
	ShowPasskey         *bool             `json:"show_passkey,omitempty"`
	ShowAPIKey          *bool             `json:"show_apikey,omitempty"`
	ShowRSSKey          *bool             `json:"show_rsskey,omitempty"`
	ShowBearerToken     *bool             `json:"show_bearer_token,omitempty"`
	ShowAuthKey         *bool             `json:"show_authkey,omitempty"`
	ShowAuthHash        *bool             `json:"show_authhash,omitempty"`
	ShowUserID          *bool             `json:"show_userid,omitempty"`
	PublishFormFields   map[string]string `json:"publish_form_fields,omitempty"`
	Paths               map[string]string `json:"paths,omitempty"`
	VerificationStatus  string            `json:"verification_status"`
	SpecialNotes        string            `json:"special_notes,omitempty"`
}

var (
	supportedSitesOnce  sync.Once
	supportedSitesMap   map[string]*SupportedSite
	supportedSitesList  []*SupportedSite
	supportedSitesErr   error
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
