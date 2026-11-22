package util

import (
	"net/url"
	"strings"
)

// ExtractHost §59.551: 从 rawURL 提取 host（小写·含端口如有）。
// rawURL 为空或解析失败返回空串。
// 收敛自 publish/image_check.go 与 metadata/extract/unwanted_images.go 的
// 两份同实现私有副本。注意：返回 u.Host 含端口——与去端口的 domain
// 语义（如 P1 的 extractDomain）不同，勿混用。
func ExtractHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}
