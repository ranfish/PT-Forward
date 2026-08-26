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
		miText   string
	}{
		{"单条", 3, 2, "英语 评论音轨 简繁英字幕", ""},
		{"双", 4, 2, "双评论音轨 带章节名", ""},
		{"三", 5, 2, "英语 三评论音轨 简繁英双语字幕", ""},
		{"带字幕变体", 3, 1, "双评论音轨带字幕", ""},
		{"无评论轨", 3, 3, "英语 简繁英双语字幕", ""},
		{"MI=1 防御(单轨不可能有评论轨)", 1, 1, "评论音轨", ""},
		{"扣到负钳 0", 2, 0, "三评论音轨", ""},
		{"MI=0 不动", 0, 0, "评论音轨", ""},
		// §59.114: MI Title 英文 Commentary 信号（绝地计划实锤——两行 Commentary 扣 2）
		{"MI Commentary 单行", 3, 2, "", "Audio #1\nTitle : English\nAudio #2\nTitle : Commentary by film critics"},
		{"MI Commentary 双行", 4, 2, "", "Audio #1\nTitle : English\nAudio #2\nTitle : Commentary by film critics\nAudio #3\nTitle : Commentary by production"},
		{"两源取大不叠加", 3, 1, "双评论音轨", "Audio #1\nTitle : English\nAudio #2\nTitle : Commentary"},
	}
	for _, c := range cases {
		if got := AdjustCommentaryTracks(c.mi, c.sub, c.miText); got != c.want {
			t.Errorf("%s: AdjustCommentaryTracks(%d, %q, mi) = %d, want %d", c.name, c.mi, c.sub, got, c.want)
		}
	}
}

// §59.84: 媒介写法区分三缺口——3D 点分隔/HDDVDRip/DVD 原盘。
func TestMediumThreeGaps(t *testing.T) {
	cases := []struct {
		title        string
		wantST, wantSpec string
	}{
		// 1. 3D 点分隔: preferredBlurayToken 只认 "3D BLU"(空格)/"3DBLU"(无分隔), 点分隔 "3D.Blu-ray" 落入普通 BLU
		{"Movie.3D.Blu-ray.1080p.AVC", "3D Blu-ray", ""},
		{"Movie.2024.3D.BluRay.1080p.x264", "3D BluRay", ""},
		// 2. HDDVDRip: extractMedium/splitMedium 双双无 HDDVD case(v1.05 压制类明列)
		{"Movie.2024.1080p.HDDVDRip.x264", "HD DVD", "HDDVDRip"},
		// 3. DVD 原盘: reDVDDiscToken 只认 DVD5/DVD9, 裸 "DVD" 不命中(v1.05 原盘类)
		{"Movie.2024.DVD.Full", "DVD", ""},
		// 既有行为回归锚
		{"Movie.2024.UHD.BluRay.2160p.x265", "UHD BluRay", ""},
		{"Movie.2024.DVDRip", "DVD", "DVDRip"},
	}
	for _, c := range cases {
		p := BuildTechProfile(c.title, "", "", "", "", "")
		if p.SourceType != c.wantST || p.Specification != c.wantSpec {
			t.Errorf("%s: ST=%q Spec=%q, want ST=%q Spec=%q", c.title, p.SourceType, p.Specification, c.wantST, c.wantSpec)
		}
	}
}

// §59.116: 评论扣减 MI 信号限定 Audio 段——Text(字幕轨)的 Commentary 配套字幕
// 不是评论音轨（天堂里的烦恼 2-2=0 误扣实锤）。
func TestAdjustCommentaryTracksTextNotCounted(t *testing.T) {
	mi := `Audio #1
Format : FLAC
Title : English
Audio #2
Format : AAC LC
Title : Commentary by Lubitsch biographer Scott Eyman
Text #6
Format : UTF-8
Title : Commentary by Lubitsch biographer Scott Eyman`
	if got := AdjustCommentaryTracks(2, "", mi); got != 1 {
		t.Errorf("字幕 Commentary 不应扣: got %d want 1", got)
	}
}
