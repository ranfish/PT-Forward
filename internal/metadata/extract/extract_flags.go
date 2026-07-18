package extract

import (
	"strings"
)

// 默认禁转关键词（2b.4 硬编码 + TODO 接入 §56.1 standard_keys.is_protected）。
// 后续 6a/§56.21 实施后改为 DB 驱动。
var defaultForbiddenTransferKeywords = []string{
	"禁转", "禁止转载", "谢绝转载", "严禁转载", "谢绝搬运",
	"独占", "限时禁转", "限转", "分集",
}

// extractFlags 从文本中检测禁转/限转标记。
// 扫描范围：标题 + 副标题 + 简介 BBCode + 整页文本。
func (p *PublicExtractor) extractFlags(title, subtitle, descrBBCode, pageText string) []string {
	combined := title + " " + subtitle + " " + descrBBCode + " " + pageText
	var flags []string
	seen := make(map[string]struct{})
	for _, kw := range defaultForbiddenTransferKeywords {
		if strings.Contains(combined, kw) {
			if _, ok := seen[kw]; ok {
				continue
			}
			seen[kw] = struct{}{}
			flags = append(flags, kw)
		}
	}
	if flags == nil {
		flags = []string{}
	}
	return flags
}
