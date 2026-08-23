// Package comment 种子 comment 解析（§59.61）。
//
// 模型依据 §59.60 v5：comment = 发布者/转发者声明的"种子所在详情页"，
// 出现详情页 URL 或 tid 即代表对应站有此种子，可直达。
// 五方言（全量 20 万种子实测锚定）：
//
//	标准 URL   https?://域/(details?.php?id=|t/)(\d+)
//	ob_tid     [(,]ob_tid=(\d+)                 → 我堡
//	HDHx       ^HDHx(\d+)x\d+x[0-9a-f]{8}$       → 家园（A=tid B=发布epoch C=校验）
//	馒头纯数字  ^(\d{5,10})$（仅 tracker 域=馒头） → 馒头
//	相对路径   ^/details.php\?id=(\d+)            → tracker 首域站
package comment

import (
	"regexp"
	"strconv"
	"strings"
)

// DirectTarget 直达候选：站点名 + tid。
type DirectTarget struct {
	SiteName   string
	TorrentID  string
}

var (
	urlPattern = regexp.MustCompile(`https?://([^/\s,\]\)]+)/(?:details?\.php\?id=|t/)(\d+)`)
	obTidP     = regexp.MustCompile(`(?:^|[\s,])ob_tid[=:](\d+)`)
	hdhP       = regexp.MustCompile(`^HDHx(\d+)x\d+x[0-9a-f]{8}$`)
	plainNumP  = regexp.MustCompile(`^(\d{5,10})$`)
	relPathP   = regexp.MustCompile(`^/details\.php\?id=(\d+)`)
)

// Resolve 解析 comment → 直达候选列表（按规则序，多命中全返回）。
// hostResolver: 域名 → 站名（复用 site.TrackerMatcher 的索引语义；返回空串表示未解析）。
// trackerHost: 该种子 tracker 首域 host（馒头纯数字/相对路径方言的站点上下文）。
func Resolve(commentText string, hostResolver func(string) string, trackerHost string) []DirectTarget {
	commentText = strings.TrimSpace(commentText)
	if commentText == "" {
		return nil
	}
	var out []DirectTarget
	seen := map[string]bool{} // site:tid 去重

	add := func(site, tid string) {
		if site == "" || tid == "" {
			return
		}
		key := site + ":" + tid
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, DirectTarget{SiteName: site, TorrentID: tid})
	}

	// 1. 标准 URL（可多条——URL + ,ob_tid= 复合形态两种都取）
	for _, m := range urlPattern.FindAllStringSubmatch(commentText, 4) {
		add(hostResolver(m[1]), m[2])
	}
	// 2. ob_tid → 我堡（固定方言；与 URL 复合时两者并立）
	if m := obTidP.FindStringSubmatch(commentText); m != nil {
		add("我堡", m[1])
	}
	// 3. HDHx → 家园
	if m := hdhP.FindStringSubmatch(commentText); m != nil {
		add("家园", m[1])
	}
	// 4. 馒头纯数字（tracker 上下文限定，避免误吞其他站的纯数字签名）
	if m := plainNumP.FindStringSubmatch(commentText); m != nil && trackerHost != "" {
		if site := hostResolver(trackerHost); site == "馒头" {
			add("馒头", m[1])
		}
	}
	// 5. 相对路径 → tracker 首域站
	if m := relPathP.FindStringSubmatch(commentText); m != nil && trackerHost != "" {
		add(hostResolver(trackerHost), m[1])
	}
	return out
}

// ValidTid 语义校验辅助：tid 数字化（站点 API 均为数字 tid）。
func ValidTid(tid string) bool {
	_, err := strconv.Atoi(tid)
	return err == nil
}
