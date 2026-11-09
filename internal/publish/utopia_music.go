package publish

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
)

// §59.485 刀2: 乌托邦音乐发布翻译层（MusicMeta → 站方表单）。
//
// 架构：音乐种发布与影视链语义完全不同（type=408 恒定/name 四段式组装/
// 无 MI 无截图/影视标题重组对音乐必挂）——executor 分流到独立分支
// executeMusicPublish，本文件=分支的翻译层纯函数（可单测）。
//
// 值集词表=乌托邦摸底全量实证（docs/32/utopia.md 发布侧章节）：
// type/medium/audiocodec/team/language_id/tags 44 值——直配代码词表
// （站点专属翻译层·非 62 站通用 value_mappings 模式）。
//
// ⚠️ 主链同步债记录：本翻译层的上传/结果段内联自 executor 主链
// （§59.470 超时复查/§59.159 existing 分流等）——主链该段行为变更时
// 需同步（稳定后可收敛为公共函数）。

// utopiaMusicForm 乌托邦音乐表单值集（摸底实证·2026-11-07）
var (
	utopiaMusicType      = "408" // HQ Audio（种子区唯一音乐类型）
	utopiaMusicTeamOther = "5"   // Other（MusopiaBAK 淘汰组不配）
)

// utopiaMusicMedium 媒介值映射（MusicMeta.Media → medium_sel[4]）
var utopiaMusicMedium = map[string]string{
	"CD": "8", "SACD": "9", "DVD": "10", "VCD": "11",
}

// utopiaMusicAudioCodec 音频编码值映射（MusicMeta.Format → audiocodec_sel[4]）
var utopiaMusicAudioCodec = map[string]string{
	"FLAC": "1", "WAV": "8", "PCM": "8", "PCM/WAV": "8",
	"ALAC": "9", "WavPack": "10", "DSD": "11", "DSF": "11", "DSD/DSF": "11",
	"DFF": "12", "Hi-Res PCM": "16", "APE": "2", "MP3": "4", "AAC": "6",
	"OGG": "5", "DTS": "3", "WMA": "13",
}

// utopiaMusicTeam 组名值映射（组名识别命中 → team_sel[4]；未命中 Other）
var utopiaMusicTeam = map[string]string{
	"Musopia": "3", "HDS": "1", "CHD": "2", "WiKi": "4",
}

// utopiaMusicGenreTags 流派映射（皇后 Genre/Tags 值形 → tags[4][] 值）
// 两站均 NP 形态（"古典(Classical)"同款）——contains 词形匹配。
var utopiaMusicGenreTags = []struct{ word, val string }{
	{"古典", "15"}, {"Classical", "15"},
	{"流行", "10"}, {"Pop", "10"},
	{"爵士", "11"}, {"Jazz", "11"},
	{"摇滚", "9"}, {"rock", "9"}, {"Rock", "9"},
	{"原声", "16"}, {"OST", "16"},
	{"电子", "12"}, {"Electronic", "12"},
	{"金属", "13"}, {"Metal", "13"},
	{"说唱", "20"}, {"Rap", "20"},
	{"嘻哈", "32"}, {"hip.hop", "32"}, {"Hip", "32"},
	{"蓝调", "22"}, {"Blues", "22"},
	{"民间", "23"}, {"Folk", "23"},
	{"乡村", "24"}, {"Country", "24"},
	{"朋克", "25"}, {"Punk", "25"},
	{"新世纪", "19"}, {"NewAge", "19"},
	{"天籁", "18"}, {"Soul", "18"},
	{"发烧", "17"}, {"HiFi", "17"},
	{"民族", "26"}, {"National", "26"},
	{"音乐剧", "27"}, {"musical", "27"},
	{"宗教", "29"}, {"Religion", "29"},
	{"儿童", "35"}, {"Children", "35"},
	{"器乐", "34"}, {"Instrumental", "34"},
	{"latin", "33"}, {"Latin", "33"},
	{"piano", "42"}, {"Piano", "42"},
	{"indie", "40"}, {"Indie", "40"},
	{"japanese", "39"}, {"Japanese", "39"},
	{"korean", "41"}, {"Korean", "41"},
	{"dance", "38"}, {"Dance", "38"},
	{"bollywood", "43"}, {"Bollywood", "43"},
}

