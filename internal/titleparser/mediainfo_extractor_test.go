package titleparser

import "testing"


// §59.114: 兼容轨排除整体删除（v1.05:199 权威——唯一扣减是评论轨）。
// 兼容副本(TrueHD 内嵌 DD)也是正片音轨计入——极限审判 3 正片轨曾被误扣为 2。
func TestCountAudioTracksTitleEvidence(t *testing.T) {
	// 幽灵公主形态: DTS-HD MA + 3 条有 Title 标识的独立轨 = 4
	miMononoke := `Audio #1
Format : DTS-HD MA
Title                                    : Japanese
Audio #2
Format : AC-3
Title                                    : Mandarin (台配)
Audio #3
Format : AC-3
Title                                    : Mandarin (央视国配)
Audio #4
Format : AC-3
Title                                    : Cantonese`
	tech := ExtractMediaInfo(miMononoke)
	if tech.AudioTracks != 4 {
		t.Errorf("Title 标识独立轨不应扣减: got %d want 4", tech.AudioTracks)
	}
	// 兼容轨形态: TrueHD + 无 Title 的 DD = 1（同内容降级副本）
	miCompat := `Audio #1
Format : TrueHD
Audio #2
Format : AC-3`
	// §59.114: 兼容对(TrueHD+AC-3 无 Title)同样计入——v1.05 无兼容轨排除
	tech2 := ExtractMediaInfo(miCompat)
	if tech2.AudioTracks != 2 {
		t.Errorf("兼容对也计入(v1.05 无兼容排除): got %d want 2", tech2.AudioTracks)
	}
}

// §59.117: MI 五层结构化——ParseMISections（General/Video/Audio/Text/Menu）。
func TestParseMISections(t *testing.T) {
	mi := `General
Complete name : x.mkv
Format : Matroska
Overall bit rate : 32.1 Mb/s

Video
Format : HEVC
Width : 3 840 pixels
Frame rate : 23.976 FPS

Audio #1
Format : DTS XLL
Title : Japanese
Language : Japanese

Audio #2
Format : AC-3
Title : Mandarin (台配)
Language : Chinese (Taiwan)

Text #1
Format : PGS
Title : CHS
Language : Chinese

Menu
00:00:00.000 : en:Chapter 1`

	s := ParseMISections(mi)
	if s.General["overall bit rate"] != "32.1 Mb/s" {
		t.Errorf("General 层: %v", s.General["overall bit rate"])
	}
	if len(s.Videos) != 1 || s.Videos[0]["width"] != "3 840 pixels" {
		t.Errorf("Video 层: %+v", s.Videos)
	}
	if len(s.Audios) != 2 {
		t.Fatalf("Audio 层: %d 段", len(s.Audios))
	}
	if s.Audios[1]["title"] != "Mandarin (台配)" || s.Audios[1]["language"] != "Chinese (Taiwan)" {
		t.Errorf("Audio #2 字段: %+v", s.Audios[1])
	}
	if len(s.Texts) != 1 || s.Texts[0]["title"] != "CHS" {
		t.Errorf("Text 层: %+v", s.Texts)
	}
	if len(s.Menus) != 1 {
		t.Errorf("Menu 层: %d", len(s.Menus))
	}
	// 非标文本 fallback（不 panic、空结构）
	s2 := ParseMISections("不是 MI")
	if len(s2.Audios) != 0 {
		t.Errorf("非标文本应空结构")
	}
}
