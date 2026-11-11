package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.514 测试锚：.torrent 证据仲裁四案例（优堡拒收实战素材）。

func arbTestMeta(title, subtitle string) *model.TorrentMetadata {
	return &model.TorrentMetadata{
		InfoHash: "testhash0000000000000000000000000000",
		Title:    title,
		Subtitle: subtitle,
		Category: "category.tv_series",
	}
}

// 案例① Quacks：标题裸 E01（站方改坏）+ name S01 + 文件树 E01-E06 六文件
// → 合集修正 S01（name 权威+文件树区间双证据）
func TestArbitrateQuacks(t *testing.T) {
	e := &PublishExecutor{}
	meta := arbTestMeta("Quacks E01 2017 1080p AMZN WEBRip HEVC DDP 2.0-cXcY@FRDS",
		"江湖郎中 / 怪诞医生 | 全6集 | 主演：罗里·金尼尔 | 喜剧 | [英语] | [简中/繁中/英]")
	tp := titleparser.BuildTechProfile(meta.Title, "", "", "", "", "")
	if tp.SeasonEpisode != "E01" || tp.MainTitle != "Quacks" {
		t.Fatalf("前置: SE=%q MainTitle=%q（裸 Exx 提取失败）", tp.SeasonEpisode, tp.MainTitle)
	}
	notes := e.arbitrateFileTree(&tp, map[string]int64{
		"Quacks.E01.2017.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 1600,
		"Quacks.E02.2017.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 1600,
		"Quacks.E03.2017.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 1600,
		"Quacks.E04.2017.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 1600,
		"Quacks.E05.2017.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 1600,
		"Quacks.E06.2017.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 1600,
		"Quacks_文档.txt": 1,
	})
	if tp.SeasonEpisode != "S01" {
		t.Errorf("Quacks 修正后 SE=%q want S01（notes=%v）", tp.SeasonEpisode, notes)
	}
	// §59.424 门：归一后 S01 纯季放行（此前裸 E01 走无信号豁免误发）
	if blocked, _ := episodeReleaseBlocked(meta, &tp); blocked {
		t.Error("归一 S01 后应放行")
	}
}

// 案例② Time：标题 S01 2021（漏标）+ name S01-S02 2021-2023 + 文件树两季
// → 跨季构造 S01-S02+年份区间 2021-2023
func TestArbitrateTime(t *testing.T) {
	e := &PublishExecutor{}
	meta := arbTestMeta("Time S01 2021 1080p AMZN WEBRip HEVC DDP 5.1-cXcY@FRDS",
		"警与囚 第一季 | 全3集")
	tp := titleparser.BuildTechProfile(meta.Title, "", "", "", "", "")
	if tp.SeasonEpisode != "S01" {
		t.Fatalf("前置: SE=%q", tp.SeasonEpisode)
	}
	np := titleparser.BuildTechProfile("日日夜夜_警与囚.S01-S02.Time.2021-2023.1080p.AMZN.WEBrip.x265.AC3￡cXcY@FRDS", "", "", "", "", "")
	if np.SeasonEpisode != "S01-S02" {
		t.Errorf("name SE=%q want S01-S02", np.SeasonEpisode)
	}
	if np.Year != "2021-2023" {
		t.Errorf("name Year=%q want 2021-2023（年份区间锚）", np.Year)
	}
	notes := e.arbitrateFileTree(&tp, map[string]int64{
		"Time.S01E01.2021.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 2700,
		"Time.S01E02.2021.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 2700,
		"Time.S01E03.2021.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 2700,
		"Time.S02E01.2023.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 2700,
		"Time.S02E02.2023.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 2700,
		"Time.S02E03.2023.1080p.AMZN.WEBrip.x265.10bit.AC3￡cXcY@FRDS.mkv": 2700,
	})
	if tp.SeasonEpisode != "S01-S02" {
		t.Errorf("Time 跨季构造后 SE=%q want S01-S02（notes=%v）", tp.SeasonEpisode, notes)
	}
	if tp.Year != "2021-2023" {
		t.Errorf("Time 年份区间=%q want 2021-2023", tp.Year)
	}
	// 重组终态
	rt := titleparser.ReassembleFromTechProfile(tp, titleparser.V105TitleFormat())
	if !strings.Contains(rt, "S01-S02") || !strings.Contains(rt, "2021-2023") {
		t.Errorf("重组缺跨季/年区间: %q", rt)
	}
}

