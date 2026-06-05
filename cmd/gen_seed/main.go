// gen_seed 从 sites 表 + site_config_overrides 表 + 硬编码 frontendOverrides 生成
// internal/site/supported_sites.json（go:embed 只读 seed）。
//
// 用途：严格白名单设计（§30.6）— 系统支持的站点列表是嵌入到二进制的只读数据。
// 用户添加站点时必须从该白名单选择，前端 a-select 下拉 + 后端 handleCreate 强校验。
//
// 重新生成场景：
//   1. 新增/移除站点（修改了 sites 表或 frontendOverrides/site_config_overrides）
//   2. 修改了 SupportedSite schema（增减字段）
//   3. 框架默认 download_url_template 调整
//
// 用法：go run ./cmd/gen_seed/
// 输出：internal/site/supported_sites.json（107 站点，约 30KB）
//
// ⚠️ 注意：本脚本不修改 sites 表 / site_config_overrides 表，只读取并生成 seed。
//         迁移工作（如把 passkey_alias 从 site_config_overrides 迁到 seed）由步骤 6 完成。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ranfish/pt-forward/internal/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Site struct {
	ID                uint   `gorm:"primaryKey"`
	Domain            string `gorm:"column:domain"`
	Name              string `gorm:"column:name"`
	Framework         string `gorm:"column:framework"`
	AuthType          string `gorm:"column:auth_type"`
	CookieCloudDomain string `gorm:"column:cookie_cloud_domain"`
	DownloadURLTpl    string `gorm:"column:download_url_template"`
	BaseURL           string `gorm:"column:base_url"`
	IsSource          bool   `gorm:"column:is_source"`
	IsTarget          bool   `gorm:"column:is_target"`
}

func (Site) TableName() string { return "sites" }

type Override struct {
	SiteName   string `gorm:"column:site_name"`
	FieldPath  string `gorm:"column:field_path"`
	FieldValue string `gorm:"column:field_value"`
}

func (Override) TableName() string { return "site_config_overrides" }

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

// From site-overrides.ts (前端硬编码，迁入 seed)
// key 必须用 sites 表里的 domain（含 www. 前缀），与 normalizeDomain 无关
var frontendOverrides = map[string]SupportedSite{
	"api.m-team.cc": {
		APIKeyLabel: "存取令牌",
		APIKeyHint:  "请从 控制台-实验室 获取",
	},
	"www.yemapt.org": {
		ShowCookie:  boolPtr(true),
		APIKeyLabel: "API Auth Key",
		APIKeyHint:  "请从 个人面板-安全设定 页面创建，默认有效期 180 天",
	},
}

// 3 站 nexusphp 特殊处理：实际用 RSS Key 下载，不用 Passkey
// 把 RSS Key 字段从"基础凭据伪装"（passkey_alias）迁到"高级凭据真实字段"
var rssKeySites = map[string]bool{
	"hdhome.org":      true, // 家园
	"pthome.net":      true, // 铂金家
	"www.hddolby.com": true, // 不可杜
}

// From site-auth skill: blocked 站点（无法访问 / 已知故障）
var blockedDomains = map[string]string{
	"pt.cdy.skin":    "传道院 - HTTP 403 / 直连超时",
	"www.oshen.win":  "奥申 - 空页面",
	"piggo.me":       "二师兄 - CF 468 封锁服务器 IP（仅目标）",
	"www.joyhd.net":  "开心 - 站点故障",
	"www.ptlover.cc": "AFUN - 站点故障",
}

// From site-auth skill §3.1: special_notes
var specialNotes = map[string]string{
	"pt.keepfrds.com":       "仅源站",
	"pt.ptskit.org":         "仅源站",
	"u2.dmhy.org":           "仅源站",
	"ubits.club":            "仅源站",
	"ourbits.club":          "仅源站",
	"sunnypt.top":           "仅源站",
	"hhanclub.net":          "仅源站，全站官种",
	"hdroute.org":           "仅源站，http://",
	"piggo.me":              "仅目标",
	"bilibili.download":     "仅目标",
	"playlet.cc":            "无上传权限，仅辅种",
	"www.52movie.top":       "无上传权限，仅辅种",
	"www.tokyo-manga.top":   "无上传权限，仅辅种",
	"www.hddolby.com":       "需过 2FA 页",
	"www.qingwapt.com":      "CookieCloud 域名 qingwapt.com",
	"api.m-team.cc":         "CookieCloud 域名 m-team.cc；禁调 /login /admin/** /apikey/**",
	"www.yemapt.org":        "CookieCloud 域名 yemapt.org；上传走 /api/torrent/add JSON API",
}

