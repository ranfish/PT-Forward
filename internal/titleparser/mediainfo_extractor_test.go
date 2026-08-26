package titleparser

import "testing"


// §59.113: 兼容轨排除加 Title 佐证——Title 有内容标识(台配/央视国配/Cantonese)
// 是独立音轨非兼容副本（幽灵公主 4 轨被误扣 1 实锤）；兼容轨技术特征是 Title 空。
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
	tech2 := ExtractMediaInfo(miCompat)
	if tech2.AudioTracks != 1 {
		t.Errorf("无 Title 兼容轨仍应扣减: got %d want 1", tech2.AudioTracks)
	}
}
