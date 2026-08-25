package titleparser

import "testing"

func TestExtractEditionInfo(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Movie.2024.IMAX.1080p.BluRay", "IMAX"},
		{"Movie.Extended.Cut.1999.1080p", "Extended Cut"},
		{"Movie.Extended.1999.1080p", "Extended"},
		{"Movie.Director's.Cut.2000", "Director's Cut"},
		{"Movie.2009.10th.Anniversary.Edition", "Anniversary Edition"},
		{"Movie.2010.Remastered.2160p", "Remastered"},
		{"Movie.Hybrid.2020", "Hybrid"},
		{"Movie.4K.Remaster.2015", "4K Remaster"},
		{"Movie.IMAX.Enhanced.2024", "IMAX Enhanced"},
		{"Movie.Open.Matte.1993", "Open Matte"},
		{"Movie.MiniBD.2018", "MiniBD"},
		{"Movie.2024.1080p.BluRay.x264", ""}, // 无版本信息
	}
	for _, tt := range tests {
		got := extractEditionInfo(tt.title)
		if got != tt.want {
			t.Errorf("extractEditionInfo(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestExtractEditionInfo_ConsumeNoDup(t *testing.T) {
	// "Extended Cut" 被 "Extended Cut" pattern 消费后，"Extended" 不应重复匹配
	got := extractEditionInfo("Movie.Extended.Cut.1999")
	if got != "Extended Cut" {
		t.Errorf("Extended Cut 消费测试：got %q, want %q", got, "Extended Cut")
	}
}

func TestSplitMedium(t *testing.T) {
	tests := []struct {
		medium   string
		srcType  string
		spec     string
	}{
		{"WEB-DL", "", "WEB-DL"},
		{"WEBRip", "", "WEBRip"},
		{"Blu-ray", "Blu-ray", ""},
		{"UHD Blu-ray", "UHD Blu-ray", ""},
		{"UHD Blu-ray Remux", "UHD Blu-ray", "Remux"},
		{"Blu-ray Remux", "Blu-ray", "Remux"},
		{"HDTV", "", "HDTV"},
		{"UHDTV", "", "UHDTV"},
		{"3D Blu-ray", "3D Blu-ray", ""},
		{"DVD", "DVD", ""},
		{"DVDRip", "DVD", "DVDRip"},
		{"", "", ""},
	}
	for _, tt := range tests {
		st, sp := splitMedium(tt.medium)
		if st != tt.srcType || sp != tt.spec {
			t.Errorf("splitMedium(%q) = (%q, %q), want (%q, %q)",
				tt.medium, st, sp, tt.srcType, tt.spec)
		}
	}
}

func TestExtractAudioChannelsFromTitle(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Movie DTS-HD MA 5.1", "5.1"},
		{"Movie TrueHD 7.1 Atmos", "7.1"},
		{"Movie AAC 2.0", "2.0"},
		{"Movie 2024 1080p", ""},
		{"Movie v2.0 Release", "2.0"}, // 边界：v2.0 中的 2.0 匹配，但 MediaInfo 会覆盖
	}
	for _, tt := range tests {
		got := extractAudioChannelsFromTitle(tt.title)
		if got != tt.want {
			t.Errorf("extractAudioChannelsFromTitle(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestExtractAudioTechnologyFromTitle(t *testing.T) {
	assertEquals(t, "Atmos", "Atmos", extractAudioTechnologyFromTitle("Movie DDP 5.1 Atmos"))
	assertEquals(t, "noAtmos", "", extractAudioTechnologyFromTitle("Movie DDP 5.1"))
}

func TestExtractAudioTracksFromTitle(t *testing.T) {
	tests := []struct {
		title string
		want  int
	}{
		{"Movie 2Audios DTS", 2},
		{"Movie 3Audios", 3},
		{"Movie 2Audio", 2},
		{"Movie 2024", 0},
	}
	for _, tt := range tests {
		got := extractAudioTracksFromTitle(tt.title)
		if got != tt.want {
			t.Errorf("extractAudioTracksFromTitle(%q) = %d, want %d", tt.title, got, tt.want)
		}
	}
}

func TestParseTitleTech(t *testing.T) {
	p := ParseTitleTech("Movie 2024 IMAX 2160p Blu-ray x265 TrueHD 7.1 Atmos")

	assertEquals(t, "Resolution", "2160p", p.Resolution)
	assertEquals(t, "EditionInfo", "IMAX", p.EditionInfo)
	assertEquals(t, "SourceType", "Blu-ray", p.SourceType)
	assertEquals(t, "VideoCodec", "x265", p.VideoCodec)
	assertEquals(t, "AudioCodec", "TrueHD", p.AudioCodec)
	assertEquals(t, "AudioChannels", "7.1", p.AudioChannels)
	assertEquals(t, "AudioTechnology", "Atmos", p.AudioTechnology)
}

func TestParseTitleTech_WEBDL(t *testing.T) {
	p := ParseTitleTech("Test.Show.2024.S01E03.1080p.NF.WEB-DL.DDP5.1.Atmos.H.264-NTb")

	assertEquals(t, "SeasonEpisode", "S01E03", p.SeasonEpisode)
	assertEquals(t, "Specification", "WEB-DL", p.Specification)
	assertEquals(t, "AudioChannels", "5.1", p.AudioChannels)
}

func TestMergeMediaInfoInto(t *testing.T) {
	p := ParseTitleTech("Movie 2024 1080p WEB-DL x264 AAC")
	mi := &MediaInfoTech{
		Resolution:    "2160p",
		VideoCodec:    "HEVC",
		AudioCodec:    "DDP",
		AudioChannels: "5.1",
		HDR:           "DoVi",
		BitDepth:      "10bit",
		AudioTracks:   2,
		AudioTechnology: "Atmos",
	}
	MergeMediaInfoInto(&p, mi)

	assertEquals(t, "Resolution", "2160p", p.Resolution)
	assertEquals(t, "VideoCodec", "HEVC", p.VideoCodec)
	assertEquals(t, "AudioCodec", "DDP", p.AudioCodec)
	assertEquals(t, "AudioChannels", "5.1", p.AudioChannels)
	assertEquals(t, "HDR", "DoVi", p.HDR)
	assertEquals(t, "BitDepth", "10bit", p.BitDepth)
	assertEqualsInt(t, "AudioTracks", 2, p.AudioTracks)
	assertEquals(t, "AudioTechnology", "Atmos", p.AudioTechnology)
	assertEquals(t, "MainTitle", "Movie", p.MainTitle)
}

func TestMergeMediaInfoInto_NilSafe(t *testing.T) {
	p := ParseTitleTech("Movie 2024 1080p x264")
	MergeMediaInfoInto(&p, nil) // 不 panic，不修改
	assertEquals(t, "Resolution", "1080p", p.Resolution)

	var nilP *TechProfile
	MergeMediaInfoInto(nilP, &MediaInfoTech{Resolution: "2160p"}) // 不 panic
}

// §59.76: v1.05 资产四断点——REPACK 合并/MoC WAC pattern/音轨数/分发方展示(3b 前端)
func TestEditionMergeReleaseVersion(t *testing.T) {
	p := BuildTechProfile("Movie.2024.REPACK.1080p.BluRay.x264-GROUP", "", "", "", "", "")
	if p.EditionInfo != "REPACK" {
		t.Errorf("REPACK 应合并进 EditionInfo: %q", p.EditionInfo)
	}
	p2 := BuildTechProfile("Film.2020.IMAX.REPACK.2160p", "", "", "", "", "")
	if p2.EditionInfo != "IMAX REPACK" && p2.EditionInfo != "IMAX" {
		t.Errorf("已有 Edition 时不被覆盖: %q", p2.EditionInfo)
	}
}

func TestEditionPublisherBrands(t *testing.T) {
	for _, c := range []struct{ title, want string }{
		{"Film.2020.MoC.2160p.Blu-ray", "MoC"},
		{"Movie.2019.WAC.1080p.BluRay", "WAC"},
	} {
		p := BuildTechProfile(c.title, "", "", "", "", "")
		if p.EditionInfo != c.want {
			t.Errorf("%s: EditionInfo=%q want %q", c.title, p.EditionInfo, c.want)
		}
	}
}

// §59.77: 评论音轨从副标题提取扣减——v1.05 "评论音轨不计入音轨数"。
// 243 实证形态: 评论音轨(单)/双评论音轨/三评论音轨/双评论音轨带字幕。
func TestAdjustCommentaryTracks(t *testing.T) {
	cases := []struct {
		name     string
		mi, want int
		sub      string
	}{
		{"单条", 3, 2, "英语 评论音轨 简繁英字幕"},
		{"双", 4, 2, "双评论音轨 带章节名"},
		{"三", 5, 2, "英语 三评论音轨 简繁英双语字幕"},
		{"带字幕变体", 3, 1, "双评论音轨带字幕"},
		{"无评论轨", 3, 3, "英语 简繁英双语字幕"},
		{"MI=1 防御(单轨不可能有评论轨)", 1, 1, "评论音轨"},
		{"扣到负钳 0", 2, 0, "三评论音轨"},
		{"MI=0 不动", 0, 0, "评论音轨"},
	}
	for _, c := range cases {
		if got := AdjustCommentaryTracks(c.mi, c.sub); got != c.want {
			t.Errorf("%s: AdjustCommentaryTracks(%d, %q) = %d, want %d", c.name, c.mi, c.sub, got, c.want)
		}
	}
}
