package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

func parseTechForTest(title string) titleparser.TechProfile {
	return titleparser.ParseTitleTech(title)
}

// §59.184 size 显示等价 v3——Just Mercy 优堡实数锁边界。
func TestCompareSizeDisplayV3(t *testing.T) {
	const orphan = int64(72633544168) // 67.6452 GB → 显示 67.65
	gb := int64(1073741824)
	tb := int64(1099511627776)

	cases := []struct {
		name   string
		source int64
		cand   int64
		want   bool
	}{
		// GB 档窗口 ±5,368,709B（±0.005×1024³）
		{"Just Mercy tid=109323 站方解析值（差 5.09MB）窗内", orphan, 72638634393, true},
		{"Just Mercy tid=108076（差 5.38MB）窗外——站方显示 67.64", orphan, 72627896975, false},
		{"GB 档窗内边界（差恰 5,368,709）", orphan, orphan + 5368709, true},
		{"GB 档窗外边界（差 5,368,710）", orphan, orphan + 5368710, false},
		// 1MB 绝对地板（跨站 NFO 文件集微差 ~52KB，v0.0.484 案）
		{"MB 档小种 52KB 差（地板兜）", 647312640, 647265024, true},
		{"MB 档小种 1.5MB 差（地板外）", 647312640, 645759360, false},
		// TB 档（±5.37GB）
		{"TB 档窗内（差 3GB）", 2 * tb, 2*tb + 3*gb, true},
		{"TB 档窗外（差 6GB）", 2 * tb, 2*tb + 6*gb, false},
		// 非法值
		{"source<=0", 0, gb, false},
		{"cand<=0", gb, 0, false},
	}
	for _, c := range cases {
		if got := CompareSizeDisplay(c.source, c.cand); got != c.want {
			t.Errorf("%s: CompareSizeDisplay(%d,%d)=%v want %v", c.name, c.source, c.cand, got, c.want)
		}
	}
}

// §59.184 G2: ParseTitleTech 无括号 REPACK 合并到 EditionInfo（§59.76 意图兑现）。
func TestParseTitleTech_RepackMerge(t *testing.T) {
	cases := []struct {
		srcTitle    string // 源标题（无版本词）
		candTitle   string // 候选标题
		wantVersion bool   // techProfileVersionDefined 期望
	}{
		{"Just.Mercy.2019.2160p.UHD.Blu-ray.DoVi.HEVC.TrueHD.7.1.Atmos-DIY@UBits",
			"Just Mercy 2019 UHD BluRay 2160p REPACK DoVi HDR10 x265 10bit Atmos TrueHD 7.1-Ubits", true},
		{"Movie.2019.1080p.BluRay.x264-GRP", "Movie 2019 1080p BluRay PROPER x264-GRP", true},
		{"Movie.2019.1080p.BluRay.x264-GRP", "Movie 2019 1080p BluRay REPACK2 x264-GRP", true},
		// 双方都有版本 → 不触发
		{"Movie.2019.1080p.BluRay.REPACK.x264-GRP", "Movie 2019 1080p BluRay REPACK x264-GRP", false},
		// 源有候选无 → 不触发
		{"Movie.2019.1080p.BluRay.REPACK.x264-GRP", "Movie 2019 1080p BluRay x264-GRP", false},
		// 双方都无 → 不触发
		{"Movie.2019.1080p.BluRay.x264-GRP", "Movie 2019 1080p BluRay x264-GRP", false},
	}
	for _, c := range cases {
		src := parseTechForTest(c.srcTitle)
		got := techProfileVersionDefined(src, c.candTitle)
		if got != c.wantVersion {
			t.Errorf("versionDefined(src=%q, cand=%q)=%v want %v", c.srcTitle, c.candTitle, got, c.wantVersion)
		}
	}
}

