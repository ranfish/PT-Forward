package publish

import "testing"

// §59.62: Complete name 行去路径——本地 MI 发布到目标站不应携带服务器磁盘路径。
func TestSanitizeCompleteName(t *testing.T) {
	in := `General
Unique ID                                : 123456789
Complete name                            : /home/pt/pt1/FRDS/某电影.2026.UHD.BluRay/某电影.2026.UHD.BluRay.2160p.mkv
Format                                   : Matroska
File size                                : 50.0 GiB`

	want := `General
Unique ID                                : 123456789
Complete name                            : 某电影.2026.UHD.BluRay.2160p.mkv
Format                                   : Matroska
File size                                : 50.0 GiB`

	if got := sanitizeCompleteName(in); got != want {
		t.Errorf("sanitizeCompleteName:\n%q\nwant:\n%q", got, want)
	}
}

func TestSanitizeCompleteName_NoCompleteName(t *testing.T) {
	in := "General\nFormat : Matroska"
	if got := sanitizeCompleteName(in); got != in {
		t.Errorf("无 Complete name 行应原样返回: %q", got)
	}
}
