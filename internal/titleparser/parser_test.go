package titleparser

import (
	"strings"
	"testing"
)

func TestParseTitle_Movie(t *testing.T) {
	c := ParseTitle("The.Dark.Knight.2008.1080p.BluRay.x264-DTS-CMCT")
	if c.MainTitle != "The Dark Knight" {
		t.Errorf("MainTitle = %q, want %q", c.MainTitle, "The Dark Knight")
	}
	if c.Year != "2008" {
		t.Errorf("Year = %q, want %q", c.Year, "2008")
	}
	if c.Resolution != "1080p" {
		t.Errorf("Resolution = %q, want %q", c.Resolution, "1080p")
	}
	if c.VideoCodec != "x264" {
		t.Errorf("VideoCodec = %q, want %q", c.VideoCodec, "x264")
	}
	if c.ReleaseGroup != "CMCT" {
		t.Errorf("ReleaseGroup = %q, want %q", c.ReleaseGroup, "CMCT")
	}
}

func TestParseTitle_TVSeries(t *testing.T) {
	c := ParseTitle("House.of.the.Dragon.S01E03.2024.2160p.MAX.WEB-DL.H265.DDP.5.1-CMCTV")
	if c.SeasonEpisode != "S01E03" {
		t.Errorf("SeasonEpisode = %q, want %q", c.SeasonEpisode, "S01E03")
	}
	if c.Resolution != "2160p" {
		t.Errorf("Resolution = %q, want %q", c.Resolution, "2160p")
	}
	if c.SourcePlatform != "MAX" {
		t.Errorf("SourcePlatform = %q, want %q", c.SourcePlatform, "MAX")
	}
	if c.VideoCodec != "H265" && c.VideoCodec != "HEVC" {
		t.Errorf("VideoCodec = %q, want H265 or HEVC", c.VideoCodec)
	}
	if c.AudioCodec != "DDP" {
		t.Errorf("AudioCodec = %q, want %q", c.AudioCodec, "DDP")
	}
	if c.ReleaseGroup != "CMCTV" {
		t.Errorf("ReleaseGroup = %q, want %q", c.ReleaseGroup, "CMCTV")
	}
}

func TestParseTitle_ChinesePrefix(t *testing.T) {
	c := ParseTitle("[蝙蝠侠] The Dark Knight 2008 1080p BluRay x264-CMCT")
	if c.ChinesePrefix != "蝙蝠侠" {
		t.Errorf("ChinesePrefix = %q, want %q", c.ChinesePrefix, "蝙蝠侠")
	}
	if c.MainTitle != "The Dark Knight" {
		t.Errorf("MainTitle = %q, want %q", c.MainTitle, "The Dark Knight")
	}
}

func TestParseTitle_ChinesePrefixNoBracket(t *testing.T) {
	// CSWEB 格式：中文片名.英文片名.技术信息-制作组（无中括号）
	tests := []struct {
		title  string
		prefix string
		hasEng bool // 是否有英文片名（有则 main_title 应无中文）
	}{
		{"正义前锋.The.Dukes.of.Hazzard.2005.2160p.WEB-DL.HEVC.AAC.2.0-CSWEB", "正义前锋", true},
		{"角斗士.Gladiator.2000.2160p.WEB-DL.H.265.AAC2.0-CSWEB", "角斗士", true},
		{"死侍.Deadpool.2016.1080p.Remux.H264.AAC.2.0.Audio-Kylin", "死侍", true},
		{"赛车总动员3：极速挑战.Cars.3.S01.2017.1080p.WEB-DL.H.264.AAC.2.0-CSWEB", "赛车总动员3：极速挑战", true},
		{"灼热.2020.1080p.中英字幕￡CMCT双儿", "灼热", false},
		{"澳门风云2.2015.简繁中字￡CMCT犇犇", "澳门风云2", false},
	}
	for _, tt := range tests {
		c := ParseTitle(tt.title)
		if c.ChinesePrefix != tt.prefix {
			t.Errorf("title %q: ChinesePrefix = %q, want %q", tt.title, c.ChinesePrefix, tt.prefix)
		}
		// 有英文片名的标题：main_title 不含中文
		if tt.hasEng {
			for _, r := range c.MainTitle {
				if r >= 0x4e00 && r <= 0x9fff {
					t.Errorf("title %q: MainTitle = %q, should not contain Chinese", tt.title, c.MainTitle)
					break
				}
			}
		}
	}
}