// §59.184 三态主链——Just Mercy 优堡 8 候选全推演（§59.184 测试矩阵核心）。
func TestVerifyThreeState_JustMercy(t *testing.T) {
	orphanSize := int64(72633544168)
	srcTitle := "Just.Mercy.2019.2160p.UHD.Blu-ray.DoVi.HDR10.HEVC.TrueHD.7.1.Atmos-DIY@UBits"
	mk := func(id, title string, size int64) *model.SeedingSearchResult {
		return &model.SeedingSearchResult{TorrentID: id, Title: title, Size: size}
	}
	results := []*model.SeedingSearchResult{
		mk("211806", "Just Mercy 2019 2160p iTunes WEB-DL HDR10+ H265 10bit DDP5.1 Atmos-UBits", 25866440540),
		mk("113971", "Just Mercy 2019 UHD BluRay 2160p REPACK DoVi HDR10 x265 10bit Atmos TrueHD 7.1-Ubits", 27616639713),
		mk("109944", "Just Mercy 2019 2160p UHD BluRay REMUX DoVi HDR10 HEVC Atmos TrueHD 7.1-UBits", 63995012710),
		mk("109323", "[热门]【DIY 原盘 00884】正义的慈悲 / 以公义之名(港) 4K UHD 原盘 DIY 简体 保留杜比视界UBits官方DIY中字", 72638634393),
		mk("108076", "Just Mercy 2019 2160p UHD Blu-ray HEVC Atmos TrueHD7.1-DiY@HDHome", 72627896975),
	}

	match, stats := VerifyMatchWithTruncationCheckAndSource(results, "UBits", orphanSize, srcTitle)

	if match == nil {
		t.Fatalf("expect tid=109323 match, got nil; stats=%+v", stats)
	}
	if match.TorrentID != "109323" {
		t.Fatalf("expect tid=109323 (中文副标题候选), got %s (%s)", match.TorrentID, match.Title)
	}
	// 反驳分布：WEB-DL 变体（组名过、音频 DDP≠TrueHD）tech 反驳、REPACK 版本反驳、
	// REMUX size 超窗、HDHome 组名反驳
	if stats.TechRefute == 0 {
		t.Errorf("expect techRefute>0 (WEB-DL spec), got %+v", stats)
	}
	if stats.VersionRefute == 0 {
		t.Errorf("expect versionRefute>0 (REPACK), got %+v", stats)
	}
	if stats.GroupRefute == 0 {
		t.Errorf("expect groupRefute>0 (DiY@HDHome), got %+v", stats)
	}
	if stats.GroupNeutral == 0 && stats.TitleNeutral == 0 {
		t.Errorf("expect neutral counters>0 (中文候选组名确认含 UBits 或标题中性), got %+v", stats)
	}
}

// §59.184 三态——语言对称八组合核心断言（英源×中候放行）。
func TestVerifyThreeState_LanguageSymmetry(t *testing.T) {
	const size = int64(50 * 1073741824)

	enSrc := "Just.Mercy.2019.2160p.UHD.Blu-ray.DoVi.HEVC.TrueHD.7.1-DIY@UBits"
	cnCand := &model.SeedingSearchResult{TorrentID: "cn1",
		Title: "[热门]【DIY 原盘】正义的慈悲 4K UHD 原盘 简体 保留杜比视界", Size: size}

	// 英源 × 中候（原双杀场景）：组名中性（无后缀）+ 标题中性 + size 精确短路 → 放行
	m, _ := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cnCand}, "UBits", size, enSrc)
	if m == nil || m.TorrentID != "cn1" {
		t.Fatalf("英源×中候(size精确)应放行, got %v", m)
	}

	// 英源 × 中候：size 仅显示等价（差 3MB < 5.37MB 窗）→ 仍放行（≥1确认）
	cnCand2 := &model.SeedingSearchResult{TorrentID: "cn2", Title: cnCand.Title, Size: size + 3*1048576}
	m2, _ := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cnCand2}, "UBits", size, enSrc)
	if m2 == nil || m2.TorrentID != "cn2" {
		t.Fatalf("英源×中候(显示等价)应放行, got %v", m2)
	}

	// 英源 × 中候：size 超窗（差 500MB）→ size 反驳
	cnCand3 := &model.SeedingSearchResult{TorrentID: "cn3", Title: cnCand.Title, Size: size + 500*1048576}
	m3, stats3 := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cnCand3}, "UBits", size, enSrc)
	if m3 != nil {
		t.Fatalf("英源×中候(超窗)应拒, got %v", m3)
	}
	if stats3.SizeRefute == 0 {
		t.Errorf("expect sizeRefute=1, got %+v", stats3)
	}

	// 中源 × 中候：CJK 确认 + size 精确
	cnSrc := "三国演义.1994.DVDRip.X264"
	cnC := &model.SeedingSearchResult{TorrentID: "c1", Title: "三国演义 全95集 国语经典", Size: size}
	m4, _ := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cnC}, "", size, cnSrc)
	if m4 == nil {
		t.Fatalf("中源×中候应放行（CJK确认+size精确）")
	}

	// 续集号反驳：S01 vs S02
	srcS1 := "Show.S01.1080p.WEB-DL.x264-GRP"
	candS2 := &model.SeedingSearchResult{TorrentID: "s2", Title: "Show S02 1080p WEB-DL x264-GRP", Size: size}
	m5, stats5 := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{candS2}, "GRP", size, srcS1)
	if m5 != nil {
		t.Fatalf("续集号不同应反驳, got %v", m5)
	}
	if stats5.SequelRefute == 0 {
		t.Errorf("expect sequelRefute=1, got %+v", stats5)
	}
}

