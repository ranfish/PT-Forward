package sites

import (
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// keepfrdsExtractor keepfrds 站特殊提取器。
// PTNexus sites/keepfrds.go（229 行）。
//
// 站名对照（生产 DB 验证 + 用户确认）：
//   pt.keepfrds.com → PT-Forward 站名"朋友" → siteCode "keepfrds"
//   duckboobee.org  → PT-Forward 站名"三月"（独立站点，非 keepfrds）
type keepfrdsExtractor struct{}

func newKeepfrdsExtractor() *keepfrdsExtractor { return &keepfrdsExtractor{} }

func (e *keepfrdsExtractor) Name() string { return "keepfrds_special" }

// keepfrds 的 <title> 后缀模式
var keepfrdsTitleSuffixRe = regexp.MustCompile(`\s*::\s*种子详情.*`)

func (e *keepfrdsExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}

	// keepfrds 特殊 1: 标题提取
	// h1 是"用户评论"（非种子标题），种子标题在 <title> 标签中
	// <title> 格式: "中文标题 :: 种子详情 :: PT@KEEPFRDS"
	if title := extractKeepfrdsTitle(input.PageHTML); title != "" {
		seed.Title = title
	}

	// keepfrds 特殊 2: 主副标题对调
	// keepfrds 的"副标题"字段实际存英文标题（如 Brad's Status 2017 BluRay...）
	// "标题"字段存中文名
	// 公共提取器可能把中文名提取到 Title，英文标题提取到 Subtitle
	// 正确语义: Title=英文标题（转载目标站用），Subtitle=中文名
	// 但设计文档说"对调"，意味着 keepfrds 原始字段语义与常规相反
	// 实际操作: Title 取 <title> 提取的中文名，Subtitle 取副标题字段的英文标题
	// 上面已设 Title，这里保持 Subtitle（公共提取器提取的英文标题）

	return seed, nil
}

// extractKeepfrdsTitle 从 <title> 标签提取种子标题（去掉后缀）。
func extractKeepfrdsTitle(htmlStr string) string {
	re := regexp.MustCompile(`<title>([^<]+)</title>`)
	m := re.FindStringSubmatch(htmlStr)
	if len(m) < 2 {
		return ""
	}
	title := keepfrdsTitleSuffixRe.ReplaceAllString(m[1], "")
	return strings.TrimSpace(title)
}
