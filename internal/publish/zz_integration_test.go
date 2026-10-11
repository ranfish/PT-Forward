package publish

import (
	"testing"
)

// §59.319 附二十六 集成验证：用光之子完整 Description + MI 模拟真实调用
func TestIntegrationGuangZhizi(t *testing.T) {
	desc := "[img]https://doubaninfo.com/dbposter/doubaninfo_com_35123121.jpg[/img]\n\n" +
		"◎片　　名　光之子\n" +
		"◎译　　名　Daughter of the Light\n" +
		"◎年　　代　2020\n" +
		"◎产　　地　中国大陆\n" +
		"◎类　　别　纪录片\n" +
		"◎语　　言　汉语普通话\n" +
		"◎上映日期　2024-10-20(中国大陆)"

	mi := `General
Complete name : 光之子.Daughter.Of.The.Light.2020.2160p.WEB-DL.H.265.AAC2.0-CSWEB.mkv
Format : Matroska
File size : 3.98 GiB
Duration : 1 h 40 min
Overall bit rate : 5 676 kb/s

Video
Format : HEVC
Width : 3 840 pixels
Height : 2 160 pixels
Frame rate : 24.000 FPS
Bit depth : 8 bits

Audio
Format : AAC LC
Channel(s) : 2 channels
Sampling rate : 48.0 kHz`

	inf := NewMediaTagInferer()
	tags := inf.InferFull(TagInput{
		Title:       "Daughter Of The Light 2020 2160p WEB-DL H.265 AAC2.0-CSWEB",
		Subtitle:    "光之子 | Daughter Of The Light | 类型: 纪录片",
		Description: desc,
		MediaInfo:   mi,
	})

	t.Logf("tags: %v", tags)
	found := map[string]bool{}
	for _, tg := range tags {
		found[tg] = true
	}
	if !found["chinese_audio"] {
		t.Error("缺少 chinese_audio")
	}
	if !found["chinese_subtitle"] {
		t.Error("缺少 chinese_subtitle")
	}
	if !found["resolution_4k"] {
		t.Error("缺少 resolution_4k")
	}
}
