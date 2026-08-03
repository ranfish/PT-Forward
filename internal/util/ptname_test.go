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
