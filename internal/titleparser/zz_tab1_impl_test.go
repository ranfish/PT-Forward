package titleparser

import (
	"github.com/ranfish/pt-forward/internal/util"
	"testing"
)

// §59.226 附八: DIY 组名族——媒介正则收窄+伪复合剥离
func TestDIYGroupExtraction(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Batman.Mystery.of.the.Batwoman.2003.USA.Blu-ray.1080p.AVC.DTS-HD.MA.5.1-DIY@CMCT", "CMCT"},
		{"宾虚.Ben-Hur.1959.2160p.UHD.Blu-ray.DoVi.HDR10.HEVC.Atmos.TrueHD.7.1-DIY@UBits", "UBits"},
		{"Wanted.2008.Blu-Ray.1080p.AVC.DTS-HD.5.1-DIY-cqkyjj2", "cqkyjj2"},
		{"Movie.2023.1080p.BluRay.x264-SHB931@UBWEB", "SHB931@UBWEB"}, // 真复合整段
	}
	for _, c := range cases {
		if got := ParseTitle(c.title).ReleaseGroup; got != c.want {
			t.Errorf("group(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

// §59.226 附六: ST 反哺——平台词当源形态
func TestPlatformBackfeedST(t *testing.T) {
	// AMZN 无 requires 标记（NF requires=web 需 WEB 词激活——反哺受益者
	// 是无 requires 平台词）
	p := ParseTitleTech("Movie.2023.1080p.AMZN.DDP5.1.H.264-GROUP")
	if p.SourceType != "WEB" {
		t.Errorf("ST = %q, want WEB（platform=AMZN 反哺）", p.SourceType)
	}
	if p.SourcePlatform != "AMZN" {
		t.Errorf("platform = %q, want AMZN", p.SourcePlatform)
	}
}

// §59.226 附二十一: 帧率 MI 提取+非默认判定链
func TestFrameRateFromMI(t *testing.T) {
	cases := []struct{ in, want string }{
		{"23.976 FPS", "23.976"},
		{"60.000 FPS", "60.000"},
		{"24.000 FPS", "24.000"},
		{"", ""},
		{"29.97", "29.97"},
	}
	for _, c := range cases {
		if got := frameRateFromMI(c.in); got != c.want {
			t.Errorf("frameRateFromMI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// §59.226 附四: MediumCanonicalOf 纯函数
func TestMediumCanonicalOf(t *testing.T) {
	cases := []struct {
		p    TechProfile
		want string
	}{
		{TechProfile{Specification: "Remux", Resolution: "1080p"}, "Remux"},
		{TechProfile{Specification: "Remux", Resolution: "2160p"}, "UHD Remux"},
		{TechProfile{Specification: "WEB-DL"}, "WEB-DL"},
		{TechProfile{Specification: "WEBRip"}, "Encode"}, // §59.166 From S04
		{TechProfile{Specification: "BDRip"}, "Encode"},
	{TechProfile{SourceType: "Blu-ray"}, "Blu-ray 原盘"},
	{TechProfile{SourceType: "BluRay"}, "Encode"},
	// §59.319 附三十二: 连字符原盘分支重编码排除（雀起乡 Blu-ray.x265 案）
	// 完整路径（ParseTitleTech 填 RawTitle）+ mini profile 形态（DB 列 VideoCodec=HEVC 归一化丢 x265）
	{TechProfile{SourceType: "Blu-ray", RawTitle: "雀起乡到烛镇S01-S04.Lark.Rise.to.Candleford.2008-2011.1080p.Blu-ray.x265.AC3￡cXcY@FRDS"}, "Encode"},
	{TechProfile{SourceType: "UHD Blu-ray", RawTitle: "Movie 2026 2160p UHD Blu-ray x264"}, "Encode"},
	{TechProfile{SourceType: "Blu-ray", VideoCodec: "HEVC", RawTitle: "Movie 1080p Blu-ray x265 DDP5.1 GRP"}, "Encode"},
		{TechProfile{SourceType: "DVD"}, "DVD"},
	}
	for _, c := range cases {
		if got := MediumCanonicalOf(c.p); got != c.want {
			t.Errorf("MediumCanonicalOf(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}

// §59.226 附三: 中文集数解析
func TestCNEpisode(t *testing.T) {
	tc := ParseTitle("大江大河之岁月如歌.第01-24集.2024.1080p.WEB-DL.H264-GROUP")
	if tc.SeasonEpisode != "E01-E24" {
		t.Errorf("SeasonEpisode = %q, want E01-E24", tc.SeasonEpisode)
	}
	tc2 := ParseTitle("BBC王朝全集.全五集.2018.720p-GROUP")
	if tc2.SeasonEpisode != "E01-E05" {
		t.Errorf("全五集: SeasonEpisode = %q, want E01-E05", tc2.SeasonEpisode)
	}
	tc3 := ParseTitle("剧名.全十二集.2020.1080p-GROUP")
	if tc3.SeasonEpisode != "E01-E12" {
		t.Errorf("全十二集: SeasonEpisode = %q, want E01-E12", tc3.SeasonEpisode)
	}
}

// §59.232: region 剥除的 dash 锚保护——Ilo.Ilo 案（TWN 剥除后首部 "-CMCT"）
func TestRegionStripDashAnchor(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Ilo.Ilo.2013.TWN.BluRay.1080p.x264.DDP.5.1-CMCT", "CMCT"},
		{"Ilo.Ilo.2013.TWN.BluRay.1080p.x264.DDP.5.1-CMCT  (已审)", "CMCT"},
		{"Movie.2023.USA.BluRay.1080p.x264-GROUP", "GROUP"},
	}
	for _, c := range cases {
		if got := ParseTitle(c.title).ReleaseGroup; got != c.want {
			t.Errorf("group(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

// §59.233: 展示层 extractGroup v2（@ 锚原样优先+资产扫描+锚兜底）
func TestExtractGroupLexiconLayer(t *testing.T) {
	util.SetGroupLexicon([]string{"CMCT", "UBits", "UBWEB"})
	defer util.SetGroupLexicon(nil)

	cases := []struct{ title, want string }{
		// @ 真复合：整段原样（第 0 层——不依赖词表）
		{"Movie.2023.1080p-SHB931@UBWEB", "SHB931@UBWEB"},
		// @ 伪复合：DIY@ 剥离
		{"Movie.2023.1080p.x264.DDP.5.1-DIY@CMCT", "CMCT"},
		// 资产扫描：dash 被吃裸词（§59.232 场景——词表兜住）
		{"CMCT", "CMCT"},
		// 尾部约束：非尾部命中排除
		{"CMCT.Movie.2023.1080p-RealGRP", "RealGRP"},
	}
	for _, c := range cases {
		if got := ParseTitle(c.title).ReleaseGroup; got != c.want {
			t.Errorf("group(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

// §59.235 P1: composeMedium 源=规格合一去重（HDTV 不再 "HDTV HDTV"）
func TestComposeMediumDedup(t *testing.T) {
	if got := composeMedium(TechProfile{SourceType: "HDTV", Specification: "HDTV"}); got != "HDTV" {
		t.Errorf("合一形态: got %q, want HDTV", got)
	}
	if got := composeMedium(TechProfile{SourceType: "UHD Blu-ray", Specification: "Remux"}); got != "UHD Blu-ray Remux" {
		t.Errorf("组合形态: got %q", got)
	}
}

// §59.352 附: BDAV 容器门下线——原盘保护职责移交 FromBDInfo 优先级+本地互斥
func TestIsEncodeBDAVContainerGate(t *testing.T) {
	// §59.352 附语义更新：本地 BDAV MI 进本判定=单文件 m2ts 种（本地原盘
	// isDisc 互斥 MI 恒空不进此层；原盘保护由 FromBDInfo 结构级铁证接管）
	// ——x264 writing library=压制痕迹，判 Encode 正确。
	bdavMI := "General\nFormat           : BDAV\nMenu ID          : 1 (0x1)\n\nVideo\nFormat           : AVC\nWriting library  : x264 core 148\nWidth            : 1920 pixels\n"
	if mi := ExtractMediaInfo(bdavMI); !mi.Encoded {
		t.Errorf("§59.352 附：BDAV+x264 writing library 应判 Encode（容器门已下线）")
	}
	if mi := ExtractMediaInfo(bdavMI); mi.VideoCodec != "AVC" {
		t.Errorf("BDAV 样本 VideoCodec 应解析出 AVC，got %q（样本构造校验）", mi.VideoCodec)
	}
	// MKV 压制（§59.151 语料语义不变）
	mkvMI := "General\nFormat           : Matroska\n\nVideo\nFormat           : HEVC\nWriting library  : x265\nWidth            : 1920 pixels\n"
	if mi := ExtractMediaInfo(mkvMI); !mi.Encoded {
		t.Errorf("MKV x265 铁证应判 Encode")
	}
	// 原盘真保护锚（§59.352 附证据等级）：BDInfo 扫描在场 → FromBDInfo
	// 优先升格——BDAV 痕迹不翻转原盘词形
	p := BuildTechProfile("Movie 2012 1080p BluRay AVC-GROUP",
		"DISC INFO:\n\nDisc Title: X\nQUICK SUMMARY:\nVideo: AVC\n", "", "", "", "")
	if p.SourceType != "Blu-ray" {
		t.Errorf("FromBDInfo 优先级：原盘驼峰应升格连字符，got %q", p.SourceType)
	}
}
