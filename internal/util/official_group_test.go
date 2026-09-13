package util

import "testing"

func TestOfficialGroupKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"cXcY@FRDS", "FRDS"},
		{"SHB931@UBits", "UBits"},
		{"CMCT", "CMCT"},
		{"cXcY@", "cXcY@"},   // 空 @ 后段——原样（LastIndex 条件排除）
		{"@FRDS", "FRDS"},     // 空前段——后段仍有效
		{"a@b@FRDS", "FRDS"},  // 多 @ 取最末
	}
	for _, c := range cases {
		if got := OfficialGroupKey(c.in); got != c.want {
			t.Errorf("OfficialGroupKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
