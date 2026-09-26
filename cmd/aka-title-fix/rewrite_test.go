package main

import "testing"

// §59.290: 存量标题重写——摸底六环境实测形态
func TestRewriteTitle(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Chi l'ha vista morire? AKA Who Saw Her Die? 1972 1080p USA Blu-ray AVC LPCM 1.0-Debaucherous",
			"Who Saw Her Die? 1972 1080p USA Blu-ray AVC LPCM 1.0-Debaucherous"},
		{"Amaran aka SK21 2024 1080P NF WEB-DL AV1 DDP5.1 5Audio-SHB931@UBWEB",
			"Amaran 2024 1080P NF WEB-DL AV1 DDP5.1 5Audio-SHB931@UBWEB"},
		{"Andhra King Taluka aka RaPo 22 (2025) 1080P NF WEB-DL H264 DDP5.1 5Audio-SHB931@UBWEB",
			"Andhra King Taluka (2025) 1080P NF WEB-DL H264 DDP5.1 5Audio-SHB931@UBWEB"},
		{"Ikiru AKA To Live 1952 UHD BluRay 2160p x265 SDR FLAC mUHD-FRDS",
			"To Live 1952 UHD BluRay 2160p x265 SDR FLAC mUHD-FRDS"},
		{"The.Piano.in.a.Factory.AKA.Gang.de.qin.2010.CHN.BluRay.1080p.x264.DDP.5.1-CMCT",
			"The.Piano.in.a.Factory.2010.CHN.BluRay.1080p.x264.DDP.5.1-CMCT"},
		{"The.Tiger.AKA.Der.Tiger.2025.2160p.AMZN.WEB-DL.H265.10bit.HDR.DDP.5.1-CMCTV",
			"The.Tiger.2025.2160p.AMZN.WEB-DL.H265.10bit.HDR.DDP.5.1-CMCTV"},
		{"Mononoke-hime AKA Princess Mononoke 1997 UHD BluRay 2160p x265 DV HDR DTS-HD MA 5.1-FRDS",
			"Princess Mononoke 1997 UHD BluRay 2160p x265 DV HDR DTS-HD MA 5.1-FRDS"},
		// 无 AKA 原样
		{"Inception 2010 1080p BluRay x264-GRP", "Inception 2010 1080p BluRay x264-GRP"},
	}
	for _, tc := range cases {
		if got := RewriteTitle(tc.in); got != tc.want {
			t.Errorf("in=%q\n got=%q\nwant=%q", tc.in, got, tc.want)
		}
	}
}