// utopiaMusicRegionTags 地区映射（→ tags 值 + language_id 双消费）
var utopiaMusicRegionTags = []struct {
	word     string
	tagVal   string
	langVal  string
}{
	{"港台", "6", "41"},
	{"大陆", "4", "41"},
	{"内地", "4", "41"},
	{"华语", "4", "41"}, // 华语 tag 无独立值——归大陆；language=华语
	{"欧美", "5", "42"},
	{"日韩", "7", "43"},
	{"日本", "7", "43"},
	{"韩国", "7", "43"},
	{"其他地区", "8", "0"},
}

// 来源站标记（tags 30/31/36/37——官组制：源种有官组才打·皇后官组=OpenCD）
var utopiaMusicSourceTags = map[string]string{
	"皇后": "31", // OpenCD
}

// 源站官组名（name 第四段——mm.Origin=="原创" 即官组种）
var utopiaMusicSourceGroup = map[string]string{
	"皇后": "OpenCD",
}

var (
	// 皇后站方 Score 标注行（"******************** Score : 100 ********************"）
	utopiaScoreLineRe = regexp.MustCompile(`(?m)^\s*\*+\s*Score\s*:\s*(\d+)\s*\*+\s*$`)
	// EAC/XLD log 段头（多 CD 分界）
	utopiaEACHeadRe = regexp.MustCompile(`(?m)^Exact Audio Copy V[\d.]+ .*$`)
	utopiaXLDHeadRe = regexp.MustCompile(`(?m)^X Lossless Decoder .*$`)
)

// musicMetaOfMeta §59.485: 音乐种判定+MusicMeta 获取（executor 分流通道）。
// detail_source_json.music 在场=音乐种（与 api.detailSourceHasMusic 同语义）。
func musicMetaOfMeta(meta *model.TorrentMetadata) *model.MusicMeta {
	if meta == nil || meta.DetailSourceJSON == "" {
		return nil
	}
	ds, err := metadata.UnmarshalDetailSource(meta.DetailSourceJSON)
	if err != nil || ds == nil {
		return nil
	}
	return ds.Music
}

// utopiaMusicR7Blocked §59.485: 音乐版 R7 禁转判定（发布前拦）。
// 矩阵（用户定案）：皇后=ShareRule 文本（欢迎→放·禁转→拦）；
// 海豚=VH∨scene 拦（原创作品/Scene 发布——标记补采 §59.485 刀1）·
// 双 false 放（站方规则：转载种无权禁转）；乌托邦=放（出站开放声明）。
func utopiaMusicR7Blocked(meta *model.TorrentMetadata, mm *model.MusicMeta) (bool, string) {
	switch meta.SiteName {
	case "海豚":
		if mm.IsVanityHouse {
			return true, "海豚原创作品（Vanity House）——禁转（R7：原创资源不转发）"
		}
		if mm.IsScene {
			return true, "海豚 Scene 发布——禁转（R7 定案）"
		}
		return false, ""
	case "乌托邦":
		return false, ""
	default: // 皇后及其它音乐源站：ShareRule 文本判定
		rule := mm.ShareRule
		if strings.Contains(rule, "禁转") || strings.Contains(rule, "不得转载") ||
			strings.Contains(rule, "禁止转载") || strings.Contains(rule, "Vanity House") ||
			strings.Contains(rule, "Scene 发布") {
			return true, fmt.Sprintf("源站 %s 转载规则禁转：%s", meta.SiteName, rule)
		}
		return false, ""
	}
}

