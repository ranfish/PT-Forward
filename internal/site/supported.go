package site

import (
	"encoding/json"
	"sort"
	"strings"
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
	// 14 刀2（v0.0.1659）: 去 omitempty——false 是有意义值（未适配站
	// 是常态·键恒存在〔V-02 契约对齐：原 false 时键消失·前端
	// types.ts 声明 boolean 非 optional 契约撒谎〕）。同结构体 8 个
	// *bool 用指针+omitempty 区分「false 与未设置」——本位是开发者
	// 权威数据源（seed 必填）无「未设置」态·值类型+无 omitempty 即对。
	PublishSupported bool `json:"publish_supported"`
	// §59.580: 站点级禁转小组黑名单（站方明文谢绝的小组——瓷器
	// rules.php §6.2 实测）。硬编码随版本（前端不可增删改——站方
	// 规则是外部权威非用户业务偏好）。通用字段：任意站可配·当前
	// 只填瓷器；exact=精确匹配（EqualFold）·prefixes=前缀族
	// （CHD*/Hyperay*——「任何官方组」宁严不宽）。
	BannedGroups *BannedGroupsConfig `json:"banned_groups,omitempty"`
}

// BannedGroupsConfig §59.580: 站点禁转小组黑名单配置。
type BannedGroupsConfig struct {
	Exact    []string `json:"exact"`
	Prefixes []string `json:"prefixes"`
}

// Match §59.580: 组名是否命中黑名单（exact 大小写不敏感+前缀族）。
func (b *BannedGroupsConfig) Match(group string) bool {
	if b == nil || group == "" {
		return false
	}
	for _, e := range b.Exact {
		if strings.EqualFold(group, e) {
			return true
		}
	}
	for _, p := range b.Prefixes {
		if len(group) >= len(p) && strings.EqualFold(group[:len(p)], p) {
			return true
		}
	}
	return false
}

var (
	supportedSitesOnce sync.Once
	supportedSitesMap  map[string]*SupportedSite
	supportedSitesList []*SupportedSite
	supportedSitesErr  error
	// §59.580 回归审核：name_cn 索引（双键 O(1)——原 O(105) 遍历在
	// 辅种批量下 820 源×105 站×每次回查=900 万次比较的性能隐患）。
	supportedSitesByNameCN map[string]*SupportedSite
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
		supportedSitesByNameCN = make(map[string]*SupportedSite, len(m))
		for k := range m {
			s := m[k]
			supportedSitesMap[k] = &s
			supportedSitesList = append(supportedSitesList, &s)
			if s.NameCN != "" {
				supportedSitesByNameCN[s.NameCN] = &s
			}
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

// BannedGroupsForSite §59.580: 按站名或域名查禁转小组黑名单
// （双键——发布/辅种消费点持站名〔sites.name=name_cn〕·辅种
// SiteInfo 无 Domain 字段·domain 直查+name_cn 回查双保险）。
// 硬编码数据源=supported_sites.json（前端不可增删改）。
// 回归审核：name_cn 索引 O(1)（原 O(105) 遍历——辅种批量
// 820×105 调用面的性能修复）。
func BannedGroupsForSite(nameOrDomain string) *BannedGroupsConfig {
	loadSupportedSites()
	if supportedSitesErr != nil {
		return nil
	}
	if s, ok := supportedSitesMap[nameOrDomain]; ok {
		return s.BannedGroups
	}
	if s, ok := supportedSitesByNameCN[nameOrDomain]; ok {
		return s.BannedGroups
	}
	return nil
}

// GroupBannedOnSite §59.580: 组名在站点黑名单（便捷组合——
// 发布一种多站循环/辅种 matchAtSite 消费）。
func GroupBannedOnSite(group, nameOrDomain string) bool {
	return BannedGroupsForSite(nameOrDomain).Match(group)
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
