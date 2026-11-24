package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// TestAssembleDescription_ScreenshotFieldSplit §59.541 附十一 回归锚：
// 配置 screenshots 域 → 截图不内嵌 descr（走独立框）；未配域 → 内嵌
// 行为不变（NP 主流零回归）。
func TestAssembleDescription_ScreenshotFieldSplit(t *testing.T) {
	meta := func() *model.TorrentMetadata {
		return &model.TorrentMetadata{
			Description: "◎片　　名　Test\n◎简　　介　剧情",
			Screenshots: "[\"https://img.example/1.jpg\",\"https://img.example/2.jpg\"]",
		}
	}

	// ① 未配 screenshots 域：内嵌行为不变（NP 主流）
	cfgPlain := &model.PublishFormConfig{FormFields: map[string]string{"description": "descr"}}
	out := assembleDescription(meta(), cfgPlain)
	if !strings.Contains(out, "[img]https://img.example/1.jpg[/img]") {
		t.Fatalf("① 未配域应内嵌截图，got: %q", out)
	}

	// ② 配 screenshots 域：descr 不内嵌
	cfgSplit := &model.PublishFormConfig{FormFields: map[string]string{
		"description": "descr", "screenshots": "screenshots"}}
	out = assembleDescription(meta(), cfgSplit)
	if strings.Contains(out, "[img]") {
		t.Fatalf("② 配域后 descr 不应含 [img]，got: %q", out)
	}
	if !strings.Contains(out, "◎片　　名") {
		t.Fatalf("② 简介正文应保留，got: %q", out)
	}
}

// TestAssembleDescription_CoverFieldPosterRetained §59.569 锚（行为反转）：
// 配置 cover 域 → 简介首行海报 [img] 保留（cover 是额外投递·不替代简介内海报）；
// 未配域 → 同样保留（原行为）。§59.541 附十一剥离已撤（误伤好学/瓷器·劳改所冗余）。
func TestAssembleDescription_CoverFieldPosterRetained(t *testing.T) {
	meta := func() *model.TorrentMetadata {
		return &model.TorrentMetadata{
			Description: "[img]https://db.example/poster.jpg[/img]\n◎片　　名　Test\n◎简　　介　剧情",
		}
	}

	// ① 未配 cover 域：海报原样保留
	cfgPlain := &model.PublishFormConfig{FormFields: map[string]string{"description": "descr"}}
	out := assembleDescription(meta(), cfgPlain)
	if !strings.Contains(out, "[img]https://db.example/poster.jpg[/img]") {
		t.Fatalf("① 未配域海报应保留，got: %q", out)
	}

	// ② 配 cover 域：首行海报同样保留（cover 框是复制投递·非替代）
	cfgCover := &model.PublishFormConfig{FormFields: map[string]string{
		"description": "descr", "cover": "cover", "screenshots": "screenshots"}}
	out = assembleDescription(meta(), cfgCover)
	if !strings.Contains(out, "[img]https://db.example/poster.jpg[/img]") {
		t.Fatalf("② 配域后海报应保留（cover 额外投递不替代），got: %q", out)
	}
	if !strings.Contains(out, "◎片　　名　Test") {
		t.Fatalf("② 正文应保留，got: %q", out)
	}

	// ③ 配 cover 域 + DescrQuoteOnly（劳改所形态）：正文全跳（含海报）·仅引用块
	cfgQuote := &model.PublishFormConfig{
		DescrQuoteOnly: true,
		FormFields:     map[string]string{"description": "descr", "cover": "cover"},
	}
	meta2 := &model.TorrentMetadata{Statement: "声明", Description: "[img]https://db.example/poster.jpg[/img]\n◎片　　名　Test"}
	out = assembleDescription(meta2, cfgQuote)
	if strings.Contains(out, "https://db.example/poster.jpg") {
		t.Fatalf("③ 劳改所 DescrQuoteOnly 应跳过整个简介（含海报），got: %q", out)
	}
	if !strings.Contains(out, "[quote]") {
		t.Fatalf("③ 引用块应在场，got: %q", out)
	}
}

// TestAssembleDescription_DescrQuoteOnly §59.541 附十二 回归锚：
// DescrQuoteOnly=true（劳改所）→ descr 仅引用块·跳过 PTGen 正文
// （站方独立「豆瓣信息」渲染区自动解析·descr 里重复冗余）；
// 未开启 → 简介正文保留（默认行为零变化）。
func TestAssembleDescription_DescrQuoteOnly(t *testing.T) {
	meta := func() *model.TorrentMetadata {
		return &model.TorrentMetadata{
			Statement:   "Remux来自 Test 1999 BluRay REMUX AVC-FLS\n感谢以上所有素材提供者！",
			Description: "◎片　　名　Test\n◎译　　名　测试\n◎简　　介　剧情正文很长很长",
		}
	}

	// ① 默认（未开启）：引用块+正文
	cfgPlain := &model.PublishFormConfig{FormFields: map[string]string{"description": "descr"}}
	out := assembleDescription(meta(), cfgPlain)
	if !strings.Contains(out, "[quote]") || !strings.Contains(out, "◎片　　名") {
		t.Fatalf("① 默认应含引用块+正文，got: %q", out[:min(100, len(out))])
	}

	// ② DescrQuoteOnly=true：仅引用块·无正文
	cfgQuote := &model.PublishFormConfig{
		DescrQuoteOnly: true,
		FormFields:     map[string]string{"description": "descr", "pt_gen": "pt_gen"},
	}
	out = assembleDescription(meta(), cfgQuote)
	if !strings.Contains(out, "[quote]") {
		t.Fatalf("② 引用块应在场，got: %q", out)
	}
	if strings.Contains(out, "◎片　　名") || strings.Contains(out, "剧情正文") {
		t.Fatalf("② PTGen 正文不应在场（DescrQuoteOnly），got: %q", out)
	}
}

// TestStripLeadingPosterImage 随函数删除（§59.569 撤 §59.541 附十一）
