package publish

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.485 刀2: 乌托邦音乐翻译层测试锚（九定案回归）。

func mmFixture() *model.MusicMeta {
	return &model.MusicMeta{
		Artist: "Thelonious Monk", Album: "Misterioso", Year: "1965",
		Format: "FLAC", AudioMode: "分軌", Media: "CD",
		Genre: "爵士(Jazz)", Tags: []string{"欧美"},
		LogScore: 100, HasLog: true, HasCue: true,
		Origin: "原创", ShareRule: "欢迎转载各大PT论坛",
	}
}

func metaFixture() *model.TorrentMetadata {
	return &model.TorrentMetadata{
		SiteName:  "皇后",
		Title:     "Thelonious Monk - Misterioso 1965-FLAC 分轨-Rosebudx@OpenCD",
		Poster:    "https://img3.pixhost.to/images/6996/x.jpg",
		SourceDescription: "[img]attachments/201303/a.jpg[/img]\n专辑介绍正文。\n曲目：\n01. Well, You Needn't\n\n******************** Score : 100 ********************\nExact Audio Copy V1.6 from 23. October 2020\nEAC extraction logfile",
	}
}

func TestUtopiaMusicName_FourSegments(t *testing.T) {
	// 决策点②: 有官组（Origin=原创）→ 四段式含组名段·简繁归一
	got := utopiaMusicName(metaFixture(), mmFixture(), "OpenCD")
	want := "Thelonious Monk - Misterioso 1965 - FLAC 分轨 - OpenCD"
	if got != want {
		t.Errorf("name 四段式: got=%q want=%q", got, want)
	}
}

func TestUtopiaMusicName_NoGroup(t *testing.T) {
	// 无官组（Origin=转载）→ 三段式（第四段省略——站方"未知出处可省"）
	mm := mmFixture()
	mm.Origin = "转载"
	got := utopiaMusicName(metaFixture(), mm, utopiaMusicGroupName(metaFixture(), mm))
	want := "Thelonious Monk - Misterioso 1965 - FLAC 分轨"
	if got != want {
		t.Errorf("name 三段式: got=%q want=%q", got, want)
	}
}

func TestUtopiaMusicName_AlbumFallback(t *testing.T) {
	// 决策点④: album 空（大合集老种）→ 皇后标题中段兜底剥年份
	mm := mmFixture()
	mm.Album = ""
	mm.Artist = "Various Artists"
	meta := &model.TorrentMetadata{
		Title: "Various Artists - Franklin mint records The 100 Greatest 1974 - FLAC 分軌",
	}
	got := utopiaMusicName(meta, mm, "")
	if !strings.Contains(got, "Franklin mint records The 100 Greatest") {
		t.Errorf("album 兜底: %q", got)
	}
	if strings.Contains(got, "1974 - FLAC") {
		t.Errorf("兜底段年份应剥除: %q", got)
	}
}

func TestUtopiaMusicSubtitle_SpecChain(t *testing.T) {
	// 决策点⑦: 规格链 FLAC / Lossless / CD / Log (100%) / Cue
	got := utopiaMusicSubtitle(mmFixture())
	want := "FLAC / Lossless / CD / Log (100%) / Cue"
	if got != want {
		t.Errorf("副标题规格链: got=%q want=%q", got, want)
	}
	// 缺项跳过：无 log 无 cue 无 media
	mm2 := &model.MusicMeta{Format: "FLAC"}
	if got := utopiaMusicSubtitle(mm2); got != "FLAC / Lossless" {
		t.Errorf("缺项跳过: %q", got)
	}
}

func TestUtopiaMusicLogTexts_SplitAndStrip(t *testing.T) {
	// 决策点③: 双 CD 拆分+Score 标注行剥除
	logText := "******************** Score : 100 ********************\n" +
		"Exact Audio Copy V1.6 from 23. October 2020\nEAC CD1 content\n" +
		"******************** Score : 100 ********************\n" +
		"Exact Audio Copy V1.6 from 23. October 2020\nEAC CD2 content"
	segs := utopiaMusicLogTexts(&model.MusicMeta{LogText: logText})
	if len(segs) != 2 {
		t.Fatalf("双 CD 拆分: got %d 段", len(segs))
	}
	for i, s := range segs {
		if strings.Contains(s, "Score") {
			t.Errorf("段 %d 残留 Score 标注行", i)
		}
		if !strings.Contains(s, "Exact Audio Copy") {
			t.Errorf("段 %d 缺 EAC 头", i)
		}
	}
	// 海豚源空 → nil
	if got := utopiaMusicLogTexts(&model.MusicMeta{}); got != nil {
		t.Errorf("空 LogText 应 nil: %v", got)
	}
}

