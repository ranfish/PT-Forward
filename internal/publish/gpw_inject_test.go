package publish

import (
	"strings"
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

// §59.462 附八: 内容拦截分支锚——纯电影站放行族/拦截族/animation S/E 特判。
func TestGpwOnlyMovies(t *testing.T) {
	cases := []struct {
		cat     string
		title   string
		blocked bool
		desc    string
	}{
		{"category.movie", "Test 2020", false, "电影放行"},
		{"category.concert", "Live 2019", false, "现场演出放行"},
		{"category.short_film", "Short 2018", false, "短片放行"},
		{"category.animation", "千与千寻 2001", false, "动画电影放行（无 S/E）"},
		{"category.animation", "某动画 S01 2020", true, "动画剧拦截（S/E 信号）"},
		{"category.tv_series", "Show S01", true, "剧集拦截"},
		{"category.variety", "综艺 2023", true, "综艺拦截"},
		{"", "Unknown 2020", true, "空类目拦截（宁拦勿错）"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			blocked, _ := gpwOnlyMovies(&model.TorrentMetadata{Category: c.cat, Title: c.title})
			if blocked != c.blocked {
				t.Errorf("cat=%q title=%q blocked=%v want %v", c.cat, c.title, blocked, c.blocked)
			}
		})
	}
}

// §59.462 附八: 碟规格边界值锚——size 门（<20GB 不投）+四档边界。
func TestGpwDiscSpec(t *testing.T) {
	gb := func(g float64) int64 { return int64(g * (1 << 30)) }
	cases := []struct {
		size int64
		want string
		desc string
	}{
		{0, "", "size=0（快照 miss）不投"},
		{gb(1), "", "1GB（误判原盘防御）不投"},
		{gb(19.9), "", "19.9GB 门内不投"},
		{gb(20), "BD25", "20GB 门上（最小合法原盘）BD25"},
		{gb(23), "BD25", "23GB（BD25 实体）BD25"},
		{gb(25.5), "BD25", "25.5 边界 BD25"},
		{gb(26), "BD50", "26GB BD50"},
		{gb(50), "BD50", "50GB BD50"},
		{gb(51), "BD50", "51 边界 BD50"},
		{gb(52), "BD66", "52GB BD66"},
		{gb(66), "BD66", "66GB BD66"},
		{gb(67), "BD66", "67 边界 BD66"},
		{gb(68), "BD100", "68GB BD100"},
		{gb(100), "BD100", "100GB BD100"},
		{gb(120), "BD100", "超规格取最近档 BD100"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := gpwDiscSpec(c.size); got != c.want {
				t.Errorf("size=%dGB got=%q want=%q", c.size/(1<<30), got, c.want)
			}
		})
	}
}

// §59.462 附九: 截图三要件门锚——数量/格式/图床白名单（站规 2.2.1）。
func TestGpwScreenshotGate(t *testing.T) {
	png3 := []string{
		"https://img104.pixhost.to/abc/shot1.png",
		"https://ptpimg.me/shot2.png",
		"https://img.pterclub.net/shot3.png?x=1", // query 剥除后判
	}
	if b, r := gpwScreenshotGate(png3); b {
		t.Errorf("三要件全过应放行: %s", r)
	}
	// 数量门
	if b, _ := gpwScreenshotGate(png3[:2]); !b {
		t.Error("2 张应拦（<3）")
	}
	if b, _ := gpwScreenshotGate(nil); !b {
		t.Error("0 张应拦")
	}
	// 格式门（jpeg → 拦+重截指引）
	jpg := []string{"https://ptpimg.me/a.png", "https://ptpimg.me/b.png", "https://ptpimg.me/c.jpg"}
	if b, r := gpwScreenshotGate(jpg); !b || !strings.Contains(r, "PNG") {
		t.Errorf("jpeg 应拦+含指引: b=%v r=%q", b, r)
	}
	// 图床白名单门（非白名单域 → 拦）
	bad := []string{"https://img.example.com/a.png", "https://ptpimg.me/b.png", "https://ptpimg.me/c.png"}
	if b, r := gpwScreenshotGate(bad); !b || !strings.Contains(r, "白名单") {
		t.Errorf("非白名单域应拦: b=%v r=%q", b, r)
	}
	// 子域匹配（img104.pixhost.to ∈ pixhost.to 已在 png3 首项）
	// URL 不可解析
	ugly := []string{"not-a-url", "https://ptpimg.me/b.png", "https://ptpimg.me/c.png"}
	if b, _ := gpwScreenshotGate(ugly); !b {
		t.Error("URL 不可解析应拦")
	}
}