// Framework 默认 download_url_template（仅用于 sites 表为空时补值）
var frameworkDownloadDefaults = map[string]string{
	"nexusphp": "download.php?id={id}&passkey={passkey}",
	"mteam":    "", // 走 genDlToken API
	"gazelle":  "torrents.php?action=download&id={id}&authkey={authkey}&torrent_pass={rsskey}",
	"unit3d":   "torrents/download/{id}.{rsskey}",
	"rousi":    "api/torrent/{id}/download/{passkey}",
	"tnode":    "api/torrent/download/{id}",
	"yemapt":   "", // 走自定义 API
	"generic":  "download.php?id={id}&passkey={passkey}",
}

func boolPtr(b bool) *bool { return &b }

func main() {
	db, err := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	enc, err := crypto.NewCredentialEncryptor("7e6ed63684b150592f2668ccb89eafe72ba189e659b1f619359b5c1a47a420f4")
	if err != nil {
		panic(err)
	}

	// Load sites
	var sites []Site
	if err := db.Find(&sites).Error; err != nil {
		panic(err)
	}

	// Load overrides
	var overrides []Override
	if err := db.Find(&overrides).Error; err != nil {
		panic(err)
	}

	// Group overrides by site_name → field_path → value
	overrideMap := make(map[string]map[string]string)
	for _, o := range overrides {
		val := o.FieldValue
		if enc.IsEncrypted(val) {
			if dec, err := enc.Decrypt(val); err == nil {
				val = dec
			}
		}
		if _, ok := overrideMap[o.SiteName]; !ok {
			overrideMap[o.SiteName] = make(map[string]string)
		}
		overrideMap[o.SiteName][o.FieldPath] = val
	}

	// Build supported_sites map
	result := make(map[string]SupportedSite)
	for _, s := range sites {
		domain := s.Domain
		ccDomain := s.CookieCloudDomain
		if ccDomain == "" {
			ccDomain = domain
		}

		entry := SupportedSite{
			Domain:            domain,
			NameCN:            s.Name,
			Framework:         s.Framework,
			AuthType:          s.AuthType,
			CookiecloudDomain: ccDomain,
			DownloadURLTemplate: s.DownloadURLTpl,
			VerificationStatus:  "verified",
		}

		// Framework 默认补 download_url_template
		// 特殊 framework（adapter 不读 sites 表，强制用 framework 默认）
		forceFramework := map[string]bool{"tnode": true, "yemapt": true, "mteam": true}
		if entry.DownloadURLTemplate == "" || forceFramework[s.Framework] {
			if def, ok := frameworkDownloadDefaults[s.Framework]; ok {
				entry.DownloadURLTemplate = def
			}
		}

		// blocked?
		if reason, ok := blockedDomains[domain]; ok {
			entry.VerificationStatus = "blocked"
			entry.SpecialNotes = reason
		}

		// special_notes (not blocked)
		if notes, ok := specialNotes[domain]; ok && entry.VerificationStatus != "blocked" {
			entry.SpecialNotes = notes
		}

		// Apply site_config_overrides
		if ov, ok := overrideMap[s.Name]; ok {
			if v, ok := ov["download_url_template"]; ok && v != "" {
				entry.DownloadURLTemplate = v
			}
			if v, ok := ov["passkey_alias"]; ok {
				entry.PasskeyLabel = v
			}
			if v, ok := ov["passkey_hint"]; ok {
				entry.PasskeyHint = v
			}
			// publish.form_fields.*
			for k, v := range ov {
				if strings.HasPrefix(k, "publish.form_fields.") {
					fieldName := strings.TrimPrefix(k, "publish.form_fields.")
					if entry.PublishFormFields == nil {
						entry.PublishFormFields = make(map[string]string)
					}
					entry.PublishFormFields[fieldName] = v
				}
			}
			// paths.browse
			if v, ok := ov["paths.browse"]; ok {
				if entry.Paths == nil {
					entry.Paths = make(map[string]string)
				}
				entry.Paths["browse"] = v
			}
		}

		// Apply frontend overrides (apiKeyLabel/apiKeyHint/show_*)
		if fe, ok := frontendOverrides[domain]; ok {
			if fe.APIKeyLabel != "" {
				entry.APIKeyLabel = fe.APIKeyLabel
			}
			if fe.APIKeyHint != "" {
				entry.APIKeyHint = fe.APIKeyHint
			}
			if fe.ShowCookie != nil {
				entry.ShowCookie = fe.ShowCookie
			}
			if fe.ShowPasskey != nil {
				entry.ShowPasskey = fe.ShowPasskey
			}
			if fe.ShowAPIKey != nil {
				entry.ShowAPIKey = fe.ShowAPIKey
			}
			if fe.ShowRSSKey != nil {
				entry.ShowRSSKey = fe.ShowRSSKey
			}
		}

		// RSS Key 站点特殊处理（家园/铂金家/不可杜）
		// 实际用 RSS Key 下载（download_url_template 含 {rsskey}），不用 Passkey
		// 修复历史 bug：原 UI 把 Passkey 字段伪装成 "RSS Key"（passkey_alias），
		// 但 adapter 实际读 sites.rss_key 列（不同字段）→ UI 与 adapter 脱节
		// 正确做法：隐藏基础凭据 Passkey，显示高级凭据 RSS Key
		if rssKeySites[domain] {
			if entry.PasskeyLabel != "" {
				entry.RSSKeyLabel = entry.PasskeyLabel
				entry.PasskeyLabel = ""
			}
			if entry.PasskeyHint != "" {
				entry.RSSKeyHint = entry.PasskeyHint
				entry.PasskeyHint = ""
			}
			entry.ShowPasskey = boolPtr(false)
			entry.ShowRSSKey = boolPtr(true)
		}

		result[domain] = entry
	}

	// Sort domains for stable output
	domains := make([]string, 0, len(result))
	for d := range result {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	// Output as ordered list (preserves alphabetical)
	out := make(map[string]SupportedSite, len(domains))
	for _, d := range domains {
		out[d] = result[d]
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}

	outputPath := "/home/incast/PT-Forward/internal/site/supported_sites.json"
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("✅ Generated %s (%d sites)\n", outputPath, len(out))

	// Stats
	blocked := 0
	withNotes := 0
	withAPIKeyLabel := 0
	withPasskeyLabel := 0
	withRSSKeyLabel := 0
	withShowCookie := 0
	withShowPasskeyFalse := 0
	withShowRSSKeyTrue := 0
	withFormFields := 0
	withPaths := 0
	for _, s := range out {
		if s.VerificationStatus == "blocked" {
			blocked++
		}
		if s.SpecialNotes != "" {
			withNotes++
		}
		if s.APIKeyLabel != "" {
			withAPIKeyLabel++
		}
		if s.PasskeyLabel != "" {
			withPasskeyLabel++
		}
		if s.RSSKeyLabel != "" {
			withRSSKeyLabel++
		}
		if s.ShowCookie != nil {
			withShowCookie++
		}
		if s.ShowPasskey != nil && !*s.ShowPasskey {
			withShowPasskeyFalse++
		}
		if s.ShowRSSKey != nil && *s.ShowRSSKey {
			withShowRSSKeyTrue++
		}
		if len(s.PublishFormFields) > 0 {
			withFormFields++
		}
		if len(s.Paths) > 0 {
			withPaths++
		}
	}
	fmt.Printf("  blocked: %d\n", blocked)
	fmt.Printf("  with special_notes: %d\n", withNotes)
	fmt.Printf("  with api_key_label: %d\n", withAPIKeyLabel)
	fmt.Printf("  with passkey_label: %d\n", withPasskeyLabel)
	fmt.Printf("  with rss_key_label: %d\n", withRSSKeyLabel)
	fmt.Printf("  with show_cookie override: %d\n", withShowCookie)
	fmt.Printf("  with show_passkey=false: %d\n", withShowPasskeyFalse)
	fmt.Printf("  with show_rsskey=true: %d\n", withShowRSSKeyTrue)
	fmt.Printf("  with publish_form_fields: %d\n", withFormFields)
	fmt.Printf("  with paths override: %d\n", withPaths)
}
