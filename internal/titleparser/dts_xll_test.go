package titleparser

import (
	"strings"
	"testing"
)

// §59.514 附十 A: DTS ES XLL 音频选择锚——Aces Go Places V 复刻。
// MI 多轨 [DTS ES XLL/8, FLAC/2, AC-3/6, FLAC/2 评论] → 最高规格应选
// DTS-HD MA 7.1（XLL=无损后缀·docs/31 :71356 设计权威表；原 bug：
// dict 归 DTS-ES rank 75 < FLAC 85 → 误选 FLAC 2.0·熊猫/龙PT 56 行实证）。
func TestSelectBestAudioDTSXLL(t *testing.T) {
	mi := `General
Complete name    : /tmp/Aces.V.mkv
Video
Format           : HEVC
Writing library  : x265
Height           : 1 604 pixels
Audio #1
Format           : DTS ES XLL
Channel(s)       : 8 channels
Title            : Cantonese
Audio #2
Format           : FLAC
Channel(s)       : 2 channels
Title            : Cantonese
Audio #3
Format           : AC-3
Channel(s)       : 6 channels
Title            : Mandarin
Audio #4
Format           : FLAC
Channel(s)       : 2 channels
Title            : Commentary by James M
`
	got := ExtractMediaInfo(mi)
	if got.AudioCodec != "DTS-HD MA" {
		t.Errorf("AudioCodec=%q want DTS-HD MA（XLL 无损最高规格）", got.AudioCodec)
	}
	if got.AudioChannels != "7.1" {
		t.Errorf("AudioChannels=%q want 7.1（8 声道）", got.AudioChannels)
	}
}

// §59.514 附十 A: 有损 DTS-ES 标题写法归一 DTS（v1.05 音频表无 DTS-ES
// 行·canonical 身份层不动——26 行 off-spec 输出修复）。
func TestReassembleDTSESTitleForm(t *testing.T) {
	p := TechProfile{
		MainTitle:    "Movie",
		Year:         "2020",
		Resolution:   "1080p",
		SourceType:   "BluRay",
		VideoCodec:   "x265",
		AudioCodec:   "DTS-ES",
		AudioChannels: "6.1",
		ReleaseGroup: "GRP",
	}
	rt := ReassembleFromTechProfile(p, V105TitleFormat())
	if strings.Contains(rt, "DTS-ES") || strings.Contains(rt, "DTS ES") {
		t.Errorf("标题含 DTS-ES 字面: %q（v1.05 无此行——应写 DTS）", rt)
	}
	if !strings.Contains(rt, "DTS 6.1") {
		t.Errorf("标题应含归一后 \"DTS 6.1\": %q", rt)
	}
}