// utopiaMusicGroupName §59.485 决策点②: 源种官组名判定（来源标记）。
// 皇后 mm.Origin=="原创"=官组种（"来自：原创"字段行）→组名 OpenCD；
// 非官组（"转载"）→空（name 第四段省略+不打来源 tag）。
func utopiaMusicGroupName(meta *model.TorrentMetadata, mm *model.MusicMeta) string {
	if mm == nil || mm.Origin != "原创" {
		return ""
	}
	if g, ok := utopiaMusicSourceGroup[meta.SiteName]; ok {
		return g
	}
	return ""
}

// utopiaMusicName §59.485 决策点②④: name 四段式组装。
// `Artist - Album Year - Format AudioMode - Group`（站方规范原文）：
//   - album 空 → 皇后标题中段兜底（" - " 分段第 2 段剥年份——站方标题
//     有详情页字段行没有·保真提取）
//   - AudioMode 简繁归一（分軌→分轨——乌托邦 enrich 同款）
//   - 无官组 → 第四段省略（站方"未知出处可省"）
func utopiaMusicName(meta *model.TorrentMetadata, mm *model.MusicMeta, groupName string) string {
	artist := strings.TrimSpace(mm.Artist)
	if artist == "" {
		artist = strings.TrimSpace(meta.ChineseTitle)
	}
	if artist == "" {
		// 艺术家全空：标题首段兜底
		if seg := strings.SplitN(meta.Title, " - ", 2); len(seg) == 2 {
			artist = strings.TrimSpace(seg[0])
		} else {
			artist = strings.TrimSpace(meta.Title)
		}
	}
	album := strings.TrimSpace(mm.Album)
	if album == "" {
		album = utopiaAlbumFallbackFromTitle(meta.Title)
	}
	year := strings.TrimSpace(mm.Year)
	mode := utopiaNormalizeAudioMode(mm.AudioMode)
	format := strings.ToUpper(strings.TrimSpace(mm.Format))
	if format == "" {
		format = "FLAC" // 皇后音乐主形态默认（站方 audiocodec 必选——miss 走 fail-fast）
	}
	var mid string
	switch {
	case album != "" && year != "":
		mid = album + " " + year
	case album != "":
		mid = album
	case year != "":
		mid = year
	}
	third := strings.TrimSpace(format + " " + mode)
	parts := []string{artist}
	if mid != "" {
		parts = append(parts, mid)
	}
	if third != "" {
		parts = append(parts, third)
	}
	if groupName != "" {
		parts = append(parts, groupName)
	}
	return strings.Join(parts, " - ")
}

// utopiaAlbumFallbackFromTitle album 空时皇后标题中段兜底（决策点④）。
func utopiaAlbumFallbackFromTitle(title string) string {
	segs := strings.Split(title, " - ")
	if len(segs) < 2 {
		return ""
	}
	mid := strings.TrimSpace(segs[1])
	if mid == "" {
		return ""
	}
	// 剥尾部年份（"xxx 2005" → "xxx"——保留剥前形态若有意义的部分）
	if m := regexp.MustCompile(`\s+(19|20)\d{2}\s*$`).FindStringSubmatch(mid); m != nil {
		mid = strings.TrimSpace(strings.TrimSuffix(mid, m[0]))
	}
	return mid
}

// utopiaNormalizeAudioMode AudioMode 简繁归一（分軌→分轨·烏托邦 enrich 同款词序）。
func utopiaNormalizeAudioMode(mode string) string {
	mode = strings.TrimSpace(mode)
	mode = strings.ReplaceAll(mode, "軌", "轨")
	if mode == "" {
		return ""
	}
	// 归一常用形态（"整轨"/"分轨"——原始可能带前缀空格或括号）
	if strings.Contains(mode, "整") {
		return "整轨"
	}
	if strings.Contains(mode, "分") {
		return "分轨"
	}
	return mode
}

