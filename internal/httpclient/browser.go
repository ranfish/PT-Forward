package httpclient

import "net/http"

// 浏览器指纹单点（§59.320）：UA 与 Sec-Ch-Ua 版本号必须严格一致，
// 否则伪造特征比裸 UA 更明显。全项目站点请求统一走此单点，
// 禁止散落私设 User-Agent（rss/fetcher 曾用 "PT-Forward/1.0" 裸 bot
// 签名——keepfrds Cloudflare 持续 empty_response 主因）。
//
// 头序列对齐 ptool impersonateutil chrome profile；Accept-Encoding
// 不设置（Go 自动加 gzip 并透明解压，手动设 br 会导致 body 无法解压）。
const (
	browserChromeMajor = "131"
	browserChromeFull  = "131.0.0.0"
)

// BrowserUserAgent 统一浏览器 UA（Chrome on Windows x64）。
func BrowserUserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + browserChromeFull + " Safari/537.36"
}

// SetBrowserUserAgent 轻量版：仅统一 UA + Cookie（JSON API 场景，
// Sec-Fetch-Dest=document 是导航头，对 API 语义不符故不设）。
func SetBrowserUserAgent(h http.Header, cookie string) {
	h.Set("User-Agent", BrowserUserAgent())
	if cookie != "" {
		h.Set("Cookie", cookie)
	}
}

// SetBrowserHeaders 设置完整浏览器指纹头序列（UA + Sec-Ch-Ua 系 +
// Sec-Fetch 系 + Accept 系）。cookie 为空时不设 Cookie 头。
// 保留调用方已设置的 Referer/Origin 等上下文头。
func SetBrowserHeaders(h http.Header, cookie string) {
	h.Set("User-Agent", BrowserUserAgent())
	h.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	h.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	h.Set("Sec-Ch-Ua", `"Google Chrome";v="`+browserChromeMajor+`", "Chromium";v="`+browserChromeMajor+`", "Not_A Brand";v="24"`)
	h.Set("Sec-Ch-Ua-Mobile", "?0")
	h.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	h.Set("Sec-Fetch-Dest", "document")
	h.Set("Sec-Fetch-Mode", "navigate")
	h.Set("Sec-Fetch-Site", "none")
	h.Set("Sec-Fetch-User", "?1")
	h.Set("Upgrade-Insecure-Requests", "1")
	if cookie != "" {
		h.Set("Cookie", cookie)
	}
}
