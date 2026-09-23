package adapter

import (
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.266: 凭证拦截页三防线公共化——GenericAdapter（§59.263/264 首修）与
// NexusPHPAdapter（243 异地登录提醒 15:52 复发案：修复落错适配器——不可杜
// 走 NexusPHP 框架，守卫只在 Generic 不生效）共享同一实现。

// CheckCredentialRedirect 凭证拦截重定向检测——302 自动跟随后终点为
// take2fa/login（NexusPHP 二验/登录重定向，hddolby take2fa.php 实证），
// 返回 200 但内容为验证页。不识别则空解析"静默成功"，选站降级链
// 永不触发。
func CheckCredentialRedirect(resp *http.Response) error {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return nil
	}
	finalPath := strings.ToLower(resp.Request.URL.Path)
	if strings.Contains(finalPath, "take2fa") || strings.Contains(finalPath, "login") {
		return authError(fmtES("站点要求二次验证或登录（凭证被拦截，请更新 cookie）: %s", resp.Request.URL.Path), nil)
	}
	return nil
}

// CheckDetailVacuum 解析真空双守卫。
//   - 空解析守卫（§59.263）：title/desc/infohash 三全空 = 页面不是详情页
//     （未知形态拦截页/结构变化）；
//   - 结构真空守卫（§59.264）：title 有值但结构字段（desc/infohash/category/
//     subtitle）全空——凭证拦截页族变体（hddolby 异地登录提醒页：200 直出
//     无重定向、<title> 有值）绕过 URL 检测；真详情页必有结构字段至少其一。
//     泛化判据不追站点措辞，未来新变体同层覆盖。
func CheckDetailVacuum(detail *model.TorrentDetail) error {
	if detail == nil {
		return nil
	}
	if detail.Title == "" && detail.Description == "" && detail.InfoHash == "" {
		return parseError("详情页解析为空（拦截页或页面结构变化）", nil)
	}
	if detail.Title != "" && detail.Description == "" && detail.InfoHash == "" &&
		detail.Category == "" && detail.Subtitle == "" {
		return authError(fmtES("页面无详情结构（疑似凭证拦截/提醒页）: %s", detail.Title), nil)
	}
	return nil
}
