package main

import (
	"fmt"
	"pt-forward/internal/titleparser"
)

func main() {
	title := "Ding Ding Zhan Hou Wang 1980 BluRay 1080p x265 10bit FLAC MNHD-FRDS"
	tp := titleparser.BuildTechProfile(title, "", "", "", "", "")
	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Group: %q\n", tp.Group)
	fmt.Printf("MainTitle: %q\n", tp.MainTitle)
	fmt.Printf("Resolution: %q\n", tp.Resolution)
	fmt.Printf("VideoCodec: %q\n", tp.VideoCodec)
	fmt.Printf("AudioCodec: %q\n", tp.AudioCodec)
	fmt.Printf("AudioChannels: %q\n", tp.AudioChannels)
	fmt.Printf("Source: %q\n", tp.Source)
	fmt.Printf("Year: %q\n", tp.Year)
	rt := titleparser.ReassembleFromTechProfile(tp, titleparser.V105TitleFormat())
	fmt.Printf("Reassembled: %q\n", rt)
}
