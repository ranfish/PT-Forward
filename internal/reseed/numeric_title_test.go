package reseed

import (
	"strings"
	"testing"
)

// §59.201 数字片名家族——"65.2023.REPACK..."（片名=65）曾被两处误杀：
// ①数字前缀剥离当合集序号（65 消失）②KeywordHasNoTitle 当无标题（0s 秒拒跳过 L2）。
func TestNumericMovieTitle(t *testing.T) {
	// ① 前缀不剥：65/300 保留
	if kw := ExtractSearchKeyword("65.2023.REPACK.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265-UBits.mkv"); !strings.HasPrefix(kw, "65 ") {
		t.Errorf("numeric title 65 stripped: %q", kw)
	}
	if kw := ExtractSearchKeyword("300.2006.1080p.BluRay.x264-CMCT"); !strings.HasPrefix(kw, "300 ") {
		t.Errorf("numeric title 300 stripped: %q", kw)
	}
	// ② 不再判"无标题"——数字+年份相邻=片名
	if KeywordHasNoTitle("65 2023 2160p WEB-DL") {
		t.Error("65 2023 … must be treated as titled")
	}
	if KeywordHasNoTitle("300 2006 1080p") {
		t.Error("300 2006 … must be treated as titled")
	}
	// 回归：纯规格词仍判无标题；合集序号剥除保持
	if !KeywordHasNoTitle("2023 2160p WEB-DL") {
		t.Error("all-spec keyword must stay titleless")
	}
	if kw := ExtractSearchKeyword("12.仙履奇缘.Cinderella.2015.1080p"); kw != "Cinderella 2015 1080p" {
		t.Errorf("collection index strip regressed: %q", kw)
	}
	if kw := ExtractSearchKeyword("E09.The.Man.With.The.Golden.Gun.1974.720p.BluRay.x264.DTS-WiKi"); kw != "The Man With The Golden Gun 1974" { // §59.206 超限剥规格词
		t.Errorf("E-prefix strip regressed: %q", kw)
	}
}

// §59.216: 行首短数字守卫越权修复——"0 0兆赫 2019"（0.0MHz 点变空格形态）
// 其余 token 含标题成分（0兆赫）时不判无标题（官站 tid=281254 3.23GB
// 同版本，"0 0兆赫 2019" 形态实测 6 行命中）。
func TestNumericUnitTitleMHz(t *testing.T) {
	if KeywordHasNoTitle("0 0兆赫 2019") {
		t.Error("\"0 0兆赫 2019\" must be titled (0兆赫 混合 token)")
	}
	// 回归：序号污染形态保持无标题判定
	if !KeywordHasNoTitle("2 2016 1080p") {
		t.Error("\"2 2016 1080p\" must stay titleless")
	}
}