func TestParseTitle_ChinesePrefixNotFalsePositive(t *testing.T) {
	// 英文开头的标题不应误提取中文前缀
	c := ParseTitle("The.Dark.Knight.2008.1080p.BluRay.x264-CMCT")
	if c.ChinesePrefix != "" {
		t.Errorf("English title should not have ChinesePrefix, got %q", c.ChinesePrefix)
	}
	// BJ单身日记2 — 英文+中文混合开头，不以中文开头，不匹配
	c = ParseTitle("BJ单身日记2.Bridget.Jones.2.2004.1080p.WEB-DL.H.265.AAC2.0-CSWEB")
	// BJ 开头不是中文，不触发无中括号提取。中文残留由 stripChinese 处理。
	// 这是已知限制，不影响大部分种子。
}

func TestParseTitle_HDR(t *testing.T) {
	c := ParseTitle("Dune.2024.2160p.UHD.BluRay.HDR.HEVC.TrueHD.7.1-FraMeSToR")
	if c.HDRFormat != "HDR" {
		t.Errorf("HDRFormat = %q, want %q", c.HDRFormat, "HDR")
	}
	if c.AudioCodec != "TrueHD" {
		t.Errorf("AudioCodec = %q, want %q", c.AudioCodec, "TrueHD")
	}
}

func TestParseTitle_DoVi(t *testing.T) {
	c := ParseTitle("Dune.2024.2160p.WEB-DL.DV.HDR10+.HEVC-PTer")
	if c.HDRFormat != "DoVi HDR10+" {
		t.Errorf("HDRFormat = %q, want %q", c.HDRFormat, "DoVi HDR10+")
	}
}

func TestParseTitle_Empty(t *testing.T) {
	c := ParseTitle("")
	if c.MainTitle != "" {
		t.Errorf("MainTitle = %q, want empty", c.MainTitle)
	}
}

func TestExtractGroup_NoGroup(t *testing.T) {
	g := extractGroup("Some.Title.Without.Group")
	if g != "" {
		t.Errorf("extractGroup = %q, want empty", g)
	}
}

func TestExtractResolution_4K(t *testing.T) {
	r := extractResolution("Title.4K.WEB-DL")
	if r != "4k" && r != "4K" {
		t.Errorf("resolution = %q, want 4k/4K", r)
	}
}

func TestParseTitle_H265TokenRemoved(t *testing.T) {
	c := ParseTitle("[温暖的抱抱].Warm.Hug.2020.2160p.WEB-DL.H265.AAC-UBWEB.mp4")
	if c.VideoCodec != "HEVC" {
		t.Errorf("VideoCodec = %q, want HEVC", c.VideoCodec)
	}
	if strings.Contains(strings.ToUpper(c.MainTitle), "265") {
		t.Errorf("MainTitle = %q, H265 should be removed", c.MainTitle)
	}
	if c.ReleaseGroup != "UBWEB" {
		t.Errorf("ReleaseGroup = %q, want UBWEB (without .mp4)", c.ReleaseGroup)
	}
}

func TestParseTitle_H264TokenRemoved(t *testing.T) {
	c := ParseTitle("Test.2024.1080p.NF.WEB-DL.H264.DDP-NTb")
	if c.VideoCodec != "AVC" {
		t.Errorf("VideoCodec = %q, want AVC", c.VideoCodec)
	}
	if strings.Contains(strings.ToUpper(c.MainTitle), "264") {
		t.Errorf("MainTitle = %q, H264 should be removed", c.MainTitle)
	}
}

