package adapter

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.542 追记十三锚：keepfrds 详情页 h1 badge 优先判定（用户定案——
// 详情页主链必达零额外请求·老种翻页列表锚定 miss 不影响禁转判定）。
// 夹具形态来自 243 实拉（tid 2782403·2026-11-20 消毒）。

// 朋友站 tid 2782403 详情页 h1 实拉形态（消毒截取）。
const keepfrdsH1Real = `<h1 align="center" id="top">【查理·威尔森的战争/盖世奇才(台)/韦氏风云】mUHD作品 4k 杜比视界版本 英语 简繁英双语特效字幕 带章节名&nbsp;&nbsp;&nbsp; <b>[<font class='recommended'>禁转</font>]</b></h1>`

// 列表页实拉佐证形态（tid 2782612·badge 同 class——限时禁转）。
const keepfrdsListLimited = `<b>[<font class='recommended'>限时禁转</font>]</b>`

// 无禁转种 h1 形态（麒麟 tid 443135 同款结构——折扣 badge 用其它 class）。
const keepfrdsH1NoBadge = `<h1 align="center" id="top">Some Torrent 2020 2160p&nbsp;&nbsp;&nbsp; <b>[<font class='halfdown' >50%</font>]</b><span style="margin-left: 6px" title="通过"><svg viewBox="0 0 1413 1024"></svg></span></h1>`

func TestKeepfrdsH1Badge(t *testing.T) {
	t.Run("实拉形态命中禁转", func(t *testing.T) {
		if got := keepfrdsH1Badge(keepfrdsH1Real); got != "<b>[禁转]</b>" {
			t.Errorf("badge = %q, want <b>[禁转]</b>", got)
		}
	})
	t.Run("限时禁转同形态命中", func(t *testing.T) {
		h1 := `<h1 id="top">Title&nbsp;&nbsp;&nbsp; ` + keepfrdsListLimited + `</h1>`
		if got := keepfrdsH1Badge(h1); got != "<b>[限时禁转]</b>" {
			t.Errorf("badge = %q, want <b>[限时禁转]</b>", got)
		}
	})
	t.Run("无禁转badge返回空（折扣class不误抓）", func(t *testing.T) {
		if got := keepfrdsH1Badge(keepfrdsH1NoBadge); got != "" {
			t.Errorf("无 badge 应返回空, got %q", got)
		}
	})
	t.Run("双引号class兼容", func(t *testing.T) {
		h1 := `<h1 id="top">T <b>[<font class="recommended">禁转</font>]</b></h1>`
		if got := keepfrdsH1Badge(h1); got != "<b>[禁转]</b>" {
			t.Errorf("双引号形态 badge = %q", got)
		}
	})
	t.Run("h1内嵌svg跨行命中", func(t *testing.T) {
		h1 := "<h1 id=\"top\">T&nbsp;<span title=\"通过\"><svg\nviewBox='0 0 1 1'></svg></span>\n<b>[<font class='recommended'>禁转</font>]</b></h1>"
		if got := keepfrdsH1Badge(h1); got != "<b>[禁转]</b>" {
			t.Errorf("跨行形态 badge = %q", got)
		}
	})
	t.Run("h1外区域recommended不命中", func(t *testing.T) {
		page := `<h1 id="top">Plain Title</h1><div><b>[<font class='recommended'>禁转</font>]</b></div>`
		if got := keepfrdsH1Badge(page); got != "" {
			t.Errorf("h1 外 recommended 不应命中, got %q", got)
		}
	})
	t.Run("无h1返回空", func(t *testing.T) {
		if got := keepfrdsH1Badge(`<div>no h1</div>`); got != "" {
			t.Errorf("无 h1 应返回空, got %q", got)
		}
	})
}

// 集成编排：h1 命中即判定——不触网络（零值 adapter 无 doer 也不 panic，
// 证明详情页优先路径未走列表抓取）。分类器两态语义锚在
// internal/metadata（h1_badge_classify_test.go——adapter 测试态 import
// metadata 成环）。
func TestKeepfrdsH1Badge_EnrichPriority(t *testing.T) {
	a := &NexusPHPAdapter{}
	detail := &model.TorrentDetail{}
	a.enrichKeepfrdsTransferFlags(context.Background(),
		&model.SiteConfig{Domain: "pt.keepfrds.com"}, "2782403", detail, keepfrdsH1Real)
	if detail.RawListRow != "<b>[禁转]</b>" {
		t.Errorf("RawListRow = %q, want <b>[禁转]</b>（h1 优先·未走列表）", detail.RawListRow)
	}
}