// 案例③ From：标题 WEBRip vs name WEB-DL → 规格仲裁修正（站方拒收实证
// WEB-DL≠WEBRip——发布层严格不等）
func TestArbitrateFrom(t *testing.T) {
	e := &PublishExecutor{}
	meta := arbTestMeta("From S04 2026 1080p WEBRip DDP5.1 x265 10bit-Yumi@FRDS", "第四季")
	tp := titleparser.BuildTechProfile(meta.Title, "", "", "", "", "")
	notes := e.arbitrateNameTokens(meta, &tp, "From.S04.2026.1080p.WEB-DL.DDP5.1.x265.10bit-Yumi@FRDS")
	if !strings.EqualFold(tp.Specification, "WEB-DL") {
		t.Errorf("From 规格仲裁后 Spec=%q want WEB-DL（notes=%v）", tp.Specification, notes)
	}
}

// 案例④ Railroad：标题无品牌 + name CC → 品牌补全（CC 归一值——§59.514
// 三 pattern 统一）
func TestArbitrateRailroadCC(t *testing.T) {
	e := &PublishExecutor{}
	meta := arbTestMeta("The Underground Railroad E01 2021 1080p BluRay HEVC DTS 5.1 2Audios-cXcY@FRDS", "全10集")
	tp := titleparser.BuildTechProfile(meta.Title, "", "", "", "", "")
	notes := e.arbitrateNameTokens(meta, &tp, "地下铁道S01.The.Underground.Railroad.2021.1080p.CC..Blu-ray.x265.DTS￡cXcY@FRDS")
	if tp.EditionInfo != "CC" {
		t.Errorf("品牌补全=%q want CC（notes=%v）", tp.EditionInfo, notes)
	}
}

// CC 归一统一：三词形（Criterion Collection/Criterion/CC）→ 同值 "CC"
// （§59.207 品牌反驳链同品牌不同串假杀隐患消除）
func TestCCNormalizedSingleValue(t *testing.T) {
	for _, title := range []string{
		"The Hit 1984 Criterion Collection BluRay 1080p x264 LPCM 1.0-LK@UBits",
		"Repo Man 1984 Criterion BluRay 1080p x264-LK@UBits",
		"Repo Man 1984 CC BluRay 1080p x264-LK@UBits",
	} {
		tp := titleparser.BuildTechProfile(title, "", "", "", "", "")
		if tp.EditionInfo != "CC" {
			t.Errorf("%q EditionInfo=%q want CC", title, tp.EditionInfo)
		}
	}
}

// Book of Negroes 型：裸 E01+副标全六集（通道 C）→ 归一 S01；无信号裸 E01 →
// §59.424 拦（真分集）
func TestBareE01GateAndNormalize(t *testing.T) {
	// 有信号（副标全六集）→ 归一 S01 放行
	meta := arbTestMeta("The Book Of Negroes E01 2015 1080p WEBRip HEVC DD 5.1-cXcY@FRDS",
		"【黑人之书】迷你剧 全六集 | 中英字幕")
	tp := titleparser.BuildTechProfile(meta.Title, "", "", "", "", "")
	e := &PublishExecutor{}
	e.normalizeSeasonPackOf(meta, &tp)
	if tp.SeasonEpisode != "S01" {
		t.Errorf("通道 C 归一=%q want S01", tp.SeasonEpisode)
	}
	// 无信号 → 门拦（§59.424 裸形态识别——此前走无 S/E 信号豁免漏拦）
	meta2 := arbTestMeta("Some.Show E01 2020 1080p WEBRip x264-GRP", "主演：X | 剧情")
	tp2 := titleparser.BuildTechProfile(meta2.Title, "", "", "", "", "")
	if tp2.SeasonEpisode != "E01" {
		t.Fatalf("前置: SE=%q", tp2.SeasonEpisode)
	}
	if blocked, _ := episodeReleaseBlocked(meta2, &tp2); !blocked {
		t.Error("无信号裸 E01 应被 §59.424 拦截")
	}
	// BBC 单剧（E01+eps=1）：裸单集归一排除（§59.424 定案不禁亦不归一）
	tp3 := titleparser.BuildTechProfile("BBC One Show E01 2019 1080p WEBRip x264-GRP", "", "", "", "", "")
	titleparser.MergePTGenInto(&tp3, titleparser.PTGenMergeInput{Year: "2019"}, func(string, ...any) {})
	if got := titleparser.NormalizeSeasonPack(tp3.SeasonEpisode, true, 1); got != "E01" {
		t.Errorf("BBC 单剧归一=%q want E01（不归一）", got)
	}
}

