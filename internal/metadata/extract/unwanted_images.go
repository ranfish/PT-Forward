// Package extract 图片黑名单/白名单（§56.12）。
//
// embed JSON + 内存缓存（sync.Once 懒加载）。
// TODO: 后续可扩展为 DB 驱动（unwanted_images 表 + 用户自定义），与 §56.1 standard_keys 一致。
package extract

import (
	_ "embed"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
)

//go:embed data/unwanted_images.json
var unwantedImagesJSON []byte

//go:embed data/image_whitelist.json
var imageWhitelistJSON []byte

// unwantedImageRule 黑名单规则。
type unwantedImageRule struct {
	Pattern   string `json:"pattern"`
	MatchType string `json:"match_type"` // substring / exact / host / regex
	Category  string `json:"category"`   // site_logo / team_logo / ad / emoji / watermark / other
}

// imageWhitelistRule 白名单规则。
type imageWhitelistRule struct {
	Host     string `json:"host"`
	Category string `json:"category"` // image_host / metadata_host
}

var (
	unwantedImageRules  []unwantedImageRule
	imageWhitelistRules []imageWhitelistRule
	rulesLoaded         sync.Once
)

// loadRules 懒加载黑名单/白名单到内存。
func loadRules() {
	rulesLoaded.Do(func() {
		var data struct {
			UnwantedImages []unwantedImageRule `json:"unwanted_images"`
		}
		if err := json.Unmarshal(unwantedImagesJSON, &data); err == nil {
			unwantedImageRules = data.UnwantedImages
		}

		var wlData struct {
			ImageWhitelist []imageWhitelistRule `json:"image_whitelist"`
		}
		if err := json.Unmarshal(imageWhitelistJSON, &wlData); err == nil {
			imageWhitelistRules = wlData.ImageWhitelist
		}
	})
}

// IsUnwantedImage 判断 URL 是否命中黑名单。
// 支持 4 种 match_type: substring / exact / host / regex。
func IsUnwantedImage(imageURL string) bool {
	loadRules()
	lower := strings.ToLower(imageURL)
	host := extractHost(imageURL)

	for _, rule := range unwantedImageRules {
		switch rule.MatchType {
		case "substring":
			if strings.Contains(lower, strings.ToLower(rule.Pattern)) {
				return true
			}
		case "exact":
			if lower == strings.ToLower(rule.Pattern) {
				return true
			}
		case "host":
			if host != "" && (host == rule.Pattern || strings.HasSuffix(host, "."+rule.Pattern)) {
				return true
			}
		case "regex":
			// TODO: regex 支持（后续按需添加，当前规则未使用）
		}
	}
	return false
}

// IsWhitelistedImage 判断 URL 是否在白名单（图床域名）。
// 白名单优先于黑名单：白名单命中的 URL 即使含黑名单关键字也保留。
func IsWhitelistedImage(imageURL string) bool {
	loadRules()
	host := extractHost(imageURL)
	if host == "" {
		return false
	}
	for _, rule := range imageWhitelistRules {
		if host == rule.Host || strings.HasSuffix(host, "."+rule.Host) {
			return true
		}
	}
	return false
}

// extractHost 从 URL 中提取 host（小写）。
func extractHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}
