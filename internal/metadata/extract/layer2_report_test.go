package extract

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.35 Layer 2 引用完整性【报告模式】。
//
// P1 实施发现：站点 standard_keys 值域远大于 Layer 1（2309 项不在通用词表）——
// 这是站点历史事实（team.*/tag.*/开放分类树/音乐媒介等站点专属域），不是错误。
// 因此校验语义从设计稿的"fail-fast 约束"修正为"报告模式"：
//   - 闭合技术域（video./audio./hdr./resolution.）内站点使用但 Layer 1 缺失的 key
//     形成快照清单——新增缺失说明站点数据变化，人工评估是否增补 Layer 1
//   - 开放值域（category./medium./source./team./tag./...）不校验
//
// 待治理清单（发布页重构时 Layer 2 统一）：
//   audio.ac3(75站, Layer1=audio.dd) / audio.dtsx(38站, Layer1=audio.dts_x) /
//   audio.truehd_atmos(30站, Atmos 是独立字段) / *.other(站点兜底) /
//   audio.ogg / audio.m4a / audio.dts_hd / video.mpeg4 / video.prores 等
func TestLayer2ClosedDomainReport(t *testing.T) {
	// Layer 1 key 集
	layer1 := map[string]bool{}
	for _, domain := range titleparser.DictDomains() {
		for _, tok := range titleparser.DictTokens(domain) {
			if tok.StandardKey != "" {
				layer1[tok.StandardKey] = true
			}
		}
	}
	if len(layer1) == 0 {
		t.Fatal("Layer 1 key 集为空")
	}

	// 闭合域前缀
	closedPrefixes := []string{"video.", "audio.", "hdr.", "resolution."}
	miss := map[string]int{}
	loadConfig()
	for _, cfg := range sitesByDomain {
		for _, m := range cfg.StandardKeys {
			for _, key := range m {
				if !keyHasPrefix(key, closedPrefixes) {
					continue
				}
				if !layer1[key] {
					miss[key]++
				}
			}
		}
	}
	// 已知待治理清单（快照）：新增缺失 → 评估增补；清单缩小 → 更新此处
	knownMissing := map[string]bool{
		"video.other": true, "video.mpeg4": true, "video.prores": true,
		"video.mpeg": true, "video.avs": true, "video.mvc": true,
		"video.avs_plus": true, "video.avs3": true, "video.h261": true,
		"video.divx": true, "video.x264_10bit": true, "video.mpeg1": true,
		"audio.other": true, "audio.ac3": true, "audio.dtsx": true,
		"audio.truehd_atmos": true, "audio.ogg": true, "audio.m4a": true,
		"audio.dts_hd": true, "audio.atmos": true, "audio.mpeg": true,
		"audio.av3a": true, "audio.pcm": true, "audio.taa": true,
		"audio.ddp_atmos": true, "audio.dolby_atmos": true, "audio.dts_hdma": true,
		"audio.dts_hdma_x71": true, "audio.tta": true, "audio.avsa": true,
		"audio.eac3": true, "audio.av3v": true, "audio.dts_hd_hra": true,
		"audio.audio_vivid": true, "audio.dtshd_ma": true, "audio.dts_hr": true,
		"resolution.other": true, "resolution.sd": true,
		"resolution.r4k": true, "resolution.r8640p": true, "resolution.ipad": true,
		"resolution.r360p": true, "resolution.r540p": true, "resolution.ed": true,
		"resolution.r8k": true, "resolution.r576p": true,
	}
	unexpected := []string{}
	for k := range miss {
		if !knownMissing[k] {
			unexpected = append(unexpected, k)
		}
	}
	if len(unexpected) > 0 {
		t.Errorf("闭合域出现新的 Layer 1 缺失 key（评估增补或加入快照）: %v", unexpected)
	}
	for k := range knownMissing {
		if _, used := miss[k]; !used {
			t.Logf("快照 key %s 已不在站点数据中（可从待治理清单移除）", k)
		}
	}
}

func keyHasPrefix(key string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}
