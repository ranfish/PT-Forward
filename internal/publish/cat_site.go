package publish

// §59.560 猫站（pterclub）适配修正：team 域产区化 + 标签 checkbox 投递。
// 站方形态（pterclub.md + 发布页源码实证）：
//   - team_sel 字段名为制作组·实际是地区选择器（大陆/香港/台湾/...8 项）
//   - 标签是 10 个独立 checkbox（guoyu/zhongzi/yueyu/...）·非 tags[] 数组
//   - 无 uplver 字段（文档过时）
// 判据（pterclub.md §7/§8）：
//   - 地区：PTGen region 匹配（中国大陆→1 / 香港→2 / ...）
//   - 国语：MI Audio Chinese/Mandarin ∨ 副标含「国语」
//   - 中字：MI Text Chinese ∨ 副标含「字幕」

import (
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// catRegionToTeamSel §59.560: PTGen region 词→猫站 team_sel 值映射。
// 站方值集（发布页源码实证）：1=大陆 2=香港 3=台湾 4=欧美 5=韩国 6=日本 7=印度 8=其它。
// PTGen region 是数组（["中国大陆"]）——取首个匹配。
func catRegionToTeamSel(region string) string {
	r := strings.ToLower(region)
	switch {
	case strings.Contains(r, "大陆") || strings.Contains(r, "china") || strings.Contains(r, "中国") && !strings.Contains(r, "香港") && !strings.Contains(r, "台湾"):
		return "1"
	case strings.Contains(r, "香港") || strings.Contains(r, "hong kong") || strings.Contains(r, "hkg"):
		return "2"
	case strings.Contains(r, "台湾") || strings.Contains(r, "taiwan") || strings.Contains(r, "twn"):
		return "3"
	case strings.Contains(r, "韩国") || strings.Contains(r, "korea") || strings.Contains(r, "kor"):
		return "5"
	case strings.Contains(r, "日本") || strings.Contains(r, "japan") || strings.Contains(r, "jpn"):
		return "6"
	case strings.Contains(r, "印度") || strings.Contains(r, "india") || strings.Contains(r, "ind"):
		return "7"
	case strings.Contains(r, "欧美") || strings.Contains(r, "western") ||
		strings.Contains(r, "美国") || strings.Contains(r, "us") ||
		strings.Contains(r, "英国") || strings.Contains(r, "uk") ||
		strings.Contains(r, "法国") || strings.Contains(r, "germany") ||
		strings.Contains(r, "德国") || strings.Contains(r, "france"):
		return "4"
	default:
		return "8" // 其它
	}
}

// catIsTeamRegionField §59.560: 猫站形态判定——team 域值集是地区选项
// （非制作组）。判据：值集含「大陆」标签（唯一猫站特征）。
func catIsTeamRegionField(cfg *model.PublishFormConfig) bool {
	if cfg == nil {
		return false
	}
	for _, m := range cfg.ValueMappings[model.FieldDomainTeam] {
		if strings.Contains(strings.ToLower(m.Label), "大陆") || strings.Contains(strings.ToLower(m.Label), "mainland") {
			return true
		}
	}
	return false
}

// catTagCheckboxSite §59.560: 猫站标签 checkbox 形态判定——form_fields
// 无 tags 域（AssembleTagArrayForSite 产出恒空）且站名是猫。
// 站名判据（非动态检测）：标签 checkbox 形态无值集特征可探。
func catTagCheckboxSite(cfg *model.PublishFormConfig) bool {
	return cfg != nil && cfg.FormFields[model.FieldDomainTags] == "" && catIsTeamRegionField(cfg)
}

// catTeamRegionMapping §59.560: 猫站 team 域产区化——PTGen region → team_sel 值。
// 优先 PTGen region（权威产区）·miss 回落副标地区词探·再 miss 走值集 Other。
func catTeamRegionMapping(cfg *model.PublishFormConfig, meta *model.TorrentMetadata) *model.FormValueMapping {
	// ① PTGen region
	regions := ptgenRegionGenreWords(meta, model.FieldDomainRegion)
	if len(regions) > 0 {
		target := catRegionToTeamSel(regions[0])
		for i := range cfg.ValueMappings[model.FieldDomainTeam] {
			if cfg.ValueMappings[model.FieldDomainTeam][i].Value == target {
				return &cfg.ValueMappings[model.FieldDomainTeam][i]
			}
		}
	}
	// ② 副标地区词探（「国产/大陆/港/台/日/韩/美/英」等）
	if meta != nil {
		sub := strings.ToLower(meta.Subtitle)
		for _, kw := range []string{"大陆", "国产", "中国"} {
			if strings.Contains(sub, kw) {
				if m := catTeamValueByTarget(cfg, "1"); m != nil {
					return m
				}
			}
		}
		for _, kw := range []string{"香港", "港片", "港台"} {
			if strings.Contains(sub, kw) {
				if m := catTeamValueByTarget(cfg, "2"); m != nil {
					return m
				}
			}
		}
	}
	// ③ 兜底 Other
	return catTeamValueByTarget(cfg, "8")
}

func catTeamValueByTarget(cfg *model.PublishFormConfig, target string) *model.FormValueMapping {
	for i := range cfg.ValueMappings[model.FieldDomainTeam] {
		if cfg.ValueMappings[model.FieldDomainTeam][i].Value == target {
			return &cfg.ValueMappings[model.FieldDomainTeam][i]
		}
	}
	return nil
}

// catTagCheckboxFields §59.560: 猫站标签 checkbox 投递——独立 name=value="yes"
// 形态（非 tags[] 数组）。判据源=assembleTags 的 canonical 键 + 副标词探。
// 返回 TagKV 列表（adapter 的 TagArrayFields 通道消费）。
func catTagCheckboxFields(meta *model.TorrentMetadata, applied []string) []model.TagKV {
	if meta == nil {
		return nil
	}
	var out []model.TagKV
	has := func(tag string) bool {
		for _, t := range applied {
			if t == tag {
				return true
			}
		}
		return false
	}
	sub := strings.ToLower(meta.Subtitle)

	// 国语：canonical chinese_audio ∨ 副标含「国语」
	if has("chinese_audio") || strings.Contains(sub, "国语") || strings.Contains(sub, "普通话") {
		out = append(out, model.TagKV{Key: "guoyu", Value: "yes"})
	}
	// 粤语：canonical cantonese_audio ∨ 副标含「粤语」
	if has("cantonese_audio") || strings.Contains(sub, "粤语") || strings.Contains(sub, "广东话") {
		out = append(out, model.TagKV{Key: "yueyu", Value: "yes"})
	}
	// 中字：canonical chinese_subtitle ∨ 副标含「中文字幕/简繁/简体/繁体」
	if has("chinese_subtitle") || strings.Contains(sub, "字幕") ||
		strings.Contains(sub, "简繁") || strings.Contains(sub, "简体") || strings.Contains(sub, "繁体") {
		out = append(out, model.TagKV{Key: "zhongzi", Value: "yes"})
	}
	// 英字 §59.561 重构：canonical english_subtitle 优先（dict 推断层单源）
	// + 紧词形兜底（英文字幕/英字幕/简繁英——英与字幕直接相邻的形态）。
	// ⚠ 撤「双语」「中英」宽词探：双语是语言无关词（中德双语≠英字——
	// 圣克莱尔案：「英语 简繁德双语字幕」误投 ensub·站方审核脚本按
	// 「MI 无英字轨且无外挂英字但标签有英字」规则质询）；中英有合拍片
	// 伪面（中英合作…字幕）。canonical 已覆盖中英字幕词形（{0,2} 窗）。
	if has("english_subtitle") || strings.Contains(sub, "英文字幕") ||
		strings.Contains(sub, "英字幕") || strings.Contains(sub, "简繁英") {
		out = append(out, model.TagKV{Key: "ensub", Value: "yes"})
	}
	// DIY原盘 §59.560 追记：资源属性标签（碟是自制的就是自制——转载者有权
	// 也有义务勾·与 jinzhuan 权利声明本质不同——系统级铁律 2026-11-23：
	// 禁转=站点/原发布者专属权力声明·转载者在任何目标站无权勾选·见 AGENTS.md
	// 行为铁律+docs/31 §59.560 追记二；diy=客观资源属性）。
	// 双门防误报：门1 关键词（标题/副标含 DIY/自制）·门2 碟形态源
	// （SourceType 含 blu/dvd——拦截组名含 DIY 词形碰撞与非碟 DIY 字样）。
	if strings.Contains(strings.ToLower(meta.Title), "diy") ||
		strings.Contains(sub, "diy") || strings.Contains(sub, "自制") {
		st := strings.ToLower(meta.SourceType)
		if strings.Contains(st, "blu") || strings.Contains(st, "dvd") {
			out = append(out, model.TagKV{Key: "diy", Value: "yes"})
		}
	}
	return out
}
