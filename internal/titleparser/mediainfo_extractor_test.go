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
