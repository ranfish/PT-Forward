package adapter

import (
	"strings"
	"testing"
)

// §59.466: 皇后新版详情页 MusicMeta 结构化锚——真实样本片段内嵌
// （2026 古典 Web 种 Giardini: 6 Sonatas——含 tooltip JS 噪声防御场景）。
func TestExtractOpenCD_MusicMeta(t *testing.T) {
	// 字段行区（含艺术家名 tooltip data-options JS——jQuery('<div></div>') 的 > 断裂场景）
	fieldSeg := `8);" /></div>
</td>
<td class="content" colspan="2" style="padding:0px;overflow:hidden;">
<table border="0" class="resource" width="100%">
<tr>
<td class="rowtitle" style="width:30%;">专辑名称：</td><td style="width:70%;" title="Giardini: 6 Sonatas for Violin & Continuo, Op. 1">Giardini: 6 Sonatas for Violin & Continuo, Op. 1</td>
</tr>
<tr>
<td class="rowtitle">艺术家名：</td><td><a href="javascript:void(0);" class="easyui-tooltip nav-item" data-options="content: jQuery('<div></div>'),
hideEvent: 'none',
onShow: function(){
var t = jQuery(this);
t.tooltip('tip').focus().unbind().bind('blur',function(){
t.tooltip('hide');
});
},
onUpdate: function(cc){
cc.panel({
width: 330,
height: 'auto',
border: false,
href: '/wiki.php?cmd=get-artist-card&amp;name=I Solisti Ambrosiani'
});
}
">I Solisti Ambrosiani</a></td>
</tr>
<tr>
<td class="rowtitle">发行年份：</td><td>2026年</td>
</tr>
<tr>
<td class="rowtitle">大小：</td><td>1.57 GB</td>
</tr>
<tr>
<td class="rowtitle">文件数：</td><td>9 个文件</td>
</tr>
<tr>
<td class="rowtitle">类型：</td><td>音乐(Music)</td>
</tr>
<tr>
<td class="rowtitle">类别：</td><td>古典(Classical)</td>
</tr>
<tr>
<td class="rowtitle">格式：</td><td>FLAC 分轨</td>
</tr>
<tr>
<td class="rowtitle">媒介：</td><td>Web</td>
</tr>
<tr>
<td class="rowtitle">制作组：</td><td></td>
</tr>
<tr>
<td class="rowtitle">来自：</td><td>转载</td>
</tr>
<tr>
<td class="rowtitle">标签列表：</td><td><i><a href='torrents.php?tags=2'>欧美</a></i> <i><a href='torrents.php?tags=9'>古典(Classical)</a></i></td>
</tr>
<tr>
<td class="rowtitle">转载规则：</td><td style="color:red;">欢迎转载</td>
</tr>
</table>
</td>
<td class="content">
<b>试听：</b><br />
<div id="divaudition"></div>
</td>
</tr>
<tr>
<td class="rowtitle" msg="简介"><a href="javascript:void(0);" onclick="javascript: plugin_display(this);" shower="divdescr" icon="picdescr"><img class="minus" src="pic/trans.gif" alt="Show/Hide" id="picdescr" title="" /> 简介</a></td>
<td class="content" colspan="3">
<div id="divdescr">Publisher (Label): &nbsp;Tactus<br />
Catalog Number: None<br />
Released: 2026<br />
Genre: &nbsp;Classical<br />
Duration: &nbsp;01:13:18<br />
<br />
Orchestra: I Solisti Ambrosiani<br />
Composer: Felice Giardini (1716-1796)<br />
Performer: I Solisti Ambrosiani<br />
<br />
<a class="faqlink" href="https://www.hdtracks.com/#/album/68eec1ce8836203b998c628f">https://www.hdtracks.com/#/album/68eec1ce8836203b998c628f</a><br />
<br />
</div>
</td>
<td class="content">
<div id="divtracklist" style="overflow-x:auto;">
<b>曲目列表：</b><br />
Felice Giardini (1716 - 1796): 6 Sonatas for Violin &amp; Continuo, Op. 1:<br />
1 Giardini: Violin Sonata in G Major, Op. 1 No. 1 11:24<br />
2 Giardini: Violin Sonata in F Major, Op. 1 No. 2 12:48<br />
3 Giardini: Violin Sonata in E Minor, Op. 1 No. 3 12:53<br />
4 Giardini: Violin Sonata in A Major, Op. 1 No. 4 13:19<br />
5 Giardini: Violin Sonata in E Major, Op. 1 No. 5 11:34<br />
6 Giardini: Violin Sonata in G Minor, Op. 1 No. 6 11:20
</div>
</td>
</tr>
<tr>
<td class="rowtitle" msg="其他版本">其它版本</td>
<td colspan="4">0 个其它版本
<div id="copies-list"></div>
</td>
</tr>
<tr style="display:;">
<td class="rowtitle" msg="链接地址">下载链接</td>
<td colspan="4"><b>此链接包含您的个人密钥信息，请勿提供给他人使用。</b><br/><a href="https://open.cd/download.php?id=189504&passkey=b7fa39bd0b203c8349f70cde54ebbda3">https`
	// 曲目区
	trackSeg := `id="divtracklist" style="overflow-x:auto;">
<b>曲目列表：</b><br />
Felice Giardini (1716 - 1796): 6 Sonatas for Violin &amp; Continuo, Op. 1:<br />
1 Giardini: Violin Sonata in G Major, Op. 1 No. 1 11:24<br />
2 Giardini: Violin Sonata in F Major, Op. 1 No. 2 12:48<br />
3 Giardini: Violin Sonata in E Minor, Op. 1 No. 3 12:53<br />
4 Giardini: Violin Sonata in A Major, Op. 1 No. 4 13:19<br />
5 Giardini: Violin Sonata in E Major, Op. 1 No. 5 11:34<br />
6 Giardini: Violin Sonata in G Minor, Op. 1 No. 6 11:20
</div>
</td>
</tr>
<tr>
<td class="rowtitle" msg="其他版本">其它版本</td>
<td colspan="4">0 个其它版本
<div id="copies-list"></div>
</td>
</tr>
<tr style="display:;">
<td class="rowtitle" msg="链接地址">下载链接</td>
<td colspan="4"><b>此链接包含您的个人密钥信息，请勿提供给他人使用。</b><br/><a href="https://open.cd/download.php?id=189504&passkey=b7fa39bd0b203c8349f70cde54ebbda3">https://open.cd/downloa`
	html := fieldSeg + trackSeg

	detail := extractOpenCDDetail(html)
	if detail.Music == nil {
		t.Fatal("MusicMeta 应产出（专辑名称/格式在场）")
	}
	mm := detail.Music
	if mm.Album != "Giardini: 6 Sonatas for Violin & Continuo, Op. 1" {
		t.Errorf("Album: %q", mm.Album)
	}
	if mm.Artist != "I Solisti Ambrosiani" {
		t.Errorf("Artist: %q（tooltip JS 噪声应剥离）", mm.Artist)
	}
	if mm.Format != "FLAC" || mm.AudioMode != "分轨" {
		t.Errorf("Format/AudioMode: %q/%q（FLAC 分轨拆解）", mm.Format, mm.AudioMode)
	}
	if mm.Media != "Web" {
		t.Errorf("Media: %q", mm.Media)
	}
	if mm.Year != "2026" {
		t.Errorf("Year: %q（2026年剥年）", mm.Year)
	}
	if mm.Genre != "古典(Classical)" {
		t.Errorf("Genre: %q", mm.Genre)
	}
	if len(mm.Tags) != 2 || mm.Tags[0] != "欧美" || mm.Tags[1] != "古典(Classical)" {
		t.Errorf("Tags: %v（双维 Split）", mm.Tags)
	}
	if mm.ShareRule != "欢迎转载" {
		t.Errorf("ShareRule: %q", mm.ShareRule)
	}
	if mm.Origin != "转载" {
		t.Errorf("Origin: %q", mm.Origin)
	}
	if !strings.Contains(mm.TrackList, "Sonata") {
		t.Errorf("TrackList 独立通道: %.60q", mm.TrackList)
	}
}
