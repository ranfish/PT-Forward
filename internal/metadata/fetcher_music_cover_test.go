package metadata

import "testing"

// §59.485: pixhost 白名单判定（唯一白名单——资产单一化定案）。
func TestIsPixhostURL(t *testing.T) {
	cases := []struct {
		u    string
		want bool
	}{
		{"https://pixhost.to/show/abc.jpg", true},
		{"https://img3.pixhost.to/images/6996/800453564_hermes.jpg", true},
		{"http://pixhost.to/x.jpg", true},
		{"https://img.musopia.vip/img/6c2620806c/3109/original.jpg", false},
		{"https://origin.picgo.net/2026/08/09/1bf9a620.gif", false},
		{"attachments/201303/201303141245027ff349ac3d6f4d79b14d63c300a796a8.jpg", false},
		{"", false},
		{"pic/trans.gif", false},
	}
	for _, c := range cases {
		if got := isPixhostURL(c.u); got != c.want {
			t.Errorf("isPixhostURL(%q)=%v want %v", c.u, got, c.want)
		}
	}
}
