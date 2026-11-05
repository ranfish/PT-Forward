package adapter

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.469 附五: 乌托邦音乐特化锚——双形态（官组三段/转载四段）+
// 字段行双引号/单引号形态+AudioMode 繁体归一+保种区尾巴剥除。
// 样本=2026-11-05 tid=17109（官组）/tid=10072（转载）实测 HTML 精简。

const utopiaOfficialHTML = `
<h1 id="top">陈慧娴 - 归来吧 1992 FLAC 分轨 - ZHENGLIFAN@Musopia&nbsp;&nbsp;&nbsp; [免费]</h1>
<tr><td class="rowhead nowrap" valign="top" align="right">副标题</td><td class="rowfollow" valign="top" align="left">内嵌专辑封面</td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">标签</td><td class="rowfollow" valign="top" align="left"><span style="background-color:#8F77B5" title="">首发</span><span style="background-color:#0000ff" title="">官方</span><span title="">港台</span><span title="">大陆</span><span style="color:#FF00FF" title="">流行(Pop)</span></td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">语言</td><td class="rowfollow" valign="top" align="left">华语</td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">基本信息</td><td class="rowfollow" valign="top" align="left"><b><b>大小：</b></b>315.10 MB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;HQ Audio&nbsp;&nbsp;&nbsp;<b>音频编码: </b>FLAC&nbsp;&nbsp;&nbsp;<b>制作组: </b>Musopia</td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">抓轨 Log（共 1 碟）</td><td class="rowfollow" align="left" valign="top"><details><summary>Log CD 1</summary><br /><div class="codetop">代码</div><div class="codemain"><pre><code> Exact Audio Copy V1.8 from 15. July 2024
EAC extraction logfile </code></pre></div></details><div style="margin:5px 0 10px 0;">陈慧娴 - 归来吧.log　<a href="downloadriplog.php?id=1420">⬇ 下载原始文件</a></div></td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">Log score</td><td class="rowfollow" valign="top" align="left">100</td></tr>
<div id="kdescr">免责声明 本站所有资源均为网友上传。音乐乌托邦原抓组 ZHENGLIFAN 作品，欢迎转载各大PT，请在帖子顶部注明出处及作者，并保留后缀。</div>
`

const utopiaTransferHTML = `
<h1 id="top">7immy OST - INCON7ENIBLE 2007 - FLAC 分軌 - Redacted🌱 保种区 ×2（已保护 31 小时）&nbsp;&nbsp;&nbsp; [免费]</h1>
<tr><td class="rowhead nowrap" valign="top" align="right">副标题</td><td class="rowfollow" valign="top" align="left">FLAC / Lossless / CD / Log (100%) / Cue</td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">标签</td><td class="rowfollow" valign="top" align="left"><span title="">Redacted</span><span title="">摇滚(rock)</span></td></tr>
<tr><td class='rowhead'>来源站点</td><td class='rowfollow'><a href='https://redacted.sh/torrents.php' target='_blank' rel='noopener'>redacted</a>　<span style='color:#8b0000'>转载自：</span><a href='https://redacted.sh/torrents.php?id=2837293&amp;torrentid=6394198#torrent6394198' target='_blank'>https://redacted.sh/torrents.php?id=2837293&amp;torrentid=6394198#torrent6394198</a></td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">基本信息</td><td class="rowfollow" valign="top" align="left"><b><b>大小：</b></b>249.69 MB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;HQ Audio&nbsp;&nbsp;&nbsp;<b>媒介: </b>CD&nbsp;&nbsp;&nbsp;<b>音频编码: </b>FLAC</td></tr>
<tr><td class="rowhead nowrap" valign="top" align="right">Log score</td><td class="rowfollow" valign="top" align="left">100</td></tr>
<div id="kdescr">免责声明 本站所有资源均为网友上传。</div>
`

