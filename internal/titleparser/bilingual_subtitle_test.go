package titleparser

import "testing"

// §59.530 字幕双语键锚——用户语义框架：字幕双语=单轨内两种文字同显（对照
// 形态）·MI Text Title 的 & 双码=技术指纹·副标「XX双语字幕」声明词形=补充
// 通道·音轨「国英双语」（bilingual_audio）不得误触。
func TestBilingualSubtitle(t *testing.T) {
	// MI 含 CHS&ENG（Ip Man 实据形态）：泛键+简英精确键双产出
	hits := TagInferMatches(TagInferInput{
		Title:     "Ip Man Kung Fu Legend 2026 2160p UHD BluRay DoVi HDR x265 TrueHD 7.1 Atmos 3Audios-mUHD-FRDS",
		Subtitle:  "【宗师叶问2】mUHD作品 4k 杜比视界版本 国英双语 简繁英双语字幕",
		MediaInfo: "Text #3\nFormat : PGS\nTitle : CHS&ENG\nLanguage : Chinese\nText #4\nFormat : PGS\nTitle : CHT&ENG",
	})
	has := func(k string) bool {
		for _, h := range hits {
			if h == k {
				return true
			}
		}
		return false
	}
	if !has("bilingual_subtitle") {
		t.Errorf("MI 含 CHS&ENG/CHT&ENG 应产出 bilingual_subtitle（泛键）, got %v", hits)
	}
	if !has("bilingual_subtitle_chs_eng") {
		t.Errorf("MI 含 CHS&ENG 应产出 bilingual_subtitle_chs_eng（简英精确）, got %v", hits)
	}
	if !has("bilingual_audio") {
		t.Errorf("副标「国英双语」应产出 bilingual_audio（音轨域·独立判定）, got %v", hits)
	}

	// 仅 CHT&ENG（繁英对照）：泛键产出·简英精确键不产出（垃圾堆语义）
	hits2 := TagInferMatches(TagInferInput{
		MediaInfo: "Text #1\nFormat : PGS\nTitle : CHT&ENG",
	})
	has2 := func(k string) bool {
		for _, h := range hits2 {
			if h == k {
				return true
			}
		}
		return false
	}
	if !has2("bilingual_subtitle") {
		t.Errorf("MI 含 CHT&ENG 应产出泛键, got %v", hits2)
	}
	if has2("bilingual_subtitle_chs_eng") {
		t.Error("CHT&ENG（繁英对照）不应产出简英精确键")
	}

	// MI 缺失+副标声明词形：泛键产出（声明补充通道）·精确键不产出（仅 MI）
	hits3 := TagInferMatches(TagInferInput{
		Subtitle: "mUHD作品 简繁英双语字幕",
	})
	found3 := false
	for _, h := range hits3 {
		if h == "bilingual_subtitle" {
			found3 = true
		}
		if h == "bilingual_subtitle_chs_eng" {
			t.Error("声明词形不得产出简英精确键（发布者写的简英不严谨·铁证才算数）")
		}
	}
	if !found3 {
		t.Errorf("副标「双语字幕」词形应产出泛键（声明通道）, got %v", hits3)
	}

	// 纯音轨词形（国英双语·无字幕词）：字幕双键不得误触
	hits4 := TagInferMatches(TagInferInput{
		Subtitle: "mUHD作品 国英双语",
	})
	for _, h := range hits4 {
		if h == "bilingual_subtitle" || h == "bilingual_subtitle_chs_eng" {
			t.Errorf("纯音轨词形不得误触字幕双键, got %v", hits4)
		}
	}
}
