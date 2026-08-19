// Package publish 海报可信图源白名单 + 两级 PTGen fallback（§59.42）。
//
// 站点采集海报（如朋友站 poster_douban/w185 代理豆瓣形态）必然失活——
// 采集落库时异步替换为可信图源（doubaninfo 自托管缓存 / pixhost 直链）。
package publish

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// trustedPosterSuffixes 可信海报图源域名后缀（§59.42 白名单）。
// doubaninfo：PTGen endpoint ① 自托管海报缓存（长期活）
// pixhost.to / pixhost.cc：cspt endpoint ② 的图床直链家族（img3.pixhost.cc 等）
var trustedPosterSuffixes = []string{
	"doubaninfo.com",
	"pixhost.to",
	"pixhost.cc",
}

// IsTrustedPoster 判定海报 URL 是否可信图源（域名后缀匹配）。
// 空 URL 返回 false（无可信性可言，走 fallback 链）。
func IsTrustedPoster(posterURL string) bool {
	if posterURL == "" {
		return false
	}
	u, err := url.Parse(posterURL)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Host)
	// 去端口
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	for _, suffix := range trustedPosterSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// posterAliveFn HEAD 探测器（§59.42 决策 2）。包级变量供测试注入。
// 4xx/5xx/超时均判死；3xx 视作活（重定向可达）。
var posterAliveFn = defaultPosterAlive

func defaultPosterAlive(ctx context.Context, posterURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, posterURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}

// PosterFallbackResult 海报替换链产物（溯源三态）。
type PosterFallbackResult struct {
	Poster        string // 最终值（可信原图 / PTGen 海报 / 保留原值）
	Source        string // site / ptgen_doubaninfo / ptgen_cspt / ptgen_dead
	Original      string // 站点采集原值（回溯线索）
}

// PTGenQuerier PTGen 查询抽象（Provider 多 endpoint 轮询整体作为一个 querier；
// 两级 fallback 由两个 querier 表达，调用方可注入不同 endpoint 的查询器）。
type PTGenQuerier func(ctx context.Context, query string) (poster string, err error)

// RunPosterFallback 海报替换链（§59.42）。
//
// 输入站点采集 poster：
//  1. 可信图源 → 直接保留（source=site）
//  2. 不可信 → querier①（doubaninfo）→ poster₁ 过白名单 + HEAD 探活 → 活用之
//  3. poster₁ 死 → querier②（cspt）→ poster₂ 同判定
//  4. 全死 → 保留原值（source=ptgen_dead，发布防线兜底）
//
// query 优先 douban URL（精确），无则标题。
func RunPosterFallback(ctx context.Context, sitePoster, query string, queriers []PTGenQuerier) PosterFallbackResult {
	res := PosterFallbackResult{Poster: sitePoster, Source: "site", Original: sitePoster}
	if IsTrustedPoster(sitePoster) {
		return res
	}
	for i, q := range queriers {
		if q == nil || query == "" {
			continue
		}
		qctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		poster, err := q(qctx, query)
		cancel()
		if err != nil || !IsTrustedPoster(poster) || !posterAliveFn(ctx, poster) {
			continue
		}
		res.Poster = poster
		switch i {
		case 0:
			res.Source = "ptgen_doubaninfo"
		case 1:
			res.Source = "ptgen_cspt"
		default:
			res.Source = "ptgen"
		}
		return res
	}
	if sitePoster != "" {
		res.Source = "ptgen_dead" // fallback 全失败，保留原值待发布防线
	}
	return res
}
