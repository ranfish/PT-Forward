package titleparser

import (
	"fmt"
	"testing"
)

func TestReassembleBTVCase(t *testing.T) {
	// 复现候选：platform=BTV，ST/SPEC 各种组合找 "HDTV HDTV" 来源
	for _, spec := range []string{"", "HDTV", "hdtv"} {
		p := TechProfile{
			MainTitle: "Vortex", Year: "2019", Resolution: "1080p",
			SourcePlatform: "BTV", SourceType: "HDTV", Specification: spec,
			VideoCodec: "H264", AudioCodec: "AAC",
		}
		tf := TitleFormat{Separator: " ", Order: []string{"title", "year", "resolution", "platform", "medium", "video_codec", "audio_full", "group"}}
		fmt.Printf("SPEC=%-5q → %s\n", spec, ReassembleFromTechProfile(p, tf))
	}
	// platform=HDTV（若 BTV 被解析进 ST 而非 platform）
	p2 := TechProfile{MainTitle: "Vortex", Year: "2019", Resolution: "1080p",
		SourcePlatform: "HDTV", SourceType: "HDTV", VideoCodec: "H264", AudioCodec: "AAC"}
	tf := TitleFormat{Separator: " ", Order: []string{"title", "year", "resolution", "platform", "medium", "video_codec", "audio_full", "group"}}
	fmt.Printf("platform=HDTV → %s\n", ReassembleFromTechProfile(p2, tf))
}
