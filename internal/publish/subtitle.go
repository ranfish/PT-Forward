package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/util"

	bdrom "github.com/ranfish/pt-forward/internal/bdinfo/bdrom"
	bdsettings "github.com/ranfish/pt-forward/internal/bdinfo/settings"
	"go.uber.org/zap"
)

type SubtitleCandidate struct {
	StreamIndex int
	// SubIndex §59.589: 字幕流绝对序（1-based）——mpv --sid 语义。
	// Detect 按全部 ffprobe 字幕流行计数（含被 disposition 滤掉的轨——
	// 它们不参与评分但在 mpv 轨表占号）；probeDisc 同步补齐（无滤轨
	// 场景恰=StreamIndex）。原映射按「过滤后候选列表位置」数 sid——
	// 被滤轨排在中文轨前时错位（Scandal 案：kor HI 轨被滤致 chi 从
	// sid3 偏移成 sid2=韩文）。
	SubIndex int
	Codec    string
	Language string
	Title    string
	IsText   bool
}

type SubtitleDetector struct {
	ffprobePath string
	logger      *zap.Logger
}

func NewSubtitleDetector(logger *zap.Logger) *SubtitleDetector {
	return &SubtitleDetector{
		ffprobePath: "ffprobe",
		logger:      logger,
	}
}

func (d *SubtitleDetector) Available() bool {
	_, err := exec.LookPath(d.ffprobePath)
	return err == nil
}

