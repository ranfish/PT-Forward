package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.462 附七: 建组字段三层兜底测试锚——DryRun 实证 meta 派生列
// 填充率低（314 颗 movie 仅 2 颗 english_title 非空）而 PTGen JSON 全在。
// 真实形态锚：Death Ship 1980（13城）——列全空+PTGen 全在的典型瘦行。
func TestGpwPublishInject_PTGenFallback(t *testing.T) {
	ptgenJSON := `{"chinese_title":"死亡船","foreign_title":"","translated_titles":"Death Ship","year":"1980",` +
		`"genre":["悬疑","惊悚","恐怖","冒险"],"introduction":"简介","imdb_url":"https://www.imdb.com/title/tt0080603/","imdb_id":"tt0080603",` +
		`"director":["艾尔文·拉考夫 Alvin Rakoff"],"cast":[{"name":"乔治·肯尼迪 George Kennedy"}]}`
	meta := &model.TorrentMetadata{
		// 派生列全空（DryRun 实证形态）——PTGen JSON 全在
		Category:         "category.movie",
		PTGenSourceJSON: ptgenJSON,
	}
	tp := titleparser.ParseTitleTech("Death Ship 1980 2160p UHD Blu-ray HEVC DTS-HD MA 5.1-B0MBARDiERS")

	extra := map[string]string{}
	var artists []model.PublishArtist
	var subs []string
	cfg := &model.PublishFormConfig{FormFields: map[string]string{
		"processing": "processing", "processing_other": "processing_other",
	}}
	form := map[string]string{}
	gpwPublishInject(cfg, form, meta, tp, 72528179936, extra, &artists, &subs)

	if extra["gpw_name"] != "Death Ship" {
		t.Errorf("gpw_name: %q（tp.MainTitle 兜底）", extra["gpw_name"])
	}
	if extra["gpw_subname"] != "死亡船" {
		t.Errorf("gpw_subname: %q（PTGen chinese_title 兜底）", extra["gpw_subname"])
	}
	if extra["gpw_year"] != "1980" {
		t.Errorf("gpw_year: %q", extra["gpw_year"])
	}
	if extra["gpw_tags"] != "mystery,thriller,horror,adventure" {
		t.Errorf("gpw_tags: %q（PTGen genre 中文→英文映射）", extra["gpw_tags"])
	}
	if extra["gpw_imdb"] == "" {
		t.Errorf("gpw_imdb 空（PTGen imdb_url 兜底）")
	}
	if extra["gpw_desc"] != "简介" {
		t.Errorf("gpw_desc: %q（PTGen introduction 兜底）", extra["gpw_desc"])
	}
	if extra["gpw_releasetype"] != "1" {
		t.Errorf("gpw_releasetype: %q", extra["gpw_releasetype"])
	}
	// 剧组四件套（SplitPersonName 拆分）
	if len(artists) == 0 || artists[0].Name != "Alvin Rakoff" || artists[0].SubName != "艾尔文·拉考夫" || artists[0].Role != "1" {
		t.Errorf("artists[0]: %+v（director Role=1）", artists[0])
	}
	if len(artists) < 2 || artists[1].Role != "6" {
		t.Errorf("artists cast Role=6 缺: n=%d", len(artists))
	}
	// 碟规格（67.5G → BD100——MediumCanonicalOf 标题启发式判原盘族）
	if form["processing"] != "Untouched" {
		t.Errorf("processing: %q（应 Untouched）", form["processing"])
	}
	if form["processing_other"] != "BD100" {
		t.Errorf("processing_other: %q（67.5G→BD100）", form["processing_other"])
	}
}