func TestUtopiaOfficialMusicMeta(t *testing.T) {
	mm := buildUtopiaMusicMeta(utopiaOfficialHTML,
		"陈慧娴 - 归来吧 1992 FLAC 分轨 - ZHENGLIFAN@Musopia&nbsp;&nbsp;&nbsp; [免费]")
	if mm == nil {
		t.Fatal("官组种应产出 MusicMeta")
	}
	// 标题三段解析（官组形态：专辑+年+格式+模式粘连中段）
	assertEq := func(field, got, want string) {
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	assertEq("Artist", mm.Artist, "陈慧娴")
	assertEq("Album", mm.Album, "归来吧")
	assertEq("Year", mm.Year, "1992")
	assertEq("Format", mm.Format, "FLAC")
	assertEq("AudioMode", mm.AudioMode, "分轨")
	// 字段行
	if mm.LogScore != 100 {
		t.Errorf("LogScore = %d, want 100", mm.LogScore)
	}
	if !mm.HasLog {
		t.Error("HasLog 应 true（Log score 行在场）")
	}
	if mm.ReleaseType != "HQ Audio" {
		t.Errorf("ReleaseType = %q, want HQ Audio", mm.ReleaseType)
	}
	// 标签多维（首发/官方/港台/大陆/流行(Pop)）+ Genre 括号维
	if len(mm.Tags) != 5 {
		t.Errorf("Tags = %v, want 5 项", mm.Tags)
	}
	if mm.Genre != "流行(Pop)" {
		t.Errorf("Genre = %q, want 流行(Pop)", mm.Genre)
	}
	// 官组种无来源行/无媒介/副标题自由文本无 Cue
	if mm.Origin != "" {
		t.Errorf("官组种 Origin 应空, got %q", mm.Origin)
	}
	if mm.Media != "" {
		t.Errorf("官组种 Media 应空（可选字段不在场）, got %q", mm.Media)
	}
	if mm.HasCue {
		t.Error("官组种副标题自由文本不应 HasCue")
	}
	// ShareRule 简介声明
	if mm.ShareRule != "欢迎转载" {
		t.Errorf("ShareRule = %q, want 欢迎转载", mm.ShareRule)
	}
	// LogText 抓轨区
	if !strings.Contains(mm.LogText, "Exact Audio Copy") {
		t.Errorf("LogText 应含 EAC 头, got %.60s", mm.LogText)
	}
}

func TestUtopiaTransferMusicMeta(t *testing.T) {
	mm := buildUtopiaMusicMeta(utopiaTransferHTML,
		"7immy OST - INCON7ENIBLE 2007 - FLAC 分軌 - Redacted🌱 保种区 ×2（已保护 31 小时）&nbsp;&nbsp;&nbsp; [免费]")
	if mm == nil {
		t.Fatal("转载种应产出 MusicMeta")
	}
	assertEq := func(field, got, want string) {
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	// 标题四段解析（转载形态：专辑/格式分离+保种区尾巴剥+繁体分軌归一）
	assertEq("Artist", mm.Artist, "7immy OST")
	assertEq("Album", mm.Album, "INCON7ENIBLE")
	assertEq("Year", mm.Year, "2007")
	assertEq("Format", mm.Format, "FLAC")
	assertEq("AudioMode", mm.AudioMode, "分轨") // 分軌→分轨（繁体归一）
	// Origin 转载自 URL（单引号行+&amp; unescape）
	assertEq("Origin", mm.Origin,
		"https://redacted.sh/torrents.php?id=2837293&torrentid=6394198#torrent6394198")
	// 媒介（转载种基本信息含媒介:）
	assertEq("Media", mm.Media, "CD")
	// HasCue（副标题规格链含 Cue）
	if !mm.HasCue {
		t.Error("转载种副标题规格链应 HasCue")
	}
	// 标签+Genre
	if len(mm.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 项", mm.Tags)
	}
	if mm.Genre != "摇滚(rock)" {
		t.Errorf("Genre = %q, want 摇滚(rock)", mm.Genre)
	}
	// 无声明文本
	if mm.ShareRule != "" {
		t.Errorf("转载种无声明 ShareRule 应空, got %q", mm.ShareRule)
	}
}

// 域判定锚：musopia 命中/其它站零开销。
func TestUtopiaDomain(t *testing.T) {
	for _, d := range []string{"www.musopia.vip", "musopia.vip", "MUSOPIA.VIP"} {
		if !isUtopiaDomain(d) {
			t.Errorf("%s 应判乌托邦域", d)
		}
	}
	for _, d := range []string{"open.cd", "dicmusic.com", "www.hdtime.org"} {
		if isUtopiaDomain(d) {
			t.Errorf("%s 不应判乌托邦域", d)
		}
	}
}

// 模式词后尾缀词形态（Enrique 种 tid=3168 实证："…FLAC 分轨 tulipa"——
// TrimSuffix 剥不掉·任意位置删词修后 Album 不残留模式词）。
func TestUtopiaTitleTrailingWord(t *testing.T) {
	mm := &model.MusicMeta{}
	parseUtopiaMusicTitle("Enrique Ugarte - Valse Musette 手风琴圆舞曲 1993 FLAC 分轨 tulipa - ZHENGLIFAN@Musopia", mm)
	if mm.AudioMode != "分轨" {
		t.Errorf("AudioMode = %q, want 分轨", mm.AudioMode)
	}
	if mm.Album != "Valse Musette 手风琴圆舞曲 tulipa" {
		t.Errorf("Album = %q, want Valse Musette 手风琴圆舞曲 tulipa（尾缀词保真保留·模式词不残留）", mm.Album)
	}
	if mm.Year != "1993" || mm.Format != "FLAC" || mm.Artist != "Enrique Ugarte" {
		t.Errorf("Artist/Year/Format = %q/%q/%q", mm.Artist, mm.Year, mm.Format)
	}
}

// 种名内部含 " - " 的 join 残留（许景淳种 tid=11617 实证——Album 尾
// " -" 分隔符残留剥除·内容保真不变）。
func TestUtopiaTitleInnerDash(t *testing.T) {
	mm := &model.MusicMeta{}
	parseUtopiaMusicTitle("许景淳 - 路长情更长 - FLAC 分轨 - ZHENGLIFAN@Musopia", mm)
	if mm.Album != "路长情更长" {
		t.Errorf("Album = %q, want 路长情更长（尾 - 剥）", mm.Album)
	}
}

// 真实形态锚（许景淳种 tid=11617 h1 实测）：两段+年份格式连字符粘连
// （"1994-FLAC"无空格连字符——split " - " 只出 2 段）。
func TestUtopiaTitleRealForm(t *testing.T) {
	mm := &model.MusicMeta{}
	parseUtopiaMusicTitle("许景淳 - 路长情更长 1994-FLAC 分轨    [免费]", mm)
	wants := map[string]string{
		"Artist": "许景淳", "Album": "路长情更长", "Year": "1994",
		"Format": "FLAC", "AudioMode": "分轨",
	}
	gots := map[string]string{"Artist": mm.Artist, "Album": mm.Album, "Year": mm.Year, "Format": mm.Format, "AudioMode": mm.AudioMode}
	for k, w := range wants {
		if gots[k] != w {
			t.Errorf("%s = %q, want %q", k, gots[k], w)
		}
	}
}