func (d *SubtitleDetector) Detect(ctx context.Context, videoPath string) ([]SubtitleCandidate, error) {
	if !d.Available() {
		return nil, fmt.Errorf("ffprobe not found")
	}

	cmd := exec.CommandContext(ctx, d.ffprobePath, //nolint:gosec // intentional subprocess
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name:stream_tags=language,title:stream_disposition=comment,hearing_impaired,visual_impaired",
		"-of", "json",
		videoPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe subtitle detection: %w", err)
	}

	var result struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
			Disposition map[string]interface{} `json:"disposition"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var candidates []SubtitleCandidate
	// subIdx §59.589: ffprobe 字幕流绝对序（1-based）——含被滤轨
	subIdx := 0
	// §59.291: webvtt（NF/AMZN WEB-DL MKV 内嵌标配——UNABOMBER 中文简繁
	// 双轨全 WEBVTT 实证）+ mov_text（MP4 内嵌）补入——原白名单致 Detect
	// 全滤 → sid=0 → --sid=no 裸图
	textCodecs := map[string]bool{"ass": true, "subrip": true, "srt": true, "ssa": true, "webvtt": true, "mov_text": true}
	graphicCodecs := map[string]bool{
		"hdmv_pgs_subtitle": true, "pgssub": true,
		"dvd_subtitle": true, "dvdsub": true,
		"dvb_subtitle": true, "dvbsub": true,
	}

	for _, stream := range result.Streams {
		// §59.589: 字幕流绝对序先计数——被滤轨不参与评分但占 mpv 轨号
		subIdx++
		codec := strings.ToLower(stream.CodecName)
		lang := strings.ToLower(stream.Tags.Language)
		title := strings.ToLower(stream.Tags.Title)
		disposition := stream.Disposition

		if getDispositionFlag(disposition, "comment") ||
			getDispositionFlag(disposition, "hearing_impaired") ||
			getDispositionFlag(disposition, "visual_impaired") {
			continue
		}

		_, isText := textCodecs[codec]
		_, isGraphic := graphicCodecs[codec]
		if !isText && !isGraphic {
			// §59.291 附: codec unknown 兜底——MKV 内嵌 WEBVTT 的 CodecID
			// （W_WEBVTT）ffprobe 报 codec_name 缺省（UNABOMBER 实证：三轨
			// chi/中文简繁全 unknown 被白名单滤掉 → sid=0 裸图）。判据反转：
			// 有语言/标题标记的字幕流（-select_streams s 已保证是字幕流）
			// 按文本候选兜底——mpv 渲染兼容 webvtt
			if codec == "" || codec == "unknown" {
				if lang != "" || title != "" {
					isText = true
				} else {
					continue
				}
			} else {
				continue
			}
		}

		// §59.590: Score 字段与计算已删——tier 体系（§59.319 附十→§59.590 八级）
		// 取代后 SelectBestSubtitle 不消费（两套评分并存误导读者·死代码清理）

		candidates = append(candidates, SubtitleCandidate{
			StreamIndex: stream.Index,
			SubIndex:    subIdx,
			Codec:       codec,
			Language:    lang,
			Title:       stream.Tags.Title,
			IsText:      isText,
		})
	}

	return candidates, nil
}

// §59.590: PickSubtitleSidFromBDInfo 已删（原附十四设计被附十五「原盘选轨
// 单点下沉 FindSubtitleStreamID」取代后零生产消费点——死代码清理）。

// langTier §59.590（2026-11-28 用户定案·字典序两级）：
// 体系主键（简/含简 → 繁/含繁 → 英/含英 → 兜底）× 形态次键（双字 → 单语）
// 展开八级：简双字(7) > 纯简(6) > 繁双字(5) > 纯繁(4) > 英双字(3) > 纯英(2) > 兜底(0)。
//
// 判定序（§59.590 回归复审定案）：
//
//	title 信号优先于 lang 信号（title 是跨工具稳定层——ffprobe/MI 原样透传；
//	lang 是格式化敏感层——MI 全词 vs ffprobe 短码两形态并存）。
//	① title 内简繁双信号（CHS&CHT）→ 归简（简中行保障）
//	② title 繁词（[Chinese][CHT]×3831 实证）→ 繁（title 具体信号压过 lang 泛化）
//	③ 位置式（中上英下/英上中下摸底实证）→ 「中/英」为位置成分不参与简繁
//	  判定，简繁从 lang 分（lang 繁系→繁·其它→简默认）
//
// 双语词表分离（§59.590 发现1）：
//
//	宽词表（中文体系）= &｜英(裸·简英/繁英简写形态)｜双语
//	  ——裸「英」安全域=中文体系前提内（判定链保证）
//	窄词表（英体系）  = &｜双语——排除英文自身词（防纯 English 轨自吞噬误判英双字）
//
// 信号词表（摸底 88659 轨实证驱动）：
//
//	简系 lang: zh-hans/zh-cn/chs/chi/zho/zh/chinese
//	繁系 lang: zh-hant/zh-tw/zh-hk/cht/hant（hant=Chinese-Hant-TW/HK
//	           摸底 1071 条实证形态——原词表零命中）
//	简系 title: 简/chs/simplified/中/chinese/中英/英中（中英复合=中文基底·发现2）
//	繁系 title: 繁/cht/traditional
//	英系: lang en 前缀（en/eng/en-US/english 全覆盖）/title eng|english|英文
//
// Commentary 双保险：title 含 commentary → 兜底（disposition comment 主滤
// 之后的词判防御——摸底 ~200 条 Commentary & 陷阱形态）。
func langTier(lang, title string) int {
	if util.ContainsAny(title, "commentary") {
		return 0
	}
	wideSecond := util.ContainsAny(title, "&", "＆", "英", "双语")
	narrowSecond := util.ContainsAny(title, "&", "＆", "双语")
	hasEng := strings.HasPrefix(lang, "en") ||
		util.ContainsAny(title, "eng", "english", "英文")

	positional := util.ContainsAny(title, "中上英下", "英上中下")
	// §59.590 回归修正：区分词与泛指词分层——「中」是 Chinese 泛指非简信号
	// （「中文（繁體）」摸底 559 条：泛指+繁区分→繁；「CHS&CHT」双区分→简）
	tradDistinct := util.ContainsAny(title, "繁", "cht", "traditional")
	simpDistinct := util.ContainsAny(title, "简", "chs", "simplified")
	zhGeneric := util.ContainsAny(title, "中", "chinese", "中英", "英中")
	tradLang := util.ContainsAny(lang, "zh-hant", "zh-tw", "zh-hk", "cht", "hant")
	simpLang := util.ContainsAny(lang, "zh-hans", "zh-cn", "chs", "chi", "zho", "zh", "chinese")

	isTrad, isSimp := false, false
	if positional {
		// 位置式：简繁从 lang（繁系→繁·其它→简默认——上轮定案）
		isTrad, isSimp = tradLang, !tradLang
	} else if simpDistinct && tradDistinct {
		isSimp = true // 双区分词（CHS&CHT）→ 简中行保障
	} else if tradDistinct {
		isTrad = true // 繁区分词压泛指词（[Chinese][CHT]×3831+中文（繁體）×559）
	} else if simpDistinct {
		isSimp = true
	} else if zhGeneric {
		isSimp = true // 泛指无区分 → 默认简（中文即中）
	} else if tradLang {
		isTrad = true
	} else if simpLang {
		isSimp = true
	}

	switch {
	case isTrad:
		if wideSecond {
			return 5 // 繁双字
		}
		return 4 // 纯繁
	case isSimp:
		if wideSecond {
			return 7 // 简双字
		}
		return 6 // 纯简
	case hasEng:
		if narrowSecond {
			return 3 // 英双字（ENG&FRE/KOR&ENG——窄表防自吞噬）
		}
		return 2 // 纯英
	}
	return 0 // 兜底
}

// SelectBestSubtitle §59.590: 八级选轨（原 SelectBestChinese——语义已远超
// 中文三级故改名）。次序链：tier → secondLang（双字内第二语言含英 > 非英
// ——CHS&ENG 压 CHS&FRE）→ codec（ass>文本>图形）→ 保序（ffprobe 流序）。
// tier 全 0 → 兜底第一候选（宁什么不裸图——§59.298 语义）。
func (d *SubtitleDetector) SelectBestSubtitle(candidates []SubtitleCandidate) (int, string) {
	best := -1
	bestTier, bestSecond, bestRank := -1, -1, -1
	codecRank := func(c *SubtitleCandidate) int {
		if c.Codec == "ass" {
			return 2
		}
		if c.IsText {
			return 1
		}
		return 0
	}
	for i := range candidates {
		tier := langTier(strings.ToLower(candidates[i].Language), strings.ToLower(candidates[i].Title))
		rank := codecRank(&candidates[i])
		second := 0
		if tier == 7 || tier == 5 {
			// 中文双字：第二语言含英次序（简英/繁英 > 简法/繁日等）
			if strings.HasPrefix(candidates[i].Language, "en") ||
				util.ContainsAny(strings.ToLower(candidates[i].Title), "eng", "english", "英") {
				second = 1
			}
		}
		if tier > bestTier ||
			(tier == bestTier && second > bestSecond) ||
			(tier == bestTier && second == bestSecond && rank > bestRank) {
			best, bestTier, bestSecond, bestRank = i, tier, second, rank
		}
	}
	if best < 0 {
		return 0, ""
	}
	if bestTier == 0 {
		// 无体系命中：按顺序第一候选（非 codec 优先）
		return candidates[0].StreamIndex, candidates[0].Codec
	}
	return candidates[best].StreamIndex, candidates[best].Codec
}

func (d *SubtitleDetector) FindSubtitleStreamID(ctx context.Context, videoPath string) (int, error) {
	candidates, err := d.Detect(ctx, videoPath)
	// §59.319 附十五修订：回退条件=无候选**或无语言信号**——m2ts PGS 常态是
	// ffprobe 报出无语言候选（Under Current 实证 2 条空语言 PGS），原
	// "空才回退"致 tier 全 0 恒选第一轨（英文）。语言信号=任一候选
	// lang/title 非空（mkv 正常有）；全空且是原盘 → CLPI 补语言（权威源）。
	hasLangSignal := false
	for _, c := range candidates {
		if c.Language != "" || c.Title != "" {
			hasLangSignal = true
			break
		}
	}
	if (err != nil || len(candidates) == 0 || !hasLangSignal) && detectDiscForSubtitle(videoPath) != "" {
		if clpiCands := probeDiscSubtitleCandidates(ctx, detectDiscForSubtitle(videoPath), videoPath); len(clpiCands) > 0 {
			candidates = clpiCands
			err = nil
		}
	}
	if len(candidates) == 0 {
		return 0, err
	}

	streamID, _ := d.SelectBestSubtitle(candidates)
	if streamID == 0 {
		return 0, nil
	}

	// §59.589: sid=字幕流绝对序（mpv 语义）——原按「过滤后候选列表
	// 位置」数，被 disposition 滤掉的轨排在中文轨前时错位（Scandal
	// 案：kor HI 轨被滤致 chi 真实 sid3 偏移成 sid2=韩文 HI 轨）。
	for _, c := range candidates {
		if c.StreamIndex == streamID {
			return c.SubIndex, nil
		}
	}
	return 1, nil
}

func getDispositionFlag(d map[string]interface{}, key string) bool {
	if v, ok := d[key]; ok {
		switch val := v.(type) {
		case float64:
			return val != 0
		case string:
			b, _ := strconv.ParseBool(val)
			return b
		case bool:
			return val
		}
	}
	return false
}

// detectDiscForSubtitle §59.319 附十五：从 videoPath 定位原盘根。
// .m2ts → 向上找 BDMV 父目录（盘根）；.iso → 原样（bdrom 内部 UDF 分流）。
// 非原盘形态返回空。
func detectDiscForSubtitle(videoPath string) string {
	lower := strings.ToLower(videoPath)
	if strings.HasSuffix(lower, ".iso") {
		return videoPath
	}
	if !strings.HasSuffix(lower, ".m2ts") {
		return ""
	}
	dir := filepath.Dir(videoPath) // .../BDMV/STREAM
	if !strings.HasSuffix(strings.ToLower(filepath.ToSlash(dir)), "/bdmv/stream") {
		return ""
	}
	return filepath.Dir(filepath.Dir(dir)) // 盘根（BDMV 父）
}

// probeDiscSubtitleCandidates §59.319 附十五：CLPI 快速解析（毫秒级）——
// 构造与 ffprobe Detect 同构的候选列表（StreamIndex=字幕序 1..N，与 mpv
// sid 对应——CLPI StreamOrder 过滤字幕流后的序=容器字幕轨序，Under
// Current 实证 1:1）。目录盘取 videoPath 同名 CLPI（精确该文件流表）；
// ISO 取主 playlist（FileSize 最大）流序（mpv 直读 ISO 不可行的既有
// 边界下为将来留位）。语言为 ISO3 码（zho/eng），chineseTier 词表覆盖。
func probeDiscSubtitleCandidates(ctx context.Context, discRoot, videoPath string) []SubtitleCandidate {
	rom, err := bdrom.New(discRoot, bdsettings.Default(""))
	if err != nil {
		return nil
	}
	defer rom.Close()
	if res := rom.ScanMetadataWithProgress(ctx, nil); res.ScanError != nil && ctx.Err() == nil {
		return nil
	}

	// 定位目标 CLPI 流表
	var clip *bdrom.StreamClipFile
	if strings.HasSuffix(strings.ToLower(videoPath), ".m2ts") {
		name := strings.ToUpper(strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))) + ".CLPI"
		if cf, ok := rom.StreamClipFiles[name]; ok {
			clip = cf
		}
	}
	if clip == nil {
		// ISO/未命中：主 playlist（时长最长——metadata 扫描可得；TotalSize
		// 在 metadata 阶段恒 0，按体积选必错，243 实证 00001.MPLS size=0）
		var best *bdrom.PlaylistFile
		for _, pl := range rom.PlaylistFiles {
			if best == nil || pl.TotalLength() > best.TotalLength() {
				best = pl
			}
		}
		if best == nil || len(best.StreamClips) == 0 {
			return nil
		}
		clip = best.StreamClips[0].StreamClipFile
	}
	if clip == nil {
		return nil
	}

	var out []SubtitleCandidate
	for _, pid := range clip.StreamOrder {
		s := clip.Streams[pid]
		if s == nil {
			continue
		}
		isSub := false
		if x, ok := s.(interface{ IsGraphicsStream() bool }); ok {
			isSub = x.IsGraphicsStream()
		}
		if !isSub {
			if x, ok := s.(interface{ IsTextStream() bool }); ok {
				isSub = x.IsTextStream()
			}
		}
		if !isSub {
			continue
		}
		lang := ""
		if x, ok := s.(interface{ LanguageCode() string }); ok {
			lang = x.LanguageCode()
		}
		codec := "hdmv_pgs_subtitle"
		if x, ok := s.(interface{ IsTextStream() bool }); ok && x.IsTextStream() {
			codec = "text"
		}
		out = append(out, SubtitleCandidate{
			StreamIndex: len(out) + 1,
			// §59.589: CLPI 无滤轨——绝对序恰=字幕序（mpv sid 1:1）
			SubIndex: len(out) + 1,
			Codec:    codec,
			Language: strings.ToLower(lang),
			IsText:   codec == "text",
		})
	}
	return out
}
