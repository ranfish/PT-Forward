package publish

import (
	"context"
	"encoding/json"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
)

// §59.593（源站/来源站语义重定义·用户九轮定案·v0.0.1677）: 禁转权利族
// gate——唯一公共判定（发布门 checkFlagsFromMetadata 与列表链
// classifySeedStatusLite 同口径消费·防口径分裂复发）。
//
// 语义模型（七轮+三回归收敛）：
//
//	权利归属：仅初始发布者（组的官组站）有权禁转（PT 圈共识）。
//	判定辖域（用户 R5/R8 两裁定各管一域）：
//	  - 标签/标题域签（署名确定=打签站）→ 身份判定：打签站=组官组站？
//	    官组站签 → 有效拦；越权签（转载站）→ 不拦（R5·权利模型）
//	  - 描述域命中（署名不可判：可能是 B 主张也可能是 A 转述）→
//	    保守拦（R8·跟随现状——现状全拦的辖域保留）
//	退化判定（组名空）：官组站行 → 拦（个人首发保守）；非官组站行 →
//	  不拦（R6 第 2 点：转载站无官组物理不可能成为源站）。
//	外站组（映射无命中）→ 保守拦（权利归属在外站不可判）。
//
// gate 只管权利族 Flags；adult（合规体系）、限时禁转/限转（时间态/
// 站规族——§59.162/§59.369 附二现有通道）不进 gate——由调用方现有
// 逻辑继续判定（辖域正交）。
//
// 域重演：meta.Flags 是四域（标题+副标题+标签+描述）聚合的扁平数组
// （无域来源信息）——gate 用 extract.ExtractFlagsFromText 对
// Description/Title+Subtitle+Tags 分别重演检测，排除法判定签的来源域：
//
//	描述域命中 → 保守拦（一票拦——保守原则覆盖身份判定）
//	描述无∧标题标签有 → 身份判定
//	四域复查全无 → 数据不一致 → 保守拦（防御）
//
// nil detector → 保守拦（无映射判定能力=不能证伪权威性——测试构造
// 与装配前期的降级语义，main 装配后恒非 nil）。
var noTransferRightFlags = map[string]bool{
	"禁转": true, "禁止转载": true, "谢绝转载": true,
	"严禁转载": true, "谢绝搬运": true, "独占": true,
}

// parseMetaFlags 解析 meta.Flags（JSON 数组，解析失败返回空）。
func parseMetaFlags(raw string) []string {
	if raw == "" {
		return nil
	}
	var flags []string
	if err := json.Unmarshal([]byte(raw), &flags); err != nil {
		return nil
	}
	return flags
}

// hasRightFlag 权利族命中。
func hasRightFlag(flags []string) bool {
	for _, f := range flags {
		if noTransferRightFlags[f] {
			return true
		}
	}
	return false
}

// NoTransferGateBlock 权利族禁转 gate（公共单点）。返回 (拦, 原因)。
// meta 为 nil 或无权利族签 → (false, "")（不拦——时间态/合规由调用方管）。
func NoTransferGateBlock(detector *SourceSiteDetector, meta *model.TorrentMetadata) (bool, string) {
	if meta == nil || !hasRightFlag(parseMetaFlags(meta.Flags)) {
		return false, ""
	}

	// 域重演（排除法判定签来源域）
	descHas := hasRightFlag(extract.ExtractFlagsFromText(meta.Description))
	titleTagsHas := hasRightFlag(extract.ExtractFlagsFromText(
		meta.Title + " " + meta.Subtitle + " " + meta.Tags))

	if descHas {
		return true, "禁转信号在描述域（署名不可判·保守拦——跟随现状）"
	}
	if !titleTagsHas {
		return true, "Flags 含禁转权利族签但四域复查全无（数据不一致·保守拦）"
	}

	// 署名确定域（标签/标题）→ 身份判定
	if detector == nil {
		return true, "源站映射 detector 未装配（保守拦）"
	}
	group := ExtractGroupName(meta.Title)
	if group == "" {
		// 组名空退化（发现 2 定案）：官组站拦（个人首发保守）/
		// 非官组站不拦（无官组物理不可能源站）
		if detector.IsOfficialSite(meta.SiteName) {
			return true, "禁转签+组名空+官组站（个人首发保守拦）"
		}
		return false, "禁转签+组名空+非官组站（无官组≠源站·越权不拦）"
	}
	official := detector.LookupGroup(context.Background(), group)
	switch {
	case official == "":
		return true, "外站组 " + group + "（权利归属外站不可判·保守拦）"
	case official == meta.SiteName:
		return true, "源站禁转签（" + group + "→" + meta.SiteName + "·有效拦截）"
	default:
		return false, "越权禁转签（" + group + "官组站=" + official +
			"≠打签站" + meta.SiteName + "·不拦）"
	}
}
