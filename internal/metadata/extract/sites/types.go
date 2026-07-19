// Package sites 站点特殊提取器（§56.13）。
//
// 8 个 PTNexus 特殊站的 Extractor 实现。每个站点的 Extractor：
//  1. 先调公共提取器（PublicExtractor）获取基础 SeedData
//  2. 再做站点特殊覆盖（MediaInfo 容器/标题/海报/表格行 URL 等）
//
// 架构（§56.13 决策 2）：
//   - sites 子包 import extract 包的公开 API（无循环依赖）
//   - Engine 通过 NewEngine(public, specials) 注册特殊提取器
//   - 每个 Extractor 实现 extract.Extractor 接口
//
// 当前阶段（2b.7 骨架）：8 个 stub，特殊逻辑标注 TODO 等真实样本后逐站填充。
package sites

import (
	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// baseExtract 公共提取器调用（所有站点共用）。
// 返回公共提取器提取的 SeedData + error。
func baseExtract(input extract.Input) (extract.SeedData, error) {
	pub := extract.NewPublicExtractor(input.SiteCode, input.SiteNickname)
	return pub.Extract(input)
}
