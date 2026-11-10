package publish

// hdwing_inject.go §59.511: 瓷器（HDWinG）发布适配——type 混维路由+
// 固定字段注入+官组回流规则。值集/惯例依据 docs/32-站点适配器设计/hdwing.md §5。

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
)

// hdwingTypeByLabel type 域 label 精确查（路由类值无 standard_keys·由
// hdwingCategoryValue 按 label 取——"4K UltraHD"/"原盘(Full BD)" 等站方原文）。
func hdwingTypeByLabel(cfg *model.PublishFormConfig, label string) *model.FormValueMapping {
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Label == label {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// reHdwingMultiSeason 多季合集信号（S01-S04/第一季至第四季形态）——
// 剧集"剧"桶(13/25/26/24) vs "剧集包"桶(21/22/23) 分流。
var reHdwingMultiSeason = regexp.MustCompile(`(?i)S\d{1,2}\s*-\s*S\d{1,2}|第[一二三四五六七八九十\d]+季.{0,6}第[一二三四五六七八九十\d]+季`)

// hdwingCategoryValue §59.511: 瓷器 type 混维路由（typeMappingOf 第十一站）。
// 站内惯例实证（docs/32/hdwing.md §5.2 抽样）：
//   ① 4K 大桶：r2160p/r4320p 一律 410（任意媒介/类型·分辨率优先）
//   ② 电影层（非4K）：原盘→20；Remux/Encode/WEB-DL→17（同桶）；
//      1080i→16；720p/SD→9
//   ③ 剧集层：产区轴（PTGen Region 首值优先·缺失兜底欧美 13）·
//      多季合集→包桶 21/22/23
//   ④ 类型直映射：animation 14/documentary 5/sports 15/tv_shows 401/
//      concert 402/mv 406/music 408（走通用 standard_keys 链）
// 分辨率判定源：tp.Resolution 优先+tags 推理资产兜底（§59.500 完整才
// 推理单点产物——用户定案"按我们的标签消费"）。
func (e *PublishExecutor) hdwingCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "瓷器" || cfg == nil || meta == nil {
		return nil
	}
	// 媒介族判定：mediumMappingOf 判定结果（§59.451 mteam 同源链——归一损失免疫）
	medStdKeys := hdwingMedStdKeysOf(e.mediumMappingOf(cfg, siteName, meta, tp))
	// 分辨率：tp 优先 + tags 推理兜底
	res := tp.Resolution
	if res == "" {
		res = resolutionTagOfMeta(meta)
	}
	rk := extract.LookupStandardKey("resolution", res)

	// ① 4K 大桶（分辨率优先于类型/媒介）
	if rk == "resolution.r2160p" || rk == "resolution.r4320p" {
		return hdwingTypeByLabel(cfg, "4K UltraHD")
	}

	switch meta.Category {
	case "category.movie":
		// ② 媒介分桶：原盘族→20（§59.451 mteam 同源判定链——medium 的
		// StandardKeys 判 bluray 族·标题归一损失防御）
		for _, k := range medStdKeys {
			if k == "medium.bluray" || k == "medium.uhd_bluray" || k == "medium.hddvd" {
				return hdwingTypeByLabel(cfg, "原盘(Full BD)")
			}
		}
		switch rk {
		case "resolution.r1080i":
			return hdwingTypeByLabel(cfg, "电影Movie(1080i)")
		case "resolution.r720p", "resolution.sd":
			return hdwingTypeByLabel(cfg, "电影Movie(720p)")
		}
		return hdwingTypeByLabel(cfg, "电影Movie(1080p)")

	case "category.tv_series":
		// ③ 产区轴（多季合集→包桶）
		pack := reHdwingMultiSeason.MatchString(meta.Title)
		for _, raw := range ptgenRegionGenreWords(meta, model.FieldDomainRegion) {
			switch {
			case strings.Contains(raw, "日本"):
				if pack {
					return hdwingTypeByLabel(cfg, "日韩剧集包")
				}
				return hdwingTypeByLabel(cfg, "日剧")
			case strings.Contains(raw, "韩国"):
				if pack {
					return hdwingTypeByLabel(cfg, "日韩剧集包")
				}
				return hdwingTypeByLabel(cfg, "韩剧")
			case strings.Contains(raw, "中国") || strings.Contains(raw, "香港") || strings.Contains(raw, "台湾"):
				if pack {
					return hdwingTypeByLabel(cfg, "中港台剧集包")
				}
				return hdwingTypeByLabel(cfg, "中港台剧集")
			case strings.Contains(raw, "美国") || strings.Contains(raw, "英国") || strings.Contains(raw, "欧美"):
				if pack {
					return hdwingTypeByLabel(cfg, "欧美剧集包")
				}
				return hdwingTypeByLabel(cfg, "欧美剧")
			}
		}
		// 产区缺失兜底：欧美最大桶（用户定案·regionMappingOf miss 兜底同款）
		if pack {
			return hdwingTypeByLabel(cfg, "欧美剧集包")
		}
		return hdwingTypeByLabel(cfg, "欧美剧")
	}
	// ④ 类型直映射走通用链（animation/documentary/sports/tv_shows/
	// concert/mv/music 的 standard_keys 已配 value_mappings）
	return nil
}

// resolutionTagOfMeta tags 推理资产兜底（§59.500 inferFinalTags 产物——
// meta.Tags JSON 的 resolution 类标签·r2160p/r4320p canonical 值）。
func resolutionTagOfMeta(meta *model.TorrentMetadata) string {
	if meta == nil || meta.Tags == "" {
		return ""
	}
	var tags []string
	if err := json.Unmarshal([]byte(meta.Tags), &tags); err != nil {
		return ""
	}
	for _, t := range tags {
		switch t {
		case "resolution.r2160p", "2160p", "4k":
			return "2160p"
		case "resolution.r4320p", "4320p", "8k":
			return "4320p"
		case "resolution.r1080p", "1080p":
			return "1080p"
		case "resolution.r720p", "720p":
			return "720p"
		}
	}
	return ""
}

// hdwingPublishInject §59.511: 瓷器固定字段注入（haidanSeasonEpisodeInject
// 同款站特化模式·挂 jobs 段后 DryRun 检查点前）：
//   - share_rule 恒 3「欢迎转载」（§59.511 用户定案权利链：R7 保证源头
//     开放·转载者无权禁止/限制·如实转述源头开放态——与「互珍互重禁转PTT」
//     文本声明两层各表）
//   - board 恒 1「普通区」（官组专区需站方授权身份·转载者恒普通区）
//   - is3d：TechProfile.Stereo3D 非空勾 yes（3D 封装·§59.197 字段）
// 未配置对应 form_fields 键的站零动作（防御——配置缺失不注入）。
func hdwingPublishInject(form map[string]string, cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile) {
	if cfg == nil {
		return
	}
	if fn, ok := cfg.FormFields["share_rule"]; ok && fn != "" {
		form[fn] = "3"
	}
	if fn, ok := cfg.FormFields["board"]; ok && fn != "" {
		form[fn] = "1"
	}
	if tp.Stereo3D != "" {
		if fn, ok := cfg.FormFields["is3d"]; ok && fn != "" {
			form[fn] = "yes"
		}
	}
}

// checkOfficialGroupBackflow §59.511 规则⑥: 官组种禁回流源站——全站通用。
// 种子制作组 → release_group_mappings(is_official=1) 归属站 == 目标站
// → 拦截（官组种源头即目标站·转回=DUPE+站规双风险；251 组映射全站自动
// 生效）。组名提取源=ExtractGroupName(meta.Title)（teamMapping 同源）。
func (e *PublishExecutor) checkOfficialGroupBackflow(ctx context.Context, meta *model.TorrentMetadata, targetSite string) (bool, string) {
	if meta == nil || targetSite == "" {
		return false, ""
	}
	group := util.ExtractGroupName(meta.Title)
	if group == "" {
		return false, ""
	}
	var m model.ReleaseGroupMapping
	if err := e.db.WithContext(ctx).
		Where("LOWER(group_name) = LOWER(?) AND is_official = 1", group).
		First(&m).Error; err != nil {
		return false, ""
	}
	if m.SiteName == targetSite || m.Domain == targetSite {
		return true, fmt.Sprintf("官组种回流拦截：%s 是「%s」的官组——该种源头即目标站，转回=DUPE+站规双风险（§59.511 规则⑥）", group, targetSite)
	}
	return false, ""
}

// hdwingMedStdKeysOf 媒介 StandardKeys 提取（hdwingCategoryValue 电影层
// 原盘判定输入——mediumMappingOf 结果的单源同判定链 §59.451）。
func hdwingMedStdKeysOf(med *model.FormValueMapping) []string {
	if med == nil {
		return nil
	}
	return med.StandardKeys
}
