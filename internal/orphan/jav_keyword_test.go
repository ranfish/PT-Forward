package orphan

import (
	"testing"
)

// §59.225 JAV 番号专用路径——关键词提取。
func TestExtractJAVKeyword(t *testing.T) {
	cases := []struct {
		in     string
		kw     string
		isJAV  bool
	}{
		{"MDVR-400.8K", "MDVR-400", true},
		{"SVVRT-079_4K", "SVVRT-079", true},
		{"savr-1007", "SAVR-1007", true},
		
		{"ajvr-306", "AJVR-306", true},
		{"MDVR-413", "MDVR-413", true},
		// 非成人形态不触发
		{"Movie.2020.1080p.BluRay", "", false},
		{"侠女.1970.1080p.国语", "", false},
		{"Inception.2010.BluRay.x264", "", false},
		// 音乐编码（RIAJ）不误伤——需有音乐标记在余文中才排除
		{"ABCD-123", "ABCD-123", true}, // 纯番号无标记=JAV
	}
	for _, c := range cases {
		kw, ok := extractJAVKeyword(c.in)
		if ok != c.isJAV || (ok && kw != c.kw) {
			t.Errorf("extractJAVKeyword(%q) = (%q,%v), want (%q,%v)", c.in, kw, ok, c.kw, c.isJAV)
		}
	}
}

// §59.225: 前导数字形态（3dsvr-1912 → 3DSVR-1912 带前导 3）
func TestExtractJAVKeywordLeadingDigit(t *testing.T) {
	kw, ok := extractJAVKeyword("3dsvr-1912")
	if !ok || kw != "3DSVR-1912" {
		t.Errorf("extractJAVKeyword(\"3dsvr-1912\") = (%q,%v), want (\"3DSVR-1912\",true)", kw, ok)
	}
}
