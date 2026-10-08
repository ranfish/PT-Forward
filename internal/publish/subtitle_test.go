package publish

import "testing"

// §59.291: WEBVTT 中文字幕轨——UNABOMBER 案（NF WEB-DL 简繁双轨全 WEBVTT）
func TestSelectBestChinese_WebVTT(t *testing.T) {
	// UNABOMBER 实测形态：#1 English CC / #2 简中 / #3 繁中（全 webvtt）
	candidates := []SubtitleCandidate{
		{StreamIndex: 3, Codec: "webvtt", Language: "english", Title: "English [Original] (CC)", IsText: true, Score: 0},
		{StreamIndex: 4, Codec: "webvtt", Language: "chi", Title: "中文（简体）", IsText: true, Score: 15},
		{StreamIndex: 5, Codec: "webvtt", Language: "chi", Title: "中文（繁體）", IsText: true, Score: 13},
	}
	d := &SubtitleDetector{}
	idx, codec := d.SelectBestChinese(candidates)
	if idx != 4 || codec != "webvtt" {
		t.Errorf("应选简中轨(idx=4): got idx=%d codec=%s", idx, codec)
	}
	// sid 映射：候选序 1/2/3 → 简中是第 2 候选 → sid=2
	sid := 1
	for _, c := range candidates {
		if c.StreamIndex == idx {
			break
		}
		sid++
	}
	if sid != 2 {
		t.Errorf("mpv sid 应为 2: got %d", sid)
	}
	// mov_text 同槽
	c2 := []SubtitleCandidate{{StreamIndex: 7, Codec: "mov_text", Language: "chi", Title: "简体", IsText: true, Score: 15}}
	idx2, codec2 := d.SelectBestChinese(c2)
	if idx2 != 7 || codec2 != "mov_text" {
		t.Errorf("mov_text 应命中: got %d %s", idx2, codec2)
	}
	// ass 优先级不回归（同分 ass 赢）
	c3 := []SubtitleCandidate{
		{StreamIndex: 1, Codec: "webvtt", Language: "chi", Title: "简", IsText: true, Score: 15},
		{StreamIndex: 2, Codec: "ass", Language: "chi", Title: "简", IsText: true, Score: 15},
	}
	idx3, codec3 := d.SelectBestChinese(c3)
	if codec3 != "ass" {
		t.Errorf("ass 应优先: got %s", codec3)
	}
	_ = idx3
}

// §59.291 附: codec unknown 兜底——UNABOMBER 实测形态（MKV W_WEBVTT
// ffprobe 报 codec_name 缺省：三轨 eng/chi简/chi繁全空 codec）
func TestSelectBestChinese_UnknownCodec(t *testing.T) {
	d := &SubtitleDetector{}
	candidates := []SubtitleCandidate{
		{StreamIndex: 2, Codec: "", Language: "eng", Title: "English [Original] (CC)", IsText: true, Score: 0},
		{StreamIndex: 3, Codec: "", Language: "chi", Title: "中文（简体）", IsText: true, Score: 15},
		{StreamIndex: 4, Codec: "", Language: "chi", Title: "中文（繁體）", IsText: true, Score: 13},
	}
	idx, codec := d.SelectBestChinese(candidates)
	if idx != 3 {
		t.Errorf("unknown codec 简中应命中(idx=3): got %d %q", idx, codec)
	}
	// 纯 unknown 无语言无标题 → 非 candidate（Detect 层滤）——SelectBest 空集
	idx2, _ := d.SelectBestChinese(nil)
	if idx2 != 0 {
		t.Errorf("空集应 0")
	}
}

// §59.298: 无标记 PGS 兜底——蓝光原盘结构性缺失（Under Current 双 PGS 无 language/title）
func TestSelectBestChinese_UnmarkedPGS(t *testing.T) {
	d := &SubtitleDetector{}
	// 双 PGS 全无标记（Under Current 实测形态）
	candidates := []SubtitleCandidate{
		{StreamIndex: 3, Codec: "hdmv_pgs_subtitle", Language: "", Title: "", IsText: false, Score: 0},
		{StreamIndex: 4, Codec: "hdmv_pgs_subtitle", Language: "", Title: "", IsText: false, Score: 0},
	}
	idx, codec := d.SelectBestChinese(candidates)
	if idx != 3 || codec != "hdmv_pgs_subtitle" {
		t.Errorf("无标记 PGS 应选首轨(idx=3): got %d %q", idx, codec)
	}
	// 有中文标记仍优先（评分路径不回归）
	c2 := []SubtitleCandidate{
		{StreamIndex: 3, Codec: "hdmv_pgs_subtitle", Language: "", Title: "", IsText: false, Score: 0},
		{StreamIndex: 4, Codec: "hdmv_pgs_subtitle", Language: "chi", Title: "中文", IsText: false, Score: 12},
	}
	idx2, _ := d.SelectBestChinese(c2)
	if idx2 != 4 {
		t.Errorf("有标记轨应优先: got %d", idx2)
	}
	// §59.319 附十语义更新：混合零分（无中文）→ 按顺序第一轨（英文文本在前则选文本）
	// （旧 PGS 兜底优先语义废止——原盘场景双 PGS 无标记时新旧行为一致=首轨）
	c3 := []SubtitleCandidate{
		{StreamIndex: 2, Codec: "subrip", Language: "eng", Title: "English", IsText: true, Score: 0},
		{StreamIndex: 3, Codec: "hdmv_pgs_subtitle", Language: "", Title: "", IsText: false, Score: 0},
	}
	idx3, codec3 := d.SelectBestChinese(c3)
	if idx3 != 2 || codec3 != "subrip" {
		t.Errorf("混合零分应选第一轨: got %d %q", idx3, codec3)
	}
}
