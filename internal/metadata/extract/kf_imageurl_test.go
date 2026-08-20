package extract

import (
	"strings"
	"testing"
)

// §59.48: keepfrds 缩略图 URL 展开——三层结构实测锚定（243 Way.We.Talk）
func TestExpandKFImageUrl(t *testing.T) {
	// 实测三层结构（缩略图 URL → imgfetch 代理 → picgo 原图）
	thumb := "https://img.keepfrds.com/id__8GF1UTtycYFxcmPO8TrOjxy5Ex8p1xPJLi1p7FM/resize:fill:240:0:0/aHR0cHM6Ly9pbWdmZXRjaC5rZWVwZnJkcy5jb20vZmV0Y2g_dXJsPWh0dHBzJTNBJTJGJTJGb3JpZ2luLnBpY2dvLm5ldCUyRjIwMjYlMkYwOCUyRjEwJTJGVEhFLlJlZ2lvbi5QbGF5Lm9mZmVuY2UucG5nJnM9c2lnbg"
	got := ExpandKFImageUrl(thumb)
	if got == thumb {
		t.Fatalf("应展开: got 原样")
	}
	if !strings.Contains(got, "picgo.net") || !strings.HasPrefix(got, "https://origin.picgo.net") {
		t.Errorf("应展开到 picgo 直链: %q", got)
	}

	// 非 keepfrs 形态不碰
	foreign := "https://img3.pixhost.cc/images/5090/760869970_x.jpg"
	if got2 := ExpandKFImageUrl(foreign); got2 != foreign {
		t.Errorf("外站 URL 不应动: %q → %q", foreign, got2)
	}
	// 发布者直贴外站（keepfrds 域但无 resize 段）
	direct := "https://img.keepfrds.com/some/direct.png"
	if got3 := ExpandKFImageUrl(direct); got3 != direct {
		t.Errorf("无 resize 形态不应动: %q", got3)
	}
	// 空/空白
	if got4 := ExpandKFImageUrl(""); got4 != "" {
		t.Errorf("空串: %q", got4)
	}
}

func TestExpandKFImageUrlFallback(t *testing.T) {
	// imgfetch 形态但无 url 参数 → 用解出的代理 URL
	// base64("https://imgfetch.keepfrds.com/fetch") —— RawURLEncoding
	import_b64 := "aHR0cHM6Ly9pbWdmZXRjaC5rZWVwZnJkcy5jb20vZmV0Y2g"
	thumb := "https://img.keepfrds.com/sig/resize:fill:240:0:0/" + import_b64
	got := ExpandKFImageUrl(thumb)
	if got != "https://imgfetch.keepfrds.com/fetch" {
		t.Errorf("无参数应返回代理 URL: %q", got)
	}
	// 坏 base64 → 保留原 URL
	bad := "https://img.keepfrds.com/sig/resize:fill:240:0:0/!!!not-base64!!!"
	if got2 := ExpandKFImageUrl(bad); got2 != bad {
		t.Errorf("坏 b64 应保留原值: %q", got2)
	}
}

func TestExpandKFImageUrls(t *testing.T) {
	in := []string{
		"https://img3.pixhost.cc/images/1/2.jpg",
		"https://img.keepfrds.com/sig/resize:fill:240:0:0/aHR0cHM6Ly9vcmlnaW4ucGljZ28ubmV0L2EucG5n",
	}
	out := ExpandKFImageUrls(in)
	if out[0] != in[0] {
		t.Errorf("非匹配项被改动: %q", out[0])
	}
	if out[1] != "https://origin.picgo.net/a.png" {
		t.Errorf("匹配项未展开: %q", out[1])
	}
	if ExpandKFImageUrls(nil) != nil {
		t.Error("nil 应返回 nil")
	}
}