// §59.184 G3 裁决序——确认门数降序、平局 diff 升序、精确短路优先。
func TestVerifyThreeState_AdjudicationOrder(t *testing.T) {
	const size = int64(10 * 1073741824)
	src := "Movie.2019.1080p.BluRay.DTS-HD.x264-GRP"
	mk := func(id, title string, sz int64) *model.SeedingSearchResult {
		return &model.SeedingSearchResult{TorrentID: id, Title: title, Size: sz}
	}

	// 平局：两个英文候选均 4/3 确认 → diff 小者胜
	r1 := mk("near", "Movie 2019 1080p BluRay DTS-HD x264-GRP", size+2*1048576)
	r2 := mk("far", "Movie 2019 1080p BluRay DTS-HD x264-GRP", size+4*1048576)
	m, _ := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{r2, r1}, "GRP", size, src)
	if m == nil || m.TorrentID != "near" {
		t.Fatalf("平局应选 diff 小者, got %v", m)
	}

	// 确认门数：3 确认英候 vs 2 确认中候（size 均等价）→ 英候胜
	cn := mk("cn", " Movie 2019 1080p 原盘 中字", size+1048576)
	en := mk("en", "Movie 2019 1080p BluRay DTS-HD x264-GRP", size+2*1048576)
	m2, _ := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cn, en}, "GRP", size, src)
	if m2 == nil || m2.TorrentID != "en" {
		t.Fatalf("确认门数多者应胜, got %v", m2)
	}

	// 精确字节短路：即使排序更优的候选在前，精确字节候选立即返回
	rLow := mk("low", "Movie 2019 1080p BluRay DTS-HD x264-GRP", size+1048576)
	rExact := mk("exact", "Movie 2019 1080p BluRay DTS-HD x264-GRP", size)
	m3, _ := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{rLow, rExact}, "GRP", size, src)
	if m3 == nil || m3.TorrentID != "exact" {
		t.Fatalf("精确字节应短路, got %v", m3)
	}
}