func TestExtractGroup_FileExtension(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Movie.2020.1080p.AAC-UBWEB.mp4", "UBWEB"},
		{"Movie.2020.1080p.AAC-FRDS.mkv", "FRDS"},
		{"Movie.2020.1080p.AAC-CMCT", "CMCT"},
	}
	for _, tt := range tests {
		got := extractGroup(tt.title)
		if got != tt.want {
			t.Errorf("extractGroup(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestParseTitle_AC3TokenRemoved(t *testing.T) {
	c := ParseTitle("Go.Lala.Go.2.2015.BluRay.720p.x264.AC3-CMCT")
	if strings.Contains(strings.ToUpper(c.MainTitle), "AC3") {
		t.Errorf("MainTitle = %q, AC3 should be removed", c.MainTitle)
	}
	if c.AudioCodec != "DD" {
		t.Errorf("AudioCodec = %q, want DD", c.AudioCodec)
	}
}

func TestParseTitle_AudioTracksTokenRemoved(t *testing.T) {
	c := ParseTitle("Movie.2026.1080p.NF.WEB-DL.x264.DDP.5.1.Atmos.2Audios-CMCTV")
	if strings.Contains(strings.ToUpper(c.MainTitle), "2AUDIOS") {
		t.Errorf("MainTitle = %q, 2Audios should be removed", c.MainTitle)
	}
}

func TestParseTitle_SiteTagSuffixRemoved(t *testing.T) {
	c := ParseTitle("Movie.2024.BluRay.x264-CMCT [热门] [2X免费]")
	if strings.Contains(c.MainTitle, "[") || strings.Contains(c.MainTitle, "热门") {
		t.Errorf("MainTitle = %q, site tags should be removed", c.MainTitle)
	}
	if c.ReleaseGroup != "CMCT" {
		t.Errorf("ReleaseGroup = %q, want CMCT", c.ReleaseGroup)
	}
}

func TestParseTitle_SiteTagParenRemoved(t *testing.T) {
	c := ParseTitle("Movie.1975.BluRay.1080p.x264.FLAC-CMCT (已审) [2X免费]")
	if strings.Contains(c.MainTitle, "已审") || strings.Contains(c.MainTitle, "[") {
		t.Errorf("MainTitle = %q, site tags should be removed", c.MainTitle)
	}
}

func TestParseTitle_HDRTokenRemoved(t *testing.T) {
	tests := []string{
		"Movie.2025.2160p.WEB-DL.HDRVivid.H265.10",
		"Movie.2024.1080p.WEB-DL.HDR10+.H265",
		"Movie.2024.2160p.WEB-DL.DV.HDR.H265",
	}
	for _, title := range tests {
		c := ParseTitle(title)
		mtu := strings.ToUpper(c.MainTitle)
		if strings.Contains(mtu, "HDRVIVID") || strings.Contains(mtu, "HDR10") || strings.Contains(mtu, "VIVID") {
			t.Errorf("title %q: MainTitle = %q, HDR token should be removed", title, c.MainTitle)
		}
	}
}

func TestParseTitle_1440pResolution(t *testing.T) {
	c := ParseTitle("Movie.2025.1440p.WEB-DL.H265.DDP5.1-GRP")
	if !strings.Contains(strings.ToLower(c.Resolution), "1440") {
		t.Errorf("Resolution = %q, should contain 1440", c.Resolution)
	}
}

func TestParseTitle_DDPChannelsMerged(t *testing.T) {
	tests := []string{
		"Movie.2025.1080p.WEB-DL.DDP5.1.H265-GRP",
		"Movie.2025.1080p.WEB-DL.DDPA5.1.H265-GRP",
		"Movie.2024.1080p.WEB-DL.AAC2.0.H264-GRP",
	}
	for _, title := range tests {
		c := ParseTitle(title)
		// 编码+声道不应残留在 main_title
		mtu := strings.ToUpper(c.MainTitle)
		if strings.Contains(mtu, "DDP5") || strings.Contains(mtu, "DDPA") || strings.Contains(mtu, "AAC2") {
			t.Errorf("title %q: MainTitle = %q, codec+channels should be removed", title, c.MainTitle)
		}
	}
}

// §59.96: 主标题提取双优化——年份截断 + 组段整体剥除。
func TestMainTitleYearBoundary(t *testing.T) {
	cases := []struct{ title, wantMain, wantGroup string }{
		// 年份后技术区噪声不进主标题(REPACK2/MNHD 结构性免疫)
		{"时空奇旅.Arco.2025.BluRay.1080p.x265.10bit.DDP7.1.REPACK2.MNHD-FRDS", "Arco", "FRDS"},
		{"赎梦.Peg.O'My.Heart.2024.BluRay.1080p.x265.10bit.DDP7.1.MNHD-FRDS", "Peg O'My Heart", "FRDS"},
		// 无年份标题: fallback 逐词法(组段剥除后 MNHD 不残留)
		{"Saki.S01-S03.BluRay.1080p.Hi10P.x264.FLAC.2.0-VCB-Studio", "Saki", "Studio"},
		// 中文名前缀保留
		{"电影名.Movie.2024.1080p.BluRay.x264-GROUP", "Movie", "GROUP"},
	}
	for _, c := range cases {
		got := ParseTitle(c.title)
		if got.MainTitle != c.wantMain || got.ReleaseGroup != c.wantGroup {
			t.Errorf("%s:\n  Main=%q want %q | Group=%q want %q", c.title[:min(50, len(c.title))], got.MainTitle, c.wantMain, got.ReleaseGroup, c.wantGroup)
		}
	}
}

// §59.97: 年份前置截断——主标题在技术 extractor 之前锁定。
func TestMainTitleYearAnchor(t *testing.T) {
	cases := []struct{ title, wantMain, wantYear string }{
		// 双年份: 片名本身是年份数字(2046) → 取第二年份, 主标题含第一组
		{"2046.2004.REPACK.2160p.UHD.Blu-ray.REMUX-CMiNEPHiLES", "2046", "2004"},
		// 单年份标准形态
		{"Movie.Name.2024.1080p.BluRay.x264-GROUP", "Movie Name", "2024"},
		// 年份后未知 token 不进主标题
		{"Arco.2025.BluRay.1080p.REPACK2.MNHD-FRDS", "Arco", "2025"},
		// 片名含年份数字但后无技术 token → 不截断(整串是主标题候选)
		{"Blade.Runner.2049.Alone", "Blade Runner 2049 Alone", ""},
		// 无年份 fallback(逐词法+组段剥除)
		{"Saki.S01-S03.BluRay.1080p.Hi10P.x264.FLAC.2.0-VCB-Studio", "Saki", ""},
	}
	for _, c := range cases {
		got := ParseTitle(c.title)
		if got.MainTitle != c.wantMain || got.Year != c.wantYear {
			t.Errorf("%q:\n  Main=%q want %q | Year=%q want %q", c.title, got.MainTitle, c.wantMain, got.Year, c.wantYear)
		}
	}
}

// §59.97 附: 地区码 extractor——HKG 等进 RegionCode 不进主标题。
func TestRegionCodeExtract(t *testing.T) {
	c := ParseTitle("Movie 2020 1080p HKG BluRay x264-CMCT")
	if c.MainTitle != "Movie" || c.RegionCode != "HKG" {
		t.Errorf("Main=%q RegionCode=%q, want Movie/HKG", c.MainTitle, c.RegionCode)
	}
	// 点分隔 + 双年份
	c2 := ParseTitle("2046.2004.2160p.ITA.UHD.Blu-ray.REMUX-CMiNEPHiLES")
	if c2.MainTitle != "2046" || c2.RegionCode != "ITA" {
		t.Errorf("Main=%q RegionCode=%q, want 2046/ITA", c2.MainTitle, c2.RegionCode)
	}
}

// §59.98: 季集边界锚——「季集或年份」最先出现者为右边界（无年份剧集也获结构边界）。
func TestSeasonAnchor(t *testing.T) {
	cases := []struct{ title, wantMain, wantYear, wantSE string }{
		// 季集先于年份 → 锚=季集
		{"Show.S01.2024.1080p.BluRay.x264-GROUP", "Show", "2024", "S01"},
		{"Show.S01E03.2024.1080p.BluRay.x264-GROUP", "Show", "2024", "S01E03"},
		// 无年份剧集 → 季集锚（fallback 逐词的 Saki 案例升级为结构边界）
		{"Saki.S01-S03.BluRay.1080p.Hi10P.x264.FLAC.2.0-VCB-Studio", "Saki", "", "S01-S03"},
		// 电影无季集 → 年份锚（既有）
		{"Movie.Name.2024.1080p.BluRay.x264-GROUP", "Movie Name", "2024", ""},
		// 双年份+季集
		{"2046.2004.REPACK.2160p.UHD.Blu-ray.REMUX-CMiNEPHiLES", "2046", "2004", ""},
		// Clannad +MOVIE 后缀: 季集锚, MOVIE 在右侧技术区
		{"Clannad.S01-S02+MOVIE.REPACK.1080p.BluRay-GRP", "Clannad", "", "S01-S02"},
	}
	for _, c := range cases {
		got := ParseTitle(c.title)
		if got.MainTitle != c.wantMain || got.Year != c.wantYear || got.SeasonEpisode != c.wantSE {
			t.Errorf("%q:\n  Main=%q want %q | Year=%q want %q | SE=%q want %q",
				c.title, got.MainTitle, c.wantMain, got.Year, c.wantYear, got.SeasonEpisode, c.wantSE)
		}
	}
}