func TestUtopiaMusicRipScore_Fallback(t *testing.T) {
	// LogScore 直通
	if got := utopiaMusicRipScore(mmFixture()); got != "100" {
		t.Errorf("LogScore 直通: %q", got)
	}
	// LogText Score 行兜底（皇后 log_score 字段 0/181 全空的补偿）
	mm := &model.MusicMeta{LogText: "******************** Score : 88 ********************\nExact Audio Copy..."}
	if got := utopiaMusicRipScore(mm); got != "88" {
		t.Errorf("Score 行兜底: %q", got)
	}
}

func TestUtopiaMusicDescr_Assemble(t *testing.T) {
	// 决策点④+③: 致谢头+rehost 封面首图+正文（剥 img 行+截 LOG 段）
	got := utopiaMusicDescr(metaFixture(), mmFixture())
	if !strings.HasPrefix(got, "[quote]转载皇后资源，感谢[/quote]") {
		t.Errorf("致谢头缺失: %q", got[:40])
	}
	if !strings.Contains(got, "[img]https://img3.pixhost.to/images/6996/x.jpg[/img]") {
		t.Errorf("封面首图缺失（poster 直用）")
	}
	if strings.Contains(got, "attachments/") {
		t.Errorf("正文残留相对路径 img 行")
	}
	if strings.Contains(got, "Exact Audio Copy") || strings.Contains(got, "EAC extraction") {
		t.Errorf("LOG 段未截断")
	}
	if !strings.Contains(got, "专辑介绍正文") {
		t.Errorf("正文丢失")
	}
}

func TestUtopiaMusicTags_Rules(t *testing.T) {
	// 决策点②: 来源站+地区+流派；铁律：禁转(1)/首发(2)/加速(47/44) 恒不勾
	tags := utopiaMusicTags(metaFixture(), mmFixture(), "OpenCD")
	joined := strings.Join(tags, ",")
	for _, want := range []string{"31", "5", "11"} { // OpenCD 来源/欧美地区/爵士流派
		if !strings.Contains(joined, want) {
			t.Errorf("tags 缺 %s: %v", want, tags)
		}
	}
	for _, banned := range []string{"1", "2", "47", "44"} {
		for _, v := range tags {
			if v == banned {
				t.Errorf("铁律违规：勾了 %s（禁转/首发/加速）", banned)
			}
		}
	}
	// 无官组 → 无来源 tag
	mm := mmFixture()
	mm.Origin = "转载"
	tags2 := utopiaMusicTags(metaFixture(), mm, utopiaMusicGroupName(metaFixture(), mm))
	for _, v := range tags2 {
		if v == "31" {
			t.Errorf("无官组打了来源 tag")
		}
	}
}

func TestUtopiaMusicR7_Matrix(t *testing.T) {
	// 皇后：ShareRule 禁转族拦/欢迎放
	if blocked, _ := utopiaMusicR7Blocked(&model.TorrentMetadata{SiteName: "皇后"},
		&model.MusicMeta{ShareRule: "未经允许不得转载"}); !blocked {
		t.Errorf("皇后禁转文本应拦")
	}
	if blocked, _ := utopiaMusicR7Blocked(&model.TorrentMetadata{SiteName: "皇后"},
		&model.MusicMeta{ShareRule: "欢迎转载各大PT"}); blocked {
		t.Errorf("皇后欢迎转载应放")
	}
	// 海豚：VH∨scene 拦·双 false 放（§59.485 刀1 标记）
	if blocked, _ := utopiaMusicR7Blocked(&model.TorrentMetadata{SiteName: "海豚"},
		&model.MusicMeta{IsVanityHouse: true}); !blocked {
		t.Errorf("海豚 VH 应拦")
	}
	if blocked, _ := utopiaMusicR7Blocked(&model.TorrentMetadata{SiteName: "海豚"},
		&model.MusicMeta{IsScene: true}); !blocked {
		t.Errorf("海豚 scene 应拦")
	}
	if blocked, _ := utopiaMusicR7Blocked(&model.TorrentMetadata{SiteName: "海豚"},
		&model.MusicMeta{}); blocked {
		t.Errorf("海豚转载种（双 false）应放")
	}
	// 乌托邦：放
	if blocked, _ := utopiaMusicR7Blocked(&model.TorrentMetadata{SiteName: "乌托邦"},
		&model.MusicMeta{}); blocked {
		t.Errorf("乌托邦应放")
	}
}

