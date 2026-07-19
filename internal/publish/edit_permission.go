// Package publish 编辑权限校验（§56.23 决策 3）。
//
// 4 步校验（借鉴 PTNexus）:
//  1. 检测编辑按钮存在（edit.php?id=X）
//  2. 解析发布者用户名
//  3. 解析当前登录用户
//  4. 比对发布者 == 当前用户
//
// 当前版本: NexusPHP 基本实现（正则匹配）。
// TODO(7e P1): 需真实 HTML 样本验证正则 + MTeam/UNIT3D/Gazelle API 模式。
package publish

import (
	"regexp"
	"strings"
)

var (
	// NexusPHP 编辑按钮: <a href="edit.php?id=123">
	npEditButtonRe = regexp.MustCompile(`(?i)<a[^>]*href=["']edit\.php\?id=(\d+)["'][^>]*>`)

	// 发布者用户名: <a href="userdetails.php?id=123">用户名</a>
	npPublisherNameRe = regexp.MustCompile(`(?i)<a[^>]*href=["']userdetails\.php\?id=\d+["'][^>]*>([^<]+)</a>`)

	// 当前登录用户（页面顶部 userinfo / userbar 区域）
	npCurrentUserRe = regexp.MustCompile(`(?is)(?:id=["']userinfo["']|class=["'][^"']*userbar[^"']*["'])[^<]*<a[^>]*>([^<]+)</a>`)

	// 朱雀编辑按钮
	zhuqueEditButtonRe = regexp.MustCompile(`(?i)<a[^>]*href=["'][^"']*torrent/edit/\d+["'][^>]*>`)
)

// CheckEditPermission 检查当前用户是否有编辑某种子的权限。
// 返回: (是否允许, 原因说明)
// detailHTML: 种子详情页 HTML
// torrentID: 种子 ID
// siteFramework: 站点框架（nexusphp/zhuque/mteam/unit3d/gazelle）
func CheckEditPermission(detailHTML, torrentID, siteFramework string) (bool, string) {
	switch strings.ToLower(siteFramework) {
	case "zhuque":
		return checkZhuqueEditPermission(detailHTML, torrentID)
	case "mteam", "unit3d", "gazelle":
		// API 模式：权限校验在 API 层处理（返回 token 错误）
		return true, "API 模式（权限由 API 返回值决定）"
	default:
		return checkNexusPHPEditPermission(detailHTML, torrentID)
	}
}

// checkNexusPHPEditPermission NexusPHP 4 步权限校验。
func checkNexusPHPEditPermission(detailHTML, torrentID string) (bool, string) {
	if detailHTML == "" {
		return false, "详情页 HTML 为空"
	}

	// 1. 检测编辑按钮
	if !npEditButtonRe.MatchString(detailHTML) {
		return false, "未找到编辑按钮（无编辑权限或非自己发布的种子）"
	}

	// 2. 解析发布者用户名
	publisher := extractFirstMatch(detailHTML, npPublisherNameRe)
	if publisher == "" {
		// 发布者解析失败，但有编辑按钮 → 放宽允许
		return true, "编辑按钮存在（发布者解析失败，放宽允许）"
	}

	// 3. 解析当前登录用户
	currentUser := extractFirstMatch(detailHTML, npCurrentUserRe)
	if currentUser == "" {
		// 某些站点 userinfo 结构不同，放宽判断（仅检查编辑按钮存在）
		return true, "编辑按钮存在（当前用户解析失败，放宽允许）"
	}

	// 4. 比对
	if strings.EqualFold(strings.TrimSpace(publisher), strings.TrimSpace(currentUser)) {
		return true, "发布者 == 当前用户"
	}
	return false, "非自己发布的种子（发布者: " + publisher + "，当前: " + currentUser + "）"
}

// checkZhuqueEditPermission 朱雀框架权限校验。
func checkZhuqueEditPermission(detailHTML, torrentID string) (bool, string) {
	if detailHTML == "" {
		return false, "详情页 HTML 为空"
	}
	if !zhuqueEditButtonRe.MatchString(detailHTML) {
		return false, "未找到编辑按钮"
	}
	return true, "编辑按钮存在"
}

// extractFirstMatch 提取第一个匹配的捕获组。
func extractFirstMatch(s string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
