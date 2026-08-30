package main

import (
	"fmt"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

func main() {
	p5 := "Dolby Vision, Version 1.0, Profile 5, dvhe.05.06, BL+RPU, no metadata compression"
	p7 := "Dolby Vision, Version 1.0, Profile 7.6, dvhe.07.06, BL+EL+RPU, no metadata compression, Blu-ray compatible / SMPTE ST 2094 App 4, Version HDR10+ Profile A, HDR10+ Profile A compatible"
	for _, hdr := range []string{p5, p7} {
		s := titleparser.ParseMISections("Video\nHDR format : " + hdr)
		out := inferHDR(titleparser.MISections{})
		fmt.Printf("MI HDR=%s...\n→ hdrFromMI: %q\n", hdr[:40], titleparser.ExtractMediaInfo("Video\nHDR format : " + hdr).HDR)
		_ = s
		_ = out
	}
}

func inferHDR(s titleparser.MISections) []string { return nil }