func TestUtopiaMusicFormValues(t *testing.T) {
	form := utopiaMusicFormValues(metaFixture(), mmFixture(), nil) // FLAC 直映射·镜像分支不进
	if form["type"] != "408" {
		t.Errorf("type 恒 408: %q", form["type"])
	}
	if form["medium_sel[4]"] != "8" { // CD
		t.Errorf("medium CD=8: %q", form["medium_sel[4]"])
	}
	if form["audiocodec_sel[4]"] != "1" { // FLAC
		t.Errorf("audiocodec FLAC=1: %q", form["audiocodec_sel[4]"])
	}
	if form["team_sel[4]"] != "5" { // 组名无识别通道 → Other（决策点①零冒认）
		t.Errorf("team Other=5: %q", form["team_sel[4]"])
	}
	if form["language_id"] != "42" { // 欧美
		t.Errorf("language 欧美=42: %q", form["language_id"])
	}
	if form["rip_score"] != "100" {
		t.Errorf("rip_score: %q", form["rip_score"])
	}
}

// benc 造最小合法 torrent（多文件·files 按字典序不敏感——fingerprint 解析器容错）。
func bencTorrent(paths []string) []byte {
	// info: files[{length,path[]}] 单/多文件统一 files 形态
	var fb string
	for _, p := range paths {
		fb += "d6:lengthi1024e4:pathl"
		for _, seg := range strings.Split(p, "/") {
			fb += fmt.Sprintf("%d:%s", len(seg), seg)
		}
		fb += "ee"
	}
	info := "d5:filesl" + fb + "e4:name4:teste" // files=list(l..e)·字典序 files<name
	t := "d8:announce18:https://test.com/a4:info" + info + "e"
	return []byte(t)
}

// §59.492 刀2: log 硬拦预检纯函数锚——四态（有 log/无 log/多 log/解析失败 fail-close）。
func TestUtopiaMusicLogCount(t *testing.T) {
	// 有 log（25174 真实形态：flac+cue+log）
	n, err := utopiaMusicLogCount(bencTorrent([]string{"a.flac", "a.cue", "a.log"}))
	if err != nil || n != 1 {
		t.Fatalf("has-log: n=%d err=%v want 1/nil", n, err)
	}
	// 无 log（朱孝天案形态）
	n, err = utopiaMusicLogCount(bencTorrent([]string{"a.flac", "a.cue"}))
	if err != nil || n != 0 {
		t.Fatalf("no-log: n=%d err=%v want 0/nil", n, err)
	}
	// 多 CD 多 log
	n, err = utopiaMusicLogCount(bencTorrent([]string{"CD1/a.log", "CD2/b.LOG", "CD2/b.flac"}))
	if err != nil || n != 2 {
		t.Fatalf("multi-log: n=%d err=%v want 2/nil（大小写不敏感）", n, err)
	}
	// 解析失败 fail-close 输入
	if _, err := utopiaMusicLogCount([]byte("not-bencode")); err == nil {
		t.Fatal("garbage: expect error (fail-close)")
	}
	if _, err := utopiaMusicLogCount(nil); err == nil {
		t.Fatal("empty: expect error")
	}
}

// §59.492 刀3 L2: 值集校验锚——三态（全过/单 miss/多 miss 含标签）。
func TestUtopiaMusicValueSetAudit(t *testing.T) {
	// 全过：25174 真实形态槽位
	form := map[string]string{
		"audiocodec_sel[4]": "1", // FLAC
		"medium_sel[4]":     "8", // CD
		"team_sel[4]":       "5", // Other
	}
	tags := []model.TagKV{{Key: "tags[4][]", Value: "5"}, {Key: "tags[4][]", Value: "16"}}
	if w := utopiaMusicValueSetAudit(form, tags); len(w) != 0 {
		t.Fatalf("all-pass: want 0 warnings, got %v", w)
	}
	// 单 miss：audiocodec 不在 19 值
	form["audiocodec_sel[4]"] = "99"
	w := utopiaMusicValueSetAudit(form, tags)
	if len(w) != 1 || !strings.Contains(w[0], "音频编码") {
		t.Fatalf("single-miss: got %v", w)
	}
	// 多 miss：标签越界（"999" 不在 44 值集）+ 媒介越界
	form["audiocodec_sel[4]"] = "1"
	form["medium_sel[4]"] = "100"
	tags = append(tags, model.TagKV{Key: "tags[4][]", Value: "999"})
	w = utopiaMusicValueSetAudit(form, tags)
	if len(w) != 2 {
		t.Fatalf("multi-miss: want 2, got %v", w)
	}
}

