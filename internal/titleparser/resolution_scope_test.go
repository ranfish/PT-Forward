package titleparser

import "testing"

// §59.261: 4K/8K 变形宽银幕（scope）宽度优先——只裁高不裁宽，
// 高度落 1000-1999 区间曾被误判 1080p（Harry Potter 3840×1604/Samaritan 3840×1600 案）。
func TestResolutionFromHeightOrWidth_Scope4K(t *testing.T) {
	cases := []struct {
		name, height, width, want string
	}{
		{"4K scope 1604", "1 604", "3 840", "2160p"},
		{"4K scope 1600", "1600", "3840", "2160p"},
		{"4K flat 2160", "2 160", "3 840", "2160p"},
		{"8K scope 3200", "3200", "7 680", "4320p"},
		{"8K flat 4320", "4 320", "7 680", "4320p"},
		{"1080p flat", "1 080", "1 920", "1080p"},
		{"1080p scope 800", "800", "1 920", "1080p"},
		{"1080p IMAX 裁高 1040", "1 040", "1 920", "1080p"},
		{"2K DCI scope", "858", "2 048", "1080p"},
		{"高度缺失仅宽度 4K", "", "3840", "2160p"},
		{"宽度缺失仅高度 4K flat", "2160", "", "2160p"},
		{"全空", "", "", ""},
	}
	for _, tc := range cases {
		if got := resolutionFromHeightOrWidth(tc.height, tc.width); got != tc.want {
			t.Errorf("[%s] (%q,%q)=%q want %q", tc.name, tc.height, tc.width, got, tc.want)
		}
	}
}
