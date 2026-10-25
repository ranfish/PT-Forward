package titleparser

import (
	"encoding/json"
	"strings"
)

// BuildTechProfile 三源分层合并 → TechProfile（§56.34 决策 4 的公共入口）。
//
// 统一供 runAnalyze（手动转发 ⑫）和 fetchSingleTorrent（种子配置页）调用，
// 消除两条管线的 TechProfile 解析断裂（§59.26 管线统一）。
//
// 合并优先级：
//   - 技术参数 8 字段：MediaInfo > 标题
//   - 标题特有 8 字段：标题（唯一源）
//   - 媒介/分类 2 字段：DOM > 标题
//   - §59.254 项1 第四源：PTGen（year 校正/main_title 兜底/mismatch 可观测）
//     ——MergePTGenInto 由调用方按端点感知链准备输入后显式调用（发布链接线；
//     本入口签名不变，避免 26 处调用点波及）
func BuildTechProfile(title, mediaInfo string, domMedium, domResolution, domVideoCodec, domAudioCodec string) TechProfile {
	profile := ParseTitleTech(title)
	if mediaInfo != "" {
		miTech := ExtractMediaInfo(mediaInfo)
		MergeMediaInfoInto(&profile, &miTech)
		// §59.319 附十六: BDInfo 报告输入=原盘铁证（报告仅原盘存在）——
		// QUICK SUMMARY 锚与 ExtractMediaInfo 分流同源；MediumCanonicalOf
		// 最高优先级消费（压制写法启发式不覆盖——Under Current 案）
		// §59.319 附二十三: MINBD 排除——标题 x264/x265 重编码标记（CMCT
		// 等组的"缩小版原盘"，有 BDMV 结构+BDInfo 报告但非原盘，PT 站归
		// Encode）；设 MIEncoded=true 走 IsEncode 编码路径，不设 FromBDInfo
		if strings.Contains(mediaInfo, "QUICK SUMMARY:") {
			if hasReencodeCodecInTitle(title) {
				profile.MIEncoded = true
			} else {
				profile.FromBDInfo = true
			}
		}
	}
	MergeDOMInto(&profile, domMedium, domResolution, domVideoCodec, domAudioCodec)
	// §59.349: ST 词形与媒介铁证冲突仲裁（判定覆盖提取——与 §59.166 B2
	// MI 纠错同模式）。v1.05 原盘/压制以连字符区分：压制铁证∧连字符族→
	// 降格驼峰；原盘铁证∧驼峰族→升格连字符（对称完备）；无铁证不干预。
	arbitrateSTScript(&profile)
	// §59.76: 实现步骤 3 承诺的合并——ReleaseVersion(PROPER/REPACK/RERIP/DIRFIX/INTERNAL)
	// 并入 EditionInfo（空时补，不覆盖既有 edition token；重组/展示统一单通道）
	if profile.EditionInfo == "" && profile.ReleaseVersion != "" {
		profile.EditionInfo = profile.ReleaseVersion
	}
	return profile
}

// stScriptPairs §59.349: ST 词形族映射对（splitMedium 六形态三对——
// 连字符=原盘写法，驼峰=压制写法，v1.05 原盘/压制以连字符区分）。
var stScriptPairs = map[string]string{
	"Blu-ray":     "BluRay",
	"UHD Blu-ray": "UHD BluRay",
	"3D Blu-ray":  "3D BluRay",
}

// arbitrateSTScript §59.349: ST 词形与媒介铁证冲突仲裁。
// 压制铁证（hasReencodeMark——x264/x265 CLI 编码器标记真原盘恒无）
// ∧连字符族 → 降格驼峰（Zhizn I Sudba 案：keepfrds 标题习惯连字符表
// "源自蓝盘"不区分语境，压制种输出原盘专属 token 违规——与媒介判定
// 铁证冲突）；原盘铁证（FromBDInfo——QUICK SUMMARY 锚=BDInfo 报告
// 仅原盘存在）∧驼峰族 → 升格连字符（对称完备——同一规则两方向，
// §59.342 附修一漏一教训；非新场景预实施）。
// 无铁证 → 不干预（证据缺席时输出层忠实提取——防误伤根门）。
func arbitrateSTScript(p *TechProfile) {
	if p == nil || p.SourceType == "" {
		return
	}
	if hasReencodeMark(*p) {
		if camel, ok := stScriptPairs[p.SourceType]; ok {
			p.SourceType = camel
		}
		return
	}
	if p.FromBDInfo {
		for hyphen, camel := range stScriptPairs {
			if p.SourceType == camel {
				p.SourceType = hyphen
				return
			}
		}
	}
}

// DOMFieldsFromDetailSource §59.166 A 层公共提升（原 api 包私有 domFieldsFromDetailSource
// 三处重复——发布链 executor 同源消费需要；detail 提取器存 standard key 经
// ReverseLookup 归一，未映射 key 空=跳过保留 title 值）。
func DOMFieldsFromDetailSource(detailJSON string) (medium, resolution, videoCodec, audioCodec string) {
	if detailJSON == "" {
		return
	}
	var ds struct {
		Medium     string `json:"medium"`
		Resolution string `json:"resolution"`
		VideoCodec string `json:"video_codec"`
		AudioCodec string `json:"audio_codec"`
	}
	if json.Unmarshal([]byte(detailJSON), &ds) != nil {
		return
	}
	return ReverseLookup(ds.Medium),
		ReverseLookup(ds.Resolution),
		ReverseLookup(ds.VideoCodec),
		ReverseLookup(ds.AudioCodec)
}
