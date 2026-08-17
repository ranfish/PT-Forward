package titleparser

import "testing"

// §59.35 决策 2：platform Requires 语义约束。
//
// 风险模型：2 字符缩写（CR/NF/ID/CC...）在普通英文标题中独立成词概率不低，
// 误提取会从标题剥词污染 main_title。requires="web" 把误命中方向从
// "剥词污染标题" 变为 "不提取"（空值无损）——v1.05 :109-121 platform 是
// WEB/HDTV 类资源的"内容分发方"字段，非 WEB 标题本就不该有它。
func TestPlatformRequiresWeb(t *testing.T) {
	cases := []struct {
		name, title, wantPlatform, wantMainTitle string
	}{
		// WEB 上下文：2 字符缩写正常提取
		{"nf_webdl", "Movie 2024 1080p NF WEB-DL H.264-CMCT", "NF", "Movie"},
		{"cr_webdl", "Anime 2023 1080p CR WEB-DL AAC-GROUP", "CR", "Anime"},
		{"id_hulu", "Show 2022 1080p HULU WEB-DL DD5.1-X", "HULU", "Show"},
		{"cc_requires_web", "Film 2021 1080p CC WEB-DL H.264-X", "CC", "Film"},
		// 非 WEB 上下文：2 字符缩写不提取、不剥词（主标题完整保留）
		{"cr_bluray_no_pollute", "The CR Transaction 2019 BluRay x264-GROUP", "", "The CR Transaction"},
		{"nf_no_web", "NF Mystery 2018 1080p BluRay x264-GROUP", "", "NF Mystery"},
		{"id_in_title", "Id Ego Superego 2020 BluRay x264-G", "", "Id Ego Superego"},
		// 3+ 字符缩写无需 WEB 上下文（最小约束）
		{"amzn_anytime", "Movie 2024 1080p AMZN WEB-DL-X", "AMZN", "Movie"},
		{"hmax_webdl", "Show 2023 2160p HMAX WEB-DL DV H.265-X", "HMAX", "Show"},
		// HDTV 也算 WEB 上下文（电视源）
		{"tbs_hdtv", "Show 2019 1080p TBS HDTV x264-GROUP", "TBS", "Show"},
		// 大小写敏感词条
		{"itunes_case", "Movie 2020 1080p iT WEB-DL AAC-GROUP", "iT", "Movie"},
		{"it_upper_not_matched", "Movie IT Department 2019 1080p BluRay x264-G", "", "Movie IT Department"},
		// legacy 词条保留
		{"friday_legacy", "Drama 2021 1080p friDay WEB-DL H.264-CMCT", "friDay", "Drama"},
		// 地区码兼容词条挂 requires=web——非 WEB 标题不再误剥词（"You Can Count on Me" 案例）
		{"hkg_web_only", "Movie 2020 1080p HKG BluRay x264-CMCT", "", "Movie HKG"},
		{"hkg_webdl", "Movie 2020 1080p HKG WEB-DL H.264-CMCT", "HKG", "Movie"},
	}
	for _, c := range cases {
		got := ParseTitle(c.title)
		if got.SourcePlatform != c.wantPlatform {
			t.Errorf("%s: SourcePlatform = %q, want %q", c.name, got.SourcePlatform, c.wantPlatform)
		}
		if got.MainTitle != c.wantMainTitle {
			t.Errorf("%s: MainTitle = %q, want %q（主标题被污染）", c.name, got.MainTitle, c.wantMainTitle)
		}
	}
}

// §59.35 P2：platform 域完整性——wiki 174 + legacy 46，requires=web 全部 ≤2 字符
func TestPlatformDictIntegrity(t *testing.T) {
	tokens := DictTokens("platform")
	if len(tokens) < 200 {
		t.Errorf("platform 词条 %d，预期 ≥200（wiki ~174 + legacy ~46）", len(tokens))
	}
	seen := map[string]bool{}
	for _, tok := range tokens {
		if seen[tok.Canonical] {
			t.Errorf("canonical 重复: %q", tok.Canonical)
		}
		seen[tok.Canonical] = true
		if tok.Requires == "web" {
			// requires=web 限短缩写（≤2）与危险常用词/地区码（LIFE/NOW/CAN/HKG...）
			// ≥5 字符专有缩写不需要约束
			n := 0
			for _, r := range tok.Canonical {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
					n++
				}
			}
			if n > 4 {
				t.Errorf("requires=web 词条 %q 长度 >4（约束过度）", tok.Canonical)
			}
		}
	}
	// 关键词条锚定
	anchors := []string{"AMZN", "NF", "DSNP", "ATVP", "HULU", "PCOK", "CR", "iT", "iP", "HMAX", "MAX", "friDay", "Baha", "U-NEXT", "HKG"}
	for _, a := range anchors {
		if !seen[a] {
			t.Errorf("platform 缺关键词条 %q", a)
		}
	}
}

// §59.35 P2：词条顺序语义——长缩写在前（HMAX 不得被 MAX 抢、CMAX 不得被 MAX 抢）
func TestPlatformLongestFirst(t *testing.T) {
	order := map[string]int{}
	for i, tok := range DictTokens("platform") {
		order[tok.Canonical] = i
	}
	pairs := [][2]string{
		{"HMAX", "MAX"}, {"CMAX", "MAX"}, {"FUNi", "iT"},
		{"TVBAnywhere", "TVB"}, {"MyTVSuper", "MyTVS"},
	}
	for _, p := range pairs {
		if order[p[0]] > order[p[1]] {
			t.Errorf("词条顺序：%q 应在 %q 之前（长缩写优先）", p[0], p[1])
		}
	}
	// 行为验证
	got := ParseTitle("Show 2023 2160p HMAX WEB-DL H.265-X")
	if got.SourcePlatform != "HMAX" {
		t.Errorf("HMAX 被 MAX 抢: SourcePlatform = %q", got.SourcePlatform)
	}
}