// utopiaMusicSubtitle §59.485 决策点⑦: 副标题规格链（对齐站内转载种实践
// "FLAC / Lossless / CD / Log (100%) / Cue"——海豚同款形态）·缺项跳过。
func utopiaMusicSubtitle(mm *model.MusicMeta) string {
	var segs []string
	if f := strings.ToUpper(strings.TrimSpace(mm.Format)); f != "" {
		segs = append(segs, f)
		// Lossless 段：无损族格式才加
		switch f {
		case "FLAC", "WAV", "ALAC", "APE", "WAVPACK":
			segs = append(segs, "Lossless")
		}
	}
	if m := strings.TrimSpace(mm.Media); m != "" {
		segs = append(segs, m)
	}
	if mm.LogScore > 0 {
		segs = append(segs, fmt.Sprintf("Log (%d%%)", mm.LogScore))
	} else if mm.HasLog {
		segs = append(segs, "Log")
	}
	if mm.HasCue {
		segs = append(segs, "Cue")
	}
	return strings.Join(segs, " / ")
}

// utopiaMusicLogTexts §59.485 决策点③: logtexts[] 拆 CD。
// 皇后 LogText（divnfo 全文）按 EAC/XLD 段头分界拆段·每段一个
// logtexts[]；皇后站方 Score 标注行先剥（那是皇后站的评分·乌托邦
// logcheck.php 会重评）；海豚源 LogText 空→nil（不传·站方"留空则
// 不传"合法）。段数超种内 .log 数站方自动忽略（守恒律·无害）。
func utopiaMusicLogTexts(mm *model.MusicMeta) []string {
	raw := strings.TrimSpace(mm.LogText)
	if raw == "" {
		return nil
	}
	// ① 剥皇后 Score 标注行（全行形态）
	raw = utopiaScoreLineRe.ReplaceAllString(raw, "")
	// ② EAC/XLD 段头分界拆段
	splitRe := utopiaEACHeadRe
	if splitRe.FindStringIndex(raw) == nil {
		splitRe = utopiaXLDHeadRe
		if splitRe.FindStringIndex(raw) == nil {
			// 无段头形态：整段单 CD
			if t := strings.TrimSpace(raw); t != "" {
				return []string{t}
			}
			return nil
		}
	}
	idxs := splitRe.FindAllStringIndex(raw, -1)
	var segs []string
	for i, loc := range idxs {
		end := len(raw)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		seg := strings.TrimSpace(raw[loc[0]:end])
		if seg != "" {
			segs = append(segs, seg)
		}
	}
	return segs
}

// utopiaMusicRipScore §59.485 决策点③: rip_score——LogScore 直通+
// LogText 首个 Score 行兜底（皇后 log_score 字段 0/181 全空的补偿通道）。
func utopiaMusicRipScore(mm *model.MusicMeta) string {
	if mm.LogScore > 0 {
		return fmt.Sprintf("%d", mm.LogScore)
	}
	if m := utopiaScoreLineRe.FindStringSubmatch(mm.LogText); len(m) > 1 {
		return m[1]
	}
	return ""
}

// ── 乌托邦值集白名单（§59.492 刀3 L2——四节实时复核 2026-11-09 快照·零漂移）──

var utopiaAudioCodecValues = map[string]bool{
	"1": true, "8": true, "9": true, "10": true, "11": true, "12": true,
	"13": true, "14": true, "15": true, "4": true, "16": true, "17": true,
	"18": true, "6": true, "7": true, "5": true, "3": true, "2": true,
}
var utopiaMediumValues = map[string]bool{"8": true, "9": true, "10": true, "11": true, "12": true}
var utopiaTeamValues = map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": true, "6": true}
var utopiaTagValues = map[string]bool{
	"1": true, "2": true, "6": true, "4": true, "5": true, "7": true,
	"8": true, "10": true, "31": true, "30": true, "47": true, "44": true,
	"43": true, "42": true, "41": true, "40": true, "39": true, "38": true,
	"37": true, "36": true, "35": true, "34": true, "33": true, "32": true,
	"29": true, "28": true, "27": true, "26": true, "25": true, "24": true,
	"23": true, "22": true, "21": true, "20": true, "19": true, "18": true,
	"17": true, "16": true, "15": true, "14": true, "13": true, "12": true,
	"11": true, "9": true,
}

