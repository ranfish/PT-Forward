// Package extract keepfrds 截图 URL 展开（§59.48）。
//
// 朋友站截图三层代理结构（243 实测解剖）：
//
//	img.keepfrds.com/<sig>/resize:fill:240:0:0/<b64>   站内缩略图（403 防盗链）
//	  b64 解开 → imgfetch.keepfrds.com/fetch?url=<原图>&s=<sig>   站方代理（200）
//	    url 参数 → origin.picgo.net/...png   发布者图床原图（200）
//
// 提取时展开到第一层（picgo 直链）——出库数据为标准形态，读取端零特殊逻辑。
package extract

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"strings"
)

// reKFThumb keepfrds 站内缩略图 URL 形态（精确匹配，其他站/直链不碰）：
// https://img.keepfrds.com/<签名>/resize:<指令>/<base64段>
var reKFThumb = regexp.MustCompile(`^https://img\.keepfrds\.com/[^/]+/resize:[^/]+/([A-Za-z0-9_-]+)$`)

// ExpandKFImageUrl 展开 keepfrds 站内缩略图 URL → 原图直链。
//
// 展开链：缩略图 b64 段 → imgfetch 代理 URL → url 参数（原图直链）。
// 非匹配形态原样返回；解码失败/参数缺失时降级返回 imgfetch 完整 URL；
// 再失败保留原 URL（宁可 403 不可丢数据）。
func ExpandKFImageUrl(raw string) string {
	raw = strings.TrimSpace(raw)
	m := reKFThumb.FindStringSubmatch(raw)
	if m == nil {
		return raw // 非 keepfrds 缩略图形态，不碰
	}

	// base64（URL-safe 变体）解码 → imgfetch 代理 URL
	b64 := m[1]
	if i := strings.IndexByte(b64, '.'); i >= 0 { // 带扩展名尾巴（imgproxy 明文模式）不属此形态
		return raw
	}
	decoded, err := base64.RawURLEncoding.DecodeString(padB64(b64))
	if err != nil {
		return raw
	}
	proxyURL := strings.TrimSpace(string(decoded))
	if !strings.HasPrefix(proxyURL, "http") {
		return raw // 解出非 URL（可能是签名密文形态），保留原值
	}

	// imgfetch 形态：取 url 参数 → 原图直链
	u, err := url.Parse(proxyURL)
	if err != nil {
		return proxyURL // 解析失败用代理 URL（200 可达）
	}
	if orig := u.Query().Get("url"); orig != "" && strings.HasPrefix(orig, "http") {
		return orig
	}
	return proxyURL // 无 url 参数（可能已是直链形态），用解出值
}

// padB64 base64 段补齐（RawURLEncoding 自带无 padding 支持，此处防御异常长度）。
func padB64(s string) string {
	if m := len(s) % 4; m == 1 {
		return s[:len(s)-1] // 非法尾字符，截断
	}
	return s
}

// ExpandKFImageUrls 批量展开（screenshots 列表逐条）。
func ExpandKFImageUrls(urls []string) []string {
	if urls == nil {
		return nil
	}
	out := make([]string, len(urls))
	for i, u := range urls {
		out[i] = ExpandKFImageUrl(u)
	}
	return out
}
