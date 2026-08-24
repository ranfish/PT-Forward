package util

import "testing"

func TestExtractGroupName(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		// "-" 分隔符
		{"Title.2024-CMRG", "CMRG"},
		{"Movie.2024-CMRG.mkv", "CMRG"},
		// "-" 分隔符 + @
		{"Movie-hyb9373@CMCT", "CMCT"},
		// ￡ 分隔符 + 尾部中文
		{"乾隆王朝.2002.40集全￡CMCT小鱼", "CMCT"},
		{"2001太空漫游.4K修复版.1968.中英字幕￡CMCT旧梦", "CMCT"},
		// 年份范围 + ￡："-" 分支提取出含 CJK/点的无效组名 → fall through ￡
		{"超级工程Ⅰ.Ⅱ合集.2012-2016.国语中字￡CMCT梦幻", "CMCT"},
		{"皮克斯短片合集.1984-2012.中英字幕￡CMCT呆呆", "CMCT"},
		// NOGROUP 等忽略
		{"Movie.2024-NOGROUP", ""},
		{"Movie.2024-N/A", ""},
		// 无分隔符
		{"随机标题", ""},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := ExtractGroupName(tt.title)
			if got != tt.want {
				t.Errorf("ExtractGroupName(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

// §59.64: 尾部促销标记（克隆站把促销状态渲染进种子标题，NBSP 分隔）不阻断组名提取。
func TestExtractGroupName_PromoSuffix(t *testing.T) {
	cases := []struct{ in, want string }{
		// NBSP + 促销标记（243 The.Boys 实锤形态）
		{"The Boys S03 2022 1080p WEBRip DDP5.1 x265 10bit-Yumi@FRDS\u00a0\u00a0\u00a0 [2X 50%]", "FRDS"},
		// 多枚连贴
		{"Show.2024.1080p-CMRG [Free] [2X 50%]", "CMRG"},
		// 普通空格 + 单标记
		{"Movie.2023.2160p-BHD [Free]", "BHD"},
		// 干净标题不受影响（既有行为回归锚）
		{"The Boys S04 2024 1080p WEBRip DDP5.1 x265 10bit-Yumi@FRDS", "FRDS"},
		{"Title.2024-CMRG", "CMRG"},
	}
	for _, c := range cases {
		if got := ExtractGroupName(c.in); got != c.want {
			t.Errorf("ExtractGroupName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