// utopiaMusicValueSetAudit §59.492 刀3 L2：DryRun 组装值 vs 值集白名单
// 校验——miss 收集为 warnings（不拦：组装值来自翻译层映射·miss=映射
// bug 或数据异常·DryRun 人工核对；正式发布不变）。
func utopiaMusicValueSetAudit(form map[string]string, tagArray []model.TagKV) []string {
	var w []string
	for _, slot := range []struct{ key, name string; set map[string]bool }{
		{"audiocodec_sel[4]", "音频编码", utopiaAudioCodecValues},
		{"medium_sel[4]", "媒介", utopiaMediumValues},
		{"team_sel[4]", "制作组", utopiaTeamValues},
	} {
		if v, ok := form[slot.key]; ok && v != "" && !slot.set[v] {
			w = append(w, fmt.Sprintf("值集miss %s=%q（不在站方 %d 值白名单）", slot.name, v, len(slot.set)))
		}
	}
	for _, kv := range tagArray {
		if kv.Key == "tags[4][]" && !utopiaTagValues[kv.Value] {
			w = append(w, fmt.Sprintf("值集miss 标签=%q（不在站方 44 值集）", kv.Value))
		}
	}
	return w
}

// utopiaMusicLogCount §59.492 刀2: 种内 .log 文件数（乌托邦 log 硬拦
// 预检——朱孝天案实证站方拦截"音乐种子包内未找到EAC抓轨LOG文件"）。
// 判定=torrent 文件实体（fingerprint.FileTree·本地权威零网络·与
// 站方 parse_torrent.php log_count 同源）。err=解析失败（fail-close
// 消费——防误发）。
func utopiaMusicLogCount(torrentData []byte) (int, error) {
	m, err := fingerprint.ComputeFromTorrent(torrentData)
	if err != nil {
		return 0, err
	}
	logs := 0
	for p := range m.FileTree {
		if strings.HasSuffix(strings.ToLower(p), ".log") {
			logs++
		}
	}
	return logs, nil
}

// utopiaMusicDescr §59.485 决策点④+③: descr 组装。
// `[quote]转载{源站}资源，感谢[/quote]` + `[img]{poster}[/img]` + 正文
// （皇后重组 intro.body：剥首 [img] 行（封面段）+ 从 LOG 段边界截断
// ——log 走 logtexts[] 独立通道·乌托邦站内实证样本 descr 无 log 全文）。
func utopiaMusicDescr(meta *model.TorrentMetadata, mm *model.MusicMeta) string {
	var b strings.Builder
	b.WriteString("[quote]转载" + meta.SiteName + "资源，感谢[/quote]\n")
	if poster := strings.TrimSpace(meta.Poster); poster != "" {
		b.WriteString("[img]" + poster + "[/img]\n")
	}
	body := utopiaDescrBody(meta.SourceDescription)
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
	}
	return b.String()
}

var utopiaDescrImgLineRe = regexp.MustCompile(`(?m)^\[img\][^\[]*\[/img\]\s*$`)

// utopiaDescrBody descr 正文处理：剥首图行+LOG 段截断（EAC/XLD/Score 首见处→文末）。
func utopiaDescrBody(descr string) string {
	if descr == "" {
		return ""
	}
	// ① 剥全部 [img] 行（封面/正文图——attachments/ 相对路径跨站必裂·
	// 封面已由 rehost pixhost 首图独立承载）
	body := utopiaDescrImgLineRe.ReplaceAllString(descr, "")
	// ② LOG 段截断（三种边界首见处）
	for _, re := range []*regexp.Regexp{utopiaEACHeadRe, utopiaXLDHeadRe, utopiaScoreLineRe} {
		if loc := re.FindStringIndex(body); loc != nil {
			body = body[:loc[0]]
		}
	}
	return strings.TrimSpace(body)
}

