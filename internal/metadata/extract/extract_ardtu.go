package extract

// §56.10 extractARDTU：工具签名剥离（28 项黑名单 + 14 条技术参数 AND 匹配）。
// 2b.4 阶段 stub（返回空切片），后续完善：
//   - 移植 PTNexus review_extract.go:624-715 的 28 项黑名单（CSAUTO/ARDTU/郑重声明等）
//   - 14 条技术参数 AND 匹配
//   - By ARDTU 特殊处理（剥离前缀保留正文）
// 默认实现不剥离任何内容（保持原样），保证基础流程可用。
func (p *PublicExtractor) extractARDTUDeclarations(text string) (cleaned string, removed []string) {
	return text, nil
}
