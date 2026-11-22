package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.554 刀2 锚: TNode note 三件套——纯文本声明+IMDb/豆瓣链接行
// （全角冒号格式·站方发布页行格式定案）。空值矩阵各态独立跳行。
func TestTNodeNoteOf(t *testing.T) {
	meta := &model.TorrentMetadata{
		Statement: "[quote]\n原盘来自 Xxx 2020 BluRay-GRP，感谢制作！\n\n感谢以上所有素材提供者！\n[/quote]",
		IMDbURL:   "https://www.imdb.com/title/tt0342778",
		DoubanURL: "https://movie.douban.com/subject/1305224",
	}
	got := tnodeNoteOf(meta)
	want := "原盘来自 Xxx 2020 BluRay-GRP，感谢制作！\n\n感谢以上所有素材提供者！\nIMDb链接：https://www.imdb.com/title/tt0342778\n豆瓣链接：https://movie.douban.com/subject/1305224"
	if got != want {
		t.Fatalf("全值态:\ngot  %q\nwant %q", got, want)
	}

	// ① 声明空→仅链接两行
	if got := tnodeNoteOf(&model.TorrentMetadata{IMDbURL: "u", DoubanURL: "d"}); got != "IMDb链接：u\n豆瓣链接：d" {
		t.Fatalf("① 声明空: %q", got)
	}
	// ② 豆瓣缺→缺行
	if got := tnodeNoteOf(&model.TorrentMetadata{Statement: "声明", IMDbURL: "u"}); got != "声明\nIMDb链接：u" {
		t.Fatalf("② 豆瓣缺: %q", got)
	}
	// ③ 全空→空串（适配器 != "" 门不投）
	if got := tnodeNoteOf(&model.TorrentMetadata{}); got != "" {
		t.Fatalf("③ 全空: %q", got)
	}
	// ④ nil meta 防御
	if got := tnodeNoteOf(nil); got != "" {
		t.Fatalf("④ nil: %q", got)
	}
}

// §59.554 锚: BBCode 剥离——标签全去·内文换行保留·首尾 trim。
func TestStripBBCodeTags(t *testing.T) {
	if got := stripBBCodeTags("[quote]内文[/quote]"); got != "内文" {
		t.Errorf("quote 包裹: %q", got)
	}
	if got := stripBBCodeTags("  [b]粗体[/b] 普通文本 \n"); got != "粗体 普通文本" {
		t.Errorf("trim+换行: %q", got)
	}
	if got := stripBBCodeTags("无标签文本"); got != "无标签文本" {
		t.Errorf("无标签直通: %q", got)
	}
}