// utopiaMusicTags §59.485 决策点②: tags[4][] 组装（流派+地区+来源站）。
// 铁律：禁转(1)恒不勾（R7 上游拦）·首发(2)恒不勾（转存发布铁律）·
// 加速(47/44)恒不勾（官方功能标签）·官方标签值集外不可选。
func utopiaMusicTags(meta *model.TorrentMetadata, mm *model.MusicMeta, groupName string) []string {
	seen := map[string]bool{}
	var tags []string
	add := func(v string) {
		if v != "" && !seen[v] && len(seen) < 10 { // 站方无明确上限·10 防御界
			seen[v] = true
			tags = append(tags, v)
		}
	}
	// 来源站标记（官组制）
	if groupName != "" {
		if v, ok := utopiaMusicSourceTags[meta.SiteName]; ok {
			add(v)
		}
	}
	// 地区（mm.Tags 优先·Genre 兜底）
	regionSrc := append([]string{}, mm.Tags...)
	if mm.Genre != "" {
		regionSrc = append(regionSrc, mm.Genre)
	}
	for _, t := range regionSrc {
		for _, r := range utopiaMusicRegionTags {
			if strings.Contains(t, r.word) {
				add(r.tagVal)
				break
			}
		}
	}
	// 流派（Genre 优先·Tags 兜底）
	for _, src := range append([]string{mm.Genre}, regionSrc...) {
		if src == mm.Genre && src == "" {
			continue
		}
		for _, g := range utopiaMusicGenreTags {
			if strings.Contains(src, g.word) {
				add(g.val)
				break
			}
		}
	}
	return tags
}

// utopiaMusicLanguageID language_id 组装（地区推导·miss=0 未标注——站方有 0 选项）。
func utopiaMusicLanguageID(meta *model.TorrentMetadata, mm *model.MusicMeta) string {
	src := append([]string{}, mm.Tags...)
	if mm.Genre != "" {
		src = append(src, mm.Genre)
	}
	for _, t := range src {
		for _, r := range utopiaMusicRegionTags {
			if strings.Contains(t, r.word) && r.langVal != "0" {
				return r.langVal
			}
		}
	}
	return "0"
}

// utopiaMusicFormValues 音乐表单标量域组装（type/medium/audiocodec/team）。
// medium miss 留空（站方可选实证——官种都没填媒介）；audiocodec miss
// 留空由 fail-fast 拦（必选域）；team miss=Other（决策点①）。
func utopiaMusicFormValues(meta *model.TorrentMetadata, mm *model.MusicMeta) map[string]string {
	form := map[string]string{
		"type": utopiaMusicType,
	}
	if v, ok := utopiaMusicMedium[strings.TrimSpace(mm.Media)]; ok {
		form["medium_sel[4]"] = v
	}
	if v, ok := utopiaMusicAudioCodec[strings.ToUpper(strings.TrimSpace(mm.Format))]; ok {
		form["audiocodec_sel[4]"] = v
	}
	// team：组名识别 → 值集匹配 → Other（决策点①）。
	// 组名信号现状：meta 无站方组名列（皇后"制作组"字段行未落 meta 列）——
	// 无识别通道=恒 Other（零冒认·定案"显式识别才选"语义）；未来组名列
	// 落地后接入值集匹配。
	form["team_sel[4]"] = utopiaMusicTeamOther
	form["language_id"] = utopiaMusicLanguageID(meta, mm)
	if score := utopiaMusicRipScore(mm); score != "" {
		form["rip_score"] = score
	}
	return form
}