// §59.492 附六: 镜像(Mirror)=完整复制版（非 ISO 专属·站内 3588 镜像+WAV
// 实证）——编码看实际音频文件后缀（FileTree 权威·计数最大者）。
func TestUtopiaMusicFormValues_Mirror(t *testing.T) {
	m := &model.TorrentMetadata{Title: "Mirror Case"}
	mm := &model.MusicMeta{Format: "镜像(Mirror)", Media: "CD"}
	flacTor := bencTorrent([]string{"CD1/a.flac", "CD1/a.cue", "CD1/a.log", "CD2/b.flac", "CD2/b.log"})
	wavTor := bencTorrent([]string{"a.wav", "b.wav", "a.log"})  // 站内 3588 形态
	dsdTor := bencTorrent([]string{"a.iso", "b.dsf", "a.log"})  // RCA/SACD ISO 形态
	noAudio := bencTorrent([]string{"scan.jpg", "a.log"})       // 零音频文件

	flac := utopiaMusicFormValues(m, mm, flacTor) // 12 素材 Bach 案：镜像+CD+FLAC → 1
	if flac["audiocodec_sel[4]"] != "1" {
		t.Fatalf("flac mirror: %q want 1", flac["audiocodec_sel[4]"])
	}
	wav := utopiaMusicFormValues(m, mm, wavTor) // 站内 3588 → 8
	if wav["audiocodec_sel[4]"] != "8" {
		t.Fatalf("wav mirror: %q want 8", wav["audiocodec_sel[4]"])
	}
	dsd := utopiaMusicFormValues(m, mm, dsdTor) // RCA 60SACD：iso/dsf → 11
	if dsd["audiocodec_sel[4]"] != "11" {
		t.Fatalf("dsd mirror: %q want 11", dsd["audiocodec_sel[4]"])
	}
	na := utopiaMusicFormValues(m, mm, noAudio) // 零音频文件 → 无映射（③ fail-fast 保持）
	if _, ok := na["audiocodec_sel[4]"]; ok {
		t.Fatalf("no-audio mirror: should stay unmapped, got %q", na["audiocodec_sel[4]"])
	}
	gb := utopiaMusicFormValues(m, mm, []byte("garbage")) // 解析失败 → 无映射
	if _, ok := gb["audiocodec_sel[4]"]; ok {
		t.Fatalf("garbage mirror: should stay unmapped, got %q", gb["audiocodec_sel[4]"])
	}
}

// §59.492 刀5: 值集白名单数量锚——与 utopia.md 四节实时复核快照对齐
// （audiocodec 19 值含 0 占位→18 实值；medium 6→5；team 7→6；tags 44）。
// 站方值集漂移时此锚红——提醒同步文档+白名单+锚三处。
func TestUtopiaValueSetSizes(t *testing.T) {
	if n := len(utopiaAudioCodecValues); n != 18 {
		t.Fatalf("audiocodec 白名单 %d 项·want 18（utopia.md 四节 19 值含 0 占位）", n)
	}
	if n := len(utopiaMediumValues); n != 5 {
		t.Fatalf("medium 白名单 %d 项·want 5（6 值含 0）", n)
	}
	if n := len(utopiaTeamValues); n != 6 {
		t.Fatalf("team 白名单 %d 项·want 6（7 值含 0）", n)
	}
	if n := len(utopiaTagValues); n != 44 {
		t.Fatalf("tags 白名单 %d 项·want 44", n)
	}
	// 关键铁律值在场（1 禁转/2 首发/47·44 加速——恒不勾但须在白名单可识别）
	for _, k := range []string{"1", "2", "47", "44"} {
		if !utopiaTagValues[k] {
			t.Fatalf("tags 白名单缺关键值 %s", k)
		}
	}
}