// §59.184 B 补线——leadingCJKSegment 形态矩阵。
func TestLeadingCJKSegment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"冰冻星球.BBC.Frozen.Planet.S02.2160p.WEB-DL.H.264-HDSWEB", "冰冻星球"},
		{"忍者神龟：变种时代.BluRay.1080p-GRP", "忍者神龟：变种时代"},
		{"[热门]正义的慈悲.2019.1080p-GRP", "正义的慈悲"},
		{"BBC.Frozen.Planet.2011", ""},          // 前导 ASCII → 无 B1 前缀
		{"全程战课示威者中字超帅4k杜比视界版", "全程战课示威者中字超帅"}, // 4k ASCII 截断
		{"三国演义.全95集.2010", "三国演义.全"}, // 全 是 CJK，段延续（搜索子串仍有效）
	}
	for _, c := range cases {
		if got := leadingCJKSegment(c.in); got != c.want {
			t.Errorf("leadingCJKSegment(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// §59.184 附四: 合集数字序号前缀剥除——关键词污染/误杀/中文线三点。
func TestStripEpisodeNumberPrefix(t *testing.T) {
	cases := []struct{ in, wantKw, wantCN string }{
		// 原始名 → 期望关键词不含序号 + 中文段可提取（B 补线）
		{"12.仙履奇缘.Cinderella.1950.BluRay.1080p.x265.10bit.4Audios.MNHD-FRDS", "Cinderella 1950 1080p", "仙履奇缘"}, // 英文优先：中文归 B 补线
		{"12.2.仙履奇缘2.Cinderella.2.2002.1080p.Bluray.x265.10bit.4Audios.MNHD-FRDS", "Cinderella 2 2002 1080p BluRay", "仙履奇缘"},
		{"11.伊老师与小蟾蜍大历险.The.Adventures.of.Ichabod.and.Mr.Toad.1949.BluRay.1080p.x265.10bit.MNHD-FRDS", "The Adventures of Ichabod Mr Toad 1949 1080p", "伊老师与小蟾蜍大历险"}, // and 剥离（§59.192 A）
	}
	for _, c := range cases {
		kw := ExtractSearchKeyword(c.in)
		if kw != c.wantKw {
			t.Errorf("keyword(%q)=%q want %q", c.in, kw, c.wantKw)
		}
		if KeywordHasNoTitle(kw) {
			t.Errorf("keyword(%q) 不应误判无标题", c.in)
		}
		if cn := leadingCJKSegment(c.in); cn != c.wantCN {
			t.Errorf("leadingCJKSegment(%q)=%q want %q", c.in, cn, c.wantCN)
		}
	}
	// 年份开头（4 位）不剥
	if got := stripEpisodeNumberPrefix("2019.冰冻星球.BBC.Frozen.Planet.S02"); got != "2019.冰冻星球.BBC.Frozen.Planet.S02" {
		t.Errorf("年份前缀误剥: %q", got)
	}
	// 无前缀不变
	if got := stripEpisodeNumberPrefix("冰冻星球.BBC.Frozen"); got != "冰冻星球.BBC.Frozen" {
		t.Errorf("无前缀误剥: %q", got)
	}
}

// §59.185: SourceType 血统反驳——BluRay vs DVD 拒；书写变体等价不误杀。
func TestSourceTypeRefute(t *testing.T) {
	const size = int64(3430444459)
	// 旋律时光实证：BluRay 源 × DVDrip 候选（同尺寸）——此前零反驳放行
	src := "旋律时光.Melody.Time.1948.BluRay.1080p.upscale.x265.10bit.MNHD-FRDS"
	cand := &model.SeedingSearchResult{TorrentID: "9322",
		Title: "Melody Time 1948 DVDrip 1080p upscale x265 10bit MNHD-FRDS", Size: size}
	m, stats := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cand}, "FRDS", size, src)
	if m != nil {
		t.Fatalf("BluRay×DVDrip 应血统反驳, got match")
	}
	if stats.TechRefute == 0 {
		t.Errorf("expect techRefute, got %+v", stats)
	}

	// 书写变体等价：UHD Blu-ray 源 × UHD BluRay 候选——不误杀
	src2 := "Just.Mercy.2019.2160p.UHD.Blu-ray.DoVi.HEVC.TrueHD.7.1-DIY@UBits"
	cand2 := &model.SeedingSearchResult{TorrentID: "t2",
		Title: "Just Mercy 2019 2160p UHD BluRay DoVi HEVC TrueHD 7.1-DIY@UBits", Size: size}
	m2, _ := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cand2}, "UBits", size, src2)
	if m2 == nil {
		t.Fatalf("UHD Blu-ray ≡ UHD BluRay 变体等价应放行")
	}
}
