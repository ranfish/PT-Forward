package adapter

import (
	"strings"
	"testing"
)

// §59.466 附: 海豚（DIC·音乐版 Gazelle）MusicMeta 结构化锚——tid=9100
// 真实 JSON 全文内嵌（2026-11-05 cookie 通道实测·Arashi BRAVE 单曲 CD）。
// 覆盖：musicInfo 7 角色数组/remaster 族/logScore 直读/fileList 清洗/
// filePath 标题/枚举翻译/整分轨推断。
func TestParseGazelleTorrentDetail_Music(t *testing.T) {
	body := `{"status":"success","response":{"group":{"wikiBody":"&#30331;&#37682;&#24773;&#22577;<br \/>\r\nCD (2019\/9\/11)<br \/>\r\n&#12487;&#12451;&#12473;&#12463;&#26522;&#25968;: 1<br \/>\r\n&#12501;&#12457;&#12540;&#12510;&#12483;&#12488;&#65306; &#12471;&#12531;&#12464;&#12523;, &#12510;&#12461;&#12471;<br \/>\r\n&#12524;&#12540;&#12505;&#12523;: &#12472;&#12455;&#12452;&#12539;&#12473;&#12488;&#12540;&#12512;<br \/>\r\n&#21454;&#37682;&#26178;&#38291;: 9 &#20998;<br \/>\r\nASIN: B07VSXM4GP<br \/>\r\nJAN&#65306; 4580117628466<br \/>\r\n&#20182;&#12398;&#12456;&#12487;&#12451;&#12471;&#12519;&#12531;&#65306; CD<br \/>\r\n<br \/>\r\n<br \/>\r\n&#23888; &#12362;&#12424;&#12381;1&#24180;&#25391;&#12426;&#12398;&#12491;&#12517;&#12540;&#12471;&#12531;&#12464;&#12523;&#30330;&#22770;&#27770;&#23450;!<br \/>\r\n<br \/>\r\n&ldquo;&#26085;&#26412;&#12486;&#12524;&#12499;&#31995;&#12521;&#12464;&#12499;&#12540;2019 &#12452;&#12513;&#12540;&#12472;&#12477;&#12531;&#12464;&ldquo;&#12392;&#12375;&#12390;&#12289;&#20170;&#12414;&#12391;&#12418;TV&#12391;&#24230;&#12293;&#12458;&#12531;&#12456;&#12450;&#12539;&#25259;&#38706;&#12373;&#12428;&#12390;&#12365;&#12383;&#12300;BRAVE&#12301;&#12434;&#12289;&#23888;&#36890;&#31639;57&#26522;&#30446;&#12398;&#12491;&#12517;&#12540;&#12471;&#12531;&#12464;&#12523;&#12392;&#12375;&#12390;&#12522;&#12522;&#12540;&#12473;&#12290;<br \/>\r\n&#23888;&#12392;&#12375;&#12390;&#12399;&#29645;&#12375;&#12367;&#12289;&#30007;&#12387;&#12413;&#12373;&#12539;&#38596;&#12293;&#12375;&#12373;&#12364;&#20840;&#38754;&#12395;&#20986;&#12383;&#12513;&#12525;&#12487;&#12451;&#12392;&#27468;&#35422;&#12398;&#20013;&#12395;&#12418;&#12289;&#12469;&#12499;&#12391;&#12399;&#31361;&#12365;&#25244;&#12369;&#12383;&#29245;&#24555;&#24863;&#12364;&#12411;&#12392;&#12400;&#12375;&#12427;&#27005;&#26354;&#12391;&#12289;&#12471;&#12531;&#12464;&#12523;&#12392;&#12375;&#12390;&#12399;2016&#24180;&#30330;&#22770;&#12398;&#12300;Daylight&#12301;&#20197;&#26469;&#20037;&#12293;&#12395;&#27387;&#20117;&#12364;Rap&#12434;&#26360;&#12365;&#19979;&#12429;&#12375;&#12383;&#12371;&#12392;&#12391;&#12418;&#22823;&#12365;&#12394;&#35441;&#38988;&#12434;&#21628;&#12435;&#12384;&#20170;&#20316;&#12290;<br \/>\r\n&#20170;&#22238;&#12398;&#12479;&#12452;&#12450;&#12483;&#12503;&#12394;&#12425;&#12391;&#12399;&#12398;&#12289;&ldquo;&#23888;&times;&#12521;&#12464;&#12499;&#12540;&rdquo;&#12398;&#12467;&#12521;&#12508;&#12364;&#32340;&#12426;&#25104;&#12377;&#27005;&#26354;&#12300;BRAVE&#12301;&#12391;&#12289;9&#26376;20&#26085;(&#37329;)&#12363;&#12425;&#38283;&#24149;&#12377;&#12427;&#12521;&#12464;&#12499;&#12540;&#12527;&#12540;&#12523;&#12489;&#12459;&#12483;&#12503;&#12434;&#12289;&#36984;&#25163;&#12434;&#12289;&#12381;&#12375;&#12390;&#35222;&#32884;&#32773;&#12434;&#12289;&#27005;&#26354;&#38754;&#12363;&#12425;&#30427;&#12426;&#19978;&#12370;&#12427;&#12290;<br \/>\r\n<br \/>\r\n<br \/>\r\n&#26354;&#30446;&#12522;&#12473;&#12488;<br \/>\r\n&#12487;&#12451;&#12473;&#12463;&#65306;1<br \/>\r\n&nbsp;&nbsp;1. BRAVE<br \/>\r\n&nbsp;&nbsp;2. BRAVE (&#12458;&#12522;&#12472;&#12490;&#12523;&#12539;&#12459;&#12521;&#12458;&#12465;)","wikiImage":"https:\/\/i.loli.net\/2020\/01\/17\/qO4JUMnah13rdKS.jpg","id":7868,"name":"Brave","year":2019,"recordLabel":"","catalogueNumber":"","releaseType":9,"categoryId":1,"categoryName":"Music","time":"2020-01-17 18:16:24","vanityHouse":false,"isBookmarked":false,"musicInfo":{"composers":[],"dj":[],"artists":[{"id":8671,"name":"Arashi (\u5d50)"}],"with":[],"conductor":[],"remixedBy":[],"producer":[]},"tags":["japanese"]},"torrent":{"id":9100,"infoHash":"FEA9342E5EA0D99EE57DB97A2CF55CC87CDDBD4A","media":"CD","format":"FLAC","encoding":"Lossless","remastered":true,"remasterYear":2019,"remasterTitle":"","remasterRecordLabel":"","remasterCatalogueNumber":"JACA-5810","scene":false,"hasLog":true,"hasCue":true,"logScore":100,"logChecksum":1,"logCount":1,"fileCount":6,"size":76993459,"seeders":2,"leechers":0,"snatched":11,"freeTorrent":false,"reported":false,"time":"2020-01-17 18:16:24","description":"","fileList":"01. BRAVE.flac{{{38485889}}}|||02. BRAVE (&#12458;&#12522;&#12472;&#12490;&#12523;&#12539;&#12459;&#12521;&#12458;&#12465;).flac{{{38394908}}}|||BRAVE.cue{{{1098}}}|||Cover.jpg{{{105806}}}|||&#23888; - BRAVE.log{{{5598}}}|||&#23888; - BRAVE.m3u8{{{160}}}","filePath":"&#23888; - BRAVE (2019) [CD] [FLAC]","userId":1226,"username":"lintoth"}}}`
	detail, err := parseGazelleTorrentDetail([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	// Title=filePath（站方文件夹规范形态·HTML unescape）
	if !strings.Contains(detail.Title, "嵐 - BRAVE (2019) [CD] [FLAC]") {
		t.Errorf("Title: %q（应 filePath 优先）", detail.Title)
	}
	if detail.Music == nil {
		t.Fatal("MusicMeta 应产出（musicInfo 在场）")
	}
	mm := detail.Music
	if mm.Artist != "Arashi (嵐)" {
		t.Errorf("Artist: %q", mm.Artist)
	}
	if mm.Album != "Brave" {
		t.Errorf("Album: %q", mm.Album)
	}
	if mm.Format != "FLAC" || mm.Media != "CD" {
		t.Errorf("Format/Media: %q/%q", mm.Format, mm.Media)
	}
	if mm.ReleaseType != "Single" {
		t.Errorf("ReleaseType: %q（枚举 9→Single）", mm.ReleaseType)
	}
	if mm.Year != "2019" {
		t.Errorf("Year: %q（remasterYear 优先）", mm.Year)
	}
	if mm.LogScore != 100 || !mm.HasLog || !mm.HasCue {
		t.Errorf("Log 族: score=%d hasLog=%v hasCue=%v（API 直读）", mm.LogScore, mm.HasLog, mm.HasCue)
	}
	if !strings.Contains(mm.TrackList, "01. BRAVE.flac") {
		t.Errorf("TrackList: %q", mm.TrackList)
	}
	// fileList 清洗：cue/log/jpg/m3u8 非音频应过滤
	for _, bad := range []string{".cue", ".log", ".jpg", ".m3u8"} {
		if strings.Contains(mm.TrackList, bad) {
			t.Errorf("TrackList 混入非音频: %s", bad)
		}
	}
	if mm.AudioMode != "分轨" {
		t.Errorf("AudioMode: %q（2 音频文件→分轨）", mm.AudioMode)
	}
	if len(detail.Tags) != 1 || detail.Tags[0] != "japanese" {
		t.Errorf("Tags: %v", detail.Tags)
	}
	if detail.InfoHash != strings.ToLower(detail.InfoHash) {
		t.Errorf("InfoHash 应小写")
	}
	// wikiBody 兜底 Description（版本级空）
	if !strings.Contains(detail.Description, "登録情報") && !strings.Contains(detail.Description, "CD") {
		t.Errorf("Description wikiBody 兜底: %.60q", detail.Description)
	}
}

// 海豹（电影版）回归锚：musicInfo 不在场=零 MusicMeta+年份拼副标保持。
func TestParseGazelleTorrentDetail_MovieNoMusic(t *testing.T) {
	body := `{"status":"success","response":{
		"group":{"name":"Send Help","year":2024,"categoryName":"Movies","conver":"https://x/c.jpg","tags":["action"]},
		"torrent":{"infoHash":"ABCDEF","size":100,"codec":"x265","processing":"Untouched","media":"","format":""}
	}}`
	detail, err := parseGazelleTorrentDetail([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if detail.Music != nil {
		t.Error("电影版 Music 应 nil（musicInfo 不在场）")
	}
	if detail.Title != "Send Help" {
		t.Errorf("Title: %q（电影版保持组名）", detail.Title)
	}
	if !strings.Contains(detail.Subtitle, "2024") {
		t.Errorf("Subtitle 年份拼接保持: %q", detail.Subtitle)
	}
}

// §59.466 附: HTML 转义场景锚——group.name 带 &#…; 形态（tid=12603 实证：
// "北半球有欧得洋" 转义原文）——Album/Artist/Title 三处 unescape。
func TestParseGazelleTorrentDetail_MusicHTMLEscape(t *testing.T) {
	body := `{"status":"success","response":{
		"group":{"name":"&#21271;&#21322;&#29699;&#26377;&#27431;&#24471;&#27915;","year":2004,"releaseType":1,
			"musicInfo":{"artists":[{"id":1,"name":"&#27431;&#24471;&#27915; (Ocean Ou)"}]}},
		"torrent":{"infoHash":"AB","size":1,"media":"CD","format":"FLAC","logScore":100,"hasLog":true,
			"fileList":"01. a.flac{{{100}}}|||a.log{{{1}}}"}
	}}`
	detail, err := parseGazelleTorrentDetail([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if detail.Music == nil {
		t.Fatal("Music 应产出")
	}
	if detail.Music.Album != "北半球有欧得洋" {
		t.Errorf("Album unescape: %q", detail.Music.Album)
	}
	if detail.Music.Artist != "欧得洋 (Ocean Ou)" {
		t.Errorf("Artist unescape: %q", detail.Music.Artist)
	}
	if detail.Title != "北半球有欧得洋" {
		t.Errorf("Title fallback 组名 unescape: %q", detail.Title)
	}
	// log 非音频过滤——TrackList 仅音频
	if detail.Music.TrackList != "01. a.flac" {
		t.Errorf("TrackList: %q", detail.Music.TrackList)
	}
	if detail.Music.AudioMode != "整轨" {
		t.Errorf("AudioMode: %q（单音频→整轨）", detail.Music.AudioMode)
	}
}
