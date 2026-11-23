package reseed

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
)

func (e *Engine) matchLayer2SizeTitle(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, sourceInfoHash, sourceSiteName, siteName string, sizeTolerance float64, fc *fpCache) *model.Candidate {
	return e.matchLayer2SearchVerify(ctx, adapter, config, sourceInfoHash, sourceSiteName, siteName, fc, nil)
}

func (e *Engine) matchLayer2SearchVerify(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, sourceInfoHash, sourceSiteName, siteName string, fc *fpCache, l2s *l2Stats) *model.Candidate {
	fp := fc.get(sourceInfoHash, sourceSiteName)
	if fp == nil || fp.Title == "" {
		return nil
	}

	isMusic := util.DetectContentType(fp.FileTreeParsed) == "music"

	var keyword, groupName string
	if isMusic {
		keyword = ExtractMusicKeyword(fp.Title)
	} else {
		keyword = ExtractSearchKeyword(fp.Title)
		groupName = ExtractGroupName(fp.Title)

		if (keyword == "" || KeywordStartsWithYear(keyword)) && len(fp.FileTreeParsed) > 0 {
			fileKeyword, fileGroup := extractFromFileTree(fp.FileTreeParsed)
			if fileKeyword != "" && !KeywordStartsWithYear(fileKeyword) {
				e.logger.Debug("extracting keywords from video filename",
					zap.String("title", fp.Title),
					zap.String("originalKeyword", keyword),
					zap.String("fileKeyword", fileKeyword),
					zap.String("fileGroup", fileGroup))
				keyword = fileKeyword
				if fileGroup != "" {
					groupName = fileGroup
				}
			}
		}
	}

	if keyword == "" {
		if l2s != nil {
			l2s.mu.Lock()
			l2s.noKeyword++
			l2s.mu.Unlock()
		}
		return nil
	}

	// §59.184 D1: 无组名早退删除——早期种/纯中文种（场景2）靠 标题/tech/size
	// 三门验证兜底；孤儿（getSitePriority 组名空→全站照搜）与元数据获取本就无此门，
	// 辅种为唯一异类，三业务对齐。残余敞口（同碟 REMUX 变体误匹配）由注入侧
	// pieces-hash 终审兜底。

	if isMusic {
		e.logger.Debug("music torrent L2 search",
			zap.String("site", siteName),
			zap.String("title", fp.Title),
			zap.String("keyword", keyword))
	}

	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		// §59.544 修3：任务取消路径不打噪声（CancelTask 后每颗都 err·
		// 正常收尾不需要 825 条 Warn）。
		if ctx.Err() == nil {
			// §59.544 修3：L2 搜索失败单条日志——原只进 l2s 内存统计
			// （任务结束汇总才可见），站方 hang 期 30s/颗静默爬行完全
			// 不可见（本案 79 分钟零日志=排查黑洞·§59.1529 同款）。
			e.logger.Warn("L2 search failed",
				zap.String("site", siteName),
				zap.String("keyword", keyword),
				zap.Error(err))
		}
		if l2s != nil {
			l2s.record(siteName, "搜索失败")
			l2s.mu.Lock()
			l2s.searchFailed++
			l2s.mu.Unlock()
		}
		return nil
	}

	if len(results) == 0 {
		if l2s != nil {
			l2s.record(siteName, "搜索无结果")
			l2s.mu.Lock()
			l2s.searchEmpty++
			l2s.mu.Unlock()
		}
		return nil
	}

	match, filterStats := VerifyMatchWithTruncationCheckAndSource(results, groupName, fp.TotalSize, fp.Title)

	if match != nil {
		if l2s != nil {
			l2s.record(siteName, "命中")
			l2s.mu.Lock()
			l2s.matched++
			l2s.mu.Unlock()
		}
		e.logger.Info("L2 matched",
			zap.String("site", siteName),
			zap.String("keyword", keyword),
			zap.String("groupName", groupName),
			zap.Bool("music", isMusic),
			zap.String("targetTorrentID", match.TorrentID),
			zap.String("targetTitle", match.Title),
			zap.Int64("targetSize", match.Size),
			zap.Int64("sourceSize", fp.TotalSize))
		return &model.Candidate{
			TargetSite:      siteName,
			TargetTorrentID: match.TorrentID,
			Confidence:      0.95,
			MatchMethod:     "search_verify",
		}
	}

	e.logger.Info("L2 search no match",
		zap.String("site", siteName),
		zap.String("keyword", keyword),
		zap.String("groupName", groupName),
		zap.Bool("music", isMusic),
		zap.Int("results", len(results)),
		zap.Int("emptyId", filterStats.EmptyID),
		zap.Int("groupRefute", filterStats.GroupRefute),
		zap.Int("groupNeutral", filterStats.GroupNeutral),
		zap.Int("sequelRefute", filterStats.SequelRefute),
		zap.Int("titleNeutral", filterStats.TitleNeutral),
		zap.Int("sizeInvalid", filterStats.SizeInvalid),
		zap.Int("techRefute", filterStats.TechRefute),
		zap.Int("versionRefute", filterStats.VersionRefute),
		zap.Int("sizeRefute", filterStats.SizeRefute))
	if l2s != nil {
		refuted := filterStats.GroupRefute + filterStats.SequelRefute + filterStats.TechRefute + filterStats.VersionRefute + filterStats.SizeRefute
		reason := fmt.Sprintf("未匹配(refute=%d,neutral=%d)", refuted, filterStats.GroupNeutral+filterStats.TitleNeutral)
		l2s.record(siteName, reason)
		l2s.mu.Lock()
		l2s.refuted += refuted
		l2s.neutral += filterStats.GroupNeutral + filterStats.TitleNeutral
		l2s.mu.Unlock()
	}

	return nil
}

type L2MatchResult struct {
	TorrentID string
	Title     string
	Size      int64
}

// MatchFilterStats §59.184 三态化：反驳按原因拆分、不可验证计中性
// （原 GroupMiss/SizeMiss/TitleMiss 杂烩计数消融——§59.182 排查被误导教训）。
type MatchFilterStats struct {
	EmptyID       int
	GroupRefute   int // 组名反驳：显式异组名后缀（同碟 REMUX 体积不可分，组名唯一判别器）
	GroupNeutral  int // 组名中性：无显式组名后缀（中文标题/音乐/截断）
	SequelRefute  int // 标题反驳：续集号不同
	TitleNeutral  int // 标题中性：无命中（原 TitleMiss 拒绝 → 三态改中性）
	SizeInvalid   int // size<=0 运营性拒绝（非身份判断）
	TechRefute    int // 规则 A：字段双方有值且不等价
	VersionRefute int // 规则 B：候选有版本 Token 源无（G2 修复后含无括号 REPACK）
	SizeRefute    int // 超显示等价窗
}

func VerifyMatchWithStats(results []*model.SeedingSearchResult, groupName string, sourceSize int64) (*L2MatchResult, *MatchFilterStats) {
	return VerifyMatchWithStatsAndSource(results, groupName, sourceSize, "")
}

// VerifyMatchWithStatsAndSource §59.184 三态门主链。
// 每门三态：确认（正向证据）/ 中性（语言形态无法验证→不杀）/ 反驳（身份矛盾→一票否决）。
// 裁决：任一反驳→拒；≥1 确认且 0 反驳→放行；全中性→拒。
// 排序（G3）：精确字节短路保留；通过者按 确认门数降序 → |diff| 升序。
// sourceTitle 非空时按规则 A（字段不等价→反驳）/ B（候选版本 Token 源无→禁等价匹配）过滤。
func VerifyMatchWithStatsAndSource(results []*model.SeedingSearchResult, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, *MatchFilterStats) {
	var srcProfile *titleparser.TechProfile
	if sourceTitle != "" {
		p := titleparser.ParseTitleTech(sourceTitle)
		srcProfile = &p
	}

	// §59.214: 品牌兄弟预扫描——同组（标题含组名或中性）+ size 窗内 +
	// 无品牌词的候选存在 = 品牌构成判别器。
	hasPlainBrandSibling := false
	if srcProfile != nil && !isBrandEdition(srcProfile.EditionInfo) {
		for _, r := range results {
			if r.Size <= 0 || !CompareSizeDisplay(sourceSize, r.Size) {
				continue
			}
			if groupName != "" && !strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
				continue
			}
			if p := titleparser.ParseTitleTech(r.Title); !isBrandEdition(p.EditionInfo) {
				hasPlainBrandSibling = true
				break
			}
		}
	}

	stats := &MatchFilterStats{}
	var best *L2MatchResult
	var bestConfirms int
	var bestDiff int64

	meaningfulWords := extractMeaningfulTitleWords(sourceTitle, groupName)
	sourceCJK := extractCJKSubstrings(sourceTitle)

	for _, r := range results {
		if r.TorrentID == "" {
			stats.EmptyID++
			continue
		}
		confirms := 0

		// 门2 组名（三态）：确认（含预期组名）优先于反驳短路——
		// 中文副标题内嵌组名（如 "UBits官方DIY"）无后缀形态由 contains 命中。
		if groupName != "" {
			if strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
				confirms++
			} else if cg := ExtractGroupName(r.Title); cg != "" && !strings.EqualFold(cg, groupName) && !sameGroupFamily(cg, groupName) {
				// §59.187 ①: 同族拼写变体豁免（UBits/UBbits 同映射优堡官方）——
				// 豁免不计确认（保守），靠标题/tech/size 确认通过
				stats.GroupRefute++
				continue
			} else {
				stats.GroupNeutral++
			}
		}

		// 门3 标题（三态）：续集号反驳保持；无命中中性（原 TitleMiss 消融）。
		if sourceTitle != "" {
			switch titleKeywordState(meaningfulWords, sourceCJK, sourceTitle, r.Title) {
			case titleStateRefute:
				stats.SequelRefute++
				continue
			case titleStateConfirm:
				confirms++
			default:
				stats.TitleNeutral++
			}
		}

		// 门4 size>0：运营性必填（size 恒有比较能力，无中性态）。
		if r.Size <= 0 {
			stats.SizeInvalid++
			continue
		}

		// 门5 tech 规则 A：双方有值且不等价 → 反驳（等价组豁免保持）。
		if srcProfile != nil && techProfileConflict(*srcProfile, r.Title) {
			stats.TechRefute++
			continue
		}
		// tech 确认：≥1 字段双方有值且等价。
		if srcProfile != nil && techProfileConfirm(*srcProfile, r.Title) {
			confirms++
		}

		// 门6 精确字节短路（优先于版本 Token——保持既有语义：字节相同即同一 torrent）。
		if r.Size == sourceSize {
			return &L2MatchResult{
				TorrentID: r.TorrentID,
				Title:     r.Title,
				Size:      r.Size,
			}, stats
		}

		// 门7 版本 Token（规则 B）：候选有版本定义源无 → 仅允许精确匹配。
		// §59.214: 品牌血统反驳相对化——仅当同组同 size 窗非品牌兄弟存在
		// （集合级预扫描 hasPlainBrandSibling）时反驳品牌候选（克拉之膝双版
		// 消歧保持）；无兄弟时品牌词不构成判别器（密阳案：唯一候选即 CC 版，
		// 中文名不写 CC 是命名习惯——绝对反驳假杀实证）。
		if srcProfile != nil {
			brandRefute := false
			if hasPlainBrandSibling && !isBrandEdition(srcProfile.EditionInfo) {
				if p := titleparser.ParseTitleTech(r.Title); isBrandEdition(p.EditionInfo) {
					brandRefute = true
				}
			}
			if brandRefute || techProfileVersionDefined(*srcProfile, r.Title) {
				stats.VersionRefute++
				continue
			}
		}

		// 门8 size 显示等价（v3）：超窗=内容级反驳。
		if !CompareSizeDisplay(sourceSize, r.Size) {
			stats.SizeRefute++
			continue
		}
		confirms++ // size 等价确认

		// 全中性零证据 → 拒（size 门恒有结论，此为防御性条款）。
		if confirms == 0 {
			continue
		}

		diff := r.Size - sourceSize
		if diff < 0 {
			diff = -diff
		}
		if best == nil || confirms > bestConfirms || (confirms == bestConfirms && diff < bestDiff) {
			best = &L2MatchResult{
				TorrentID: r.TorrentID,
				Title:     r.Title,
				Size:      r.Size,
			}
			bestConfirms = confirms
			bestDiff = diff
		}
	}
	if best != nil {
		return best, stats
	}
	return nil, stats
}

// titleKeywordState §59.184: 标题门三态化（原 titleKeywordRelevant 的三态重构；
// 后者保留供 loose 轮/AudioConflictCandidates 等未三态化消费方使用）。
type titleState int

const (
	titleStateNeutral titleState = iota
	titleStateConfirm
	titleStateRefute
)

func titleKeywordState(meaningfulWords []string, sourceCJK []string, sourceTitle, candidateTitle string) titleState {
	// 续集号比较（硬反驳）：双方都有续集号且不同 → 拒绝
	sourceSequel := extractSequelNumber(sourceTitle)
	candidateSequel := extractSequelNumber(candidateTitle)
	if sourceSequel > 0 && candidateSequel > 0 && sourceSequel != candidateSequel {
		return titleStateRefute
	}

	if candidateTitle == "" {
		return titleStateNeutral
	}
	if len(meaningfulWords) == 0 && len(sourceCJK) == 0 {
		return titleStateNeutral
	}
	candLower := strings.ToLower(candidateTitle)
	if len(meaningfulWords) > 0 {
		for _, w := range meaningfulWords {
			if strings.Contains(candLower, w) {
				return titleStateConfirm
			}
		}
		// §59.184: 英文词在纯中文候选中零命中=无法验证（中性），原 return false 拒绝消融
		return titleStateNeutral
	}
	candCJK := extractCJKSubstrings(candidateTitle)
	if len(sourceCJK) > 0 && len(candCJK) > 0 {
		for _, s := range sourceCJK {
			if strings.Contains(candidateTitle, s) {
				return titleStateConfirm
			}
		}
		return titleStateNeutral
	}
	return titleStateNeutral // 原 true 技术性放行 → 显式中性
}

// techProfileConfirm §59.184: tech 门确认态——≥1 字段双方有值且等价
// （等价判定与 techProfileConflictFields 同口径：EqualFold + spec/codec 等价组）。
func techProfileConfirm(src titleparser.TechProfile, candidateTitle string) bool {
	cand := titleparser.ParseTitleTech(candidateTitle)
	fields := []struct{ name, s, c string }{
		{"Resolution", src.Resolution, cand.Resolution},
		{"VideoCodec", src.VideoCodec, cand.VideoCodec},
		{"AudioCodec", src.AudioCodec, cand.AudioCodec},
		{"HDR", src.HDR, cand.HDR},
		{"Specification", src.Specification, cand.Specification},
		{"RegionCode", src.RegionCode, cand.RegionCode},
		{"SourceType", src.SourceType, cand.SourceType},
		// §59.197: 3D 封装补入——HSBS×HOU 不同封装=不同种（300勇士案：
		// 站内两版同体积同组，双显异值才反驳；省略/"3D"泛词为空值放行）
		{"Stereo3D", src.Stereo3D, cand.Stereo3D},
	}
	for _, f := range fields {
		if f.s == "" || f.c == "" {
			continue
		}
		if strings.EqualFold(f.s, f.c) {
			return true
		}
		if f.name == "Resolution" && resolutionEquivalent(f.s, f.c) {
			return true
		}
		if f.name == "SourceType" && sourceTypeEquivalent(f.s, f.c) {
			return true
		}
		if f.name == "Specification" && specEquivalent(f.s, f.c) {
			return true
		}
		if f.name == "VideoCodec" && codecEquivalent(f.s, f.c) {
			return true
		}
	}
	return false
}

// resolutionEquivalent §59.184: 分辨率显示变体同义（4K≡2160p/8K≡4320p/2K≡1440p）。
// 经词典 variants 映射到同一 standard_key 判等——中文副标题 "4K UHD 原盘"（§59.181
// 同族路径）与英文 "2160p" 由提取器返回原始 token，比较侧规范化兜底。
func resolutionEquivalent(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	ka := titleparser.LookupDictKey("resolution", a)
	return ka != "" && ka == titleparser.LookupDictKey("resolution", b)
}

// sourceTypeEquivalent §59.185: 源类型等价——剥连字符/空格后大小写不敏感比较。
// 同血统书写变体归一（"UHD Blu-ray" ≡ "UHD BluRay"、"Blu-ray" ≡ "BluRay"——
// Just Mercy 案实证两形态并存）。血统不同（BluRay vs DVD）不等价 → 反驳。
func sourceTypeEquivalent(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "-", "")
		s = strings.ReplaceAll(s, " ", "")
		// §59.193: UHD 前缀消解——源 "4K Blu-ray"（4K 被 Res 消费，ST 剩 "Blu-ray"）
		// vs 候选 "2160p UHD Blu-ray"（UHD 归 ST）——同为 UHD 蓝光仅归属路径不同
		// （Queen tid=3020 tech_refute 误杀实证）。分辨率差异由 Resolution 门独立把关，
		// 此处消解前缀安全。
		if t := strings.TrimPrefix(s, "uhd"); t != "" {
			s = t
		}
		return s
	}
	na, nb := norm(a), norm(b)
	return na != "" && na == nb
}

// GroupFamilyResolver §59.187 ①: 组名→官方映射站点域（同站=同族拼写变体）。
// 启动时由 main 注入（release_group_mappings 查询闭包）；nil 时退化为纯字符串比较。
// 背景：UBbits/UBits 同族变体（v0.0.473 映射层已认，验证层未接线——
// 回到未来II tid=109991 误杀案：映射表只在 priority 选站被查询，组名反驳是原串比较）。
var GroupFamilyResolver func(group string) string

// sameGroupFamily: 两拼写均官方映射到同一站点 = 同族 → 组名反驳豁免。
func sameGroupFamily(a, b string) bool {
	if GroupFamilyResolver == nil {
		return false
	}
	fa, fb := GroupFamilyResolver(a), GroupFamilyResolver(b)
	return fa != "" && fa == fb
}

// techProfileConflict 规则 A：源标题和候选标题都有某 Token 且值不同 → 冲突。
// §59.35: 等价组迁移至公共字典（dict/*.json equivalence_group，titleparser.Equivalent）。
// web_delivery = {WEB-DL, WEBRip}；hevc_impl = {x265, HEVC}；avc_impl = {x264, AVC}。
// 仅比较视图使用（辅种冲突豁免）；重组/发布视图禁止等价替换。
func specEquivalent(a, b string) bool {
	return titleparser.Equivalent(strings.TrimSpace(a), strings.TrimSpace(b))
}

func codecEquivalent(a, b string) bool {
	return titleparser.Equivalent(strings.TrimSpace(a), strings.TrimSpace(b))
}

func techProfileConflict(src titleparser.TechProfile, candidateTitle string) bool {
	return techProfileConflictFields(src, candidateTitle, false)
}

// techProfileConflictFields 字段级冲突检查。skipAudio 豁免 AudioCodec 冲突
// （§59.36 元数据获取宽松场景：站点文件名/标题音轨标注不一致是站点数据瑕疵，
// 非不同资源信号；视频编码/分辨率/HDR/规格仍严格——错那些的元数据才致命）。
// §59.59 修复: 字段清单补 Resolution——原清单缺失致 1080p vs 2160p 不判冲突
// （时空奇旅案：本地 2160p UHD 错配 1080p REPACK2 元数据，仲裁候选与 loose 轮
// 双路径漏放行。分辨率是版本身份强信号，任何标注差异都不等价，无需豁免组）。
func techProfileConflictFields(src titleparser.TechProfile, candidateTitle string, skipAudio bool) bool {
	cand := titleparser.ParseTitleTech(candidateTitle)
	fields := []struct{ name, s, c string }{
		{"Resolution", src.Resolution, cand.Resolution},
		{"VideoCodec", src.VideoCodec, cand.VideoCodec},
		{"AudioCodec", src.AudioCodec, cand.AudioCodec},
		{"HDR", src.HDR, cand.HDR},
		{"Specification", src.Specification, cand.Specification},
		{"RegionCode", src.RegionCode, cand.RegionCode},
		// §59.185: SourceType 补入——源类型不同=不同血统（用户定案；
		// 旋律时光 BluRay 孤儿误配 DVDrip 种实证：ST 不在字段表零反驳放行）
		{"SourceType", src.SourceType, cand.SourceType},
		// §59.197: 3D 封装补入——HSBS×HOU 不同封装=不同种（300勇士案：
		// 站内两版同体积同组，双显异值才反驳；省略/"3D"泛词为空值放行）
		{"Stereo3D", src.Stereo3D, cand.Stereo3D},
	}
	for _, f := range fields {
		if f.name == "AudioCodec" && skipAudio {
			continue
		}
		if f.s != "" && f.c != "" && !strings.EqualFold(f.s, f.c) {
			// §59.184: Resolution 显示变体等价豁免（4K ≡ 2160p）
			if f.name == "Resolution" && resolutionEquivalent(f.s, f.c) {
				continue
			}
			// §59.185: SourceType 书写变体等价豁免（UHD Blu-ray ≡ UHD BluRay）
			if f.name == "SourceType" && sourceTypeEquivalent(f.s, f.c) {
				continue
			}
			// §59.30: Specification 等价组豁免（WEB-DL ≈ WEBRip 站点标注差异）
			if f.name == "Specification" && specEquivalent(f.s, f.c) {
				continue
			}
			// §59.30: VideoCodec 等价组豁免（x265 ≈ HEVC 标注习惯差异）
			if f.name == "VideoCodec" && codecEquivalent(f.s, f.c) {
				continue
			}
			return true
		}
	}
	return false
}

// techProfileVersionDefined 规则 B：候选有版本定义 Token（ReleaseVersion/RegionCode/SourcePlatform）而源无。
// stripBracketsForVersionCheck §59.181: 剥离站点标签括号内容（半角[]与全角【】）。
// 副标题匹配路径加固：优堡 "[热门]【DIY 原盘 00884】正义的慈悲"（tid=109323 实证）
// 中的 【DIY 原盘 00884】 是站点分类标签，与资源元数据无关；其中的 "DIY" 在
// 英文主标题路径被 splitMedium 消费为 "Blu-ray DIY"，但中文副标题路径无
// Blu-ray 上下文，"DIY" 绕过媒介消费误入 platform 词典 → SourcePlatform="DIY"
// → techProfileVersionDefined 误判版本差异 → fuzzy 被阻断 → size_miss。
// 剥离括号内容消除噪音，保留两条匹配路径各自的正确行为。
var reBracketContent = regexp.MustCompile(`\[[^\]]*\]|【[^】]*】`)

func stripBracketsForVersionCheck(s string) string {
	return reBracketContent.ReplaceAllString(s, "")
}

func techProfileVersionDefined(src titleparser.TechProfile, candidateTitle string) bool {
	candidateTitle = stripBracketsForVersionCheck(candidateTitle)
	cand := titleparser.ParseTitleTech(candidateTitle)
	// §59.191 b: EditionInfo（营销版式词 Remaster/Ultimate Cut/Anniversary…）
	// 移出反驳——本地中文命名常省略版式词、站方名常更全（侠女/宾虚/卡萨布兰卡
	// 反向案三连实证系统性误杀）；物理身份交给 size 显示等价指纹。
	// ReleaseVersion（REPACK/PROPER 重发布标记）保留——重新打包≠同发布。
	if cand.ReleaseVersion != "" && src.ReleaseVersion == "" {
		return true
	}
	if cand.RegionCode != "" && src.RegionCode == "" {
		return true
	}
	if cand.SourcePlatform != "" && src.SourcePlatform == "" {
		return true
	}
	// §59.214: 品牌反驳移出本函数相对化——绝对反驳在密阳案假杀唯一正确
	// 候选（tid=2670 CC 版即本地版本，中文名不写 CC 是命名习惯；本地
	// size 差 42KB 未走门6 短路实证）。品牌词仅在候选集内存在同组同窗
	// 非品牌兄弟时才构成判别器（克拉之膝双版消歧），由调用方集合级判定。
	return false
}

// isBrandEdition §59.207: 碟片发行商品牌 EditionInfo 判定（血统类）。
func isBrandEdition(ed string) bool {
	if ed == "" {
		return false
	}
	lower := strings.ToLower(ed)
	for _, b := range []string{"criterion", "moc", "wac", "warner archive", "cc"} { // §59.514: CC 归一值（词表三 pattern 统一后品牌反驳适配）
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

func VerifyMatch(results []*model.SeedingSearchResult, groupName string, sourceSize int64) *L2MatchResult {
	match, _ := VerifyMatchWithStats(results, groupName, sourceSize)
	return match
}

func VerifyMatchWithTruncationCheck(results []*model.SeedingSearchResult, groupName string, sourceSize int64) (*L2MatchResult, *MatchFilterStats) {
	return VerifyMatchWithTruncationCheckAndSource(results, groupName, sourceSize, "")
}

// VerifyMatchWithTruncationCheckAndSource §59.184 消融：三态化后组名不可见=中性
// （CSS 截断/中文标题自然放行），空组名重跑与截断豁免条款冗余，收敛为直接委托。
// 保留签名兼容既有调用方（orphan/reseed/metadata）。
func VerifyMatchWithTruncationCheckAndSource(results []*model.SeedingSearchResult, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, *MatchFilterStats) {
	return VerifyMatchWithStatsAndSource(results, groupName, sourceSize, sourceTitle)
}

// AudioConflictCandidates §59.36 修订: 从候选中提取"仅因音频 token 冲突被拒"的
// ambiguous 候选（组名/标题相关性/size 全通过，视频编码/HDR/规格等其余字段无冲突）。
// 供元数据获取层 MI 仲裁；辅种/注入路径不消费（严格性保护）。
func AudioConflictCandidates(results []*model.SeedingSearchResult, groupName string, sourceSize int64, sourceTitle string) []*L2MatchResult {
	var srcProfile *titleparser.TechProfile
	if sourceTitle != "" {
		p := titleparser.ParseTitleTech(sourceTitle)
		srcProfile = &p
	}
	meaningfulWords := extractMeaningfulTitleWords(sourceTitle, groupName)
	sourceCJK := extractCJKSubstrings(sourceTitle)
	var out []*L2MatchResult
	for _, r := range results {
		if r.TorrentID == "" || r.Size <= 0 {
			continue
		}
		if groupName != "" && !strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
			if !strings.Contains(r.Title, "..") || !hasResolutionToken(r.Title) {
				continue
			}
		}
		if sourceTitle != "" && !titleKeywordRelevant(meaningfulWords, sourceCJK, sourceTitle, r.Title) {
			continue
		}
		if srcProfile == nil {
			continue
		}
		// 仅音频冲突（其余字段无冲突）才可仲裁；其他冲突仍出局
		if techProfileConflictFields(*srcProfile, r.Title, true) {
			continue // skipAudio 下仍冲突 = 视频/HDR/规格冲突，不可仲裁
		}
		if !techProfileConflictFields(*srcProfile, r.Title, false) {
			continue // 全字段无冲突——这不是冲突候选（正常匹配路径已处理）
		}
		out = append(out, &L2MatchResult{TorrentID: r.TorrentID, Title: r.Title, Size: r.Size})
	}
	return out
}

// ResolveAudioConflict §59.36 修订: MI 仲裁三级链——音频 token 冲突候选的仲裁。
//
// 证据可靠性: local MI（真测量）> 站内 MI（发布者粘贴）> token（描述）。
// 源侧必须是 local MI（双侧"声称 MI"比对会假确定性——源站 MI 贴错场景）。
//
// 参数:
//   - candidates: AudioConflictCandidates 提取的 ambiguous 候选（主轮+loose 轮合并）
//   - sourceLocalMI: 源侧本地文件 MediaInfo 文本（空 = 不可得 → 全体降级②）
//   - candidateMI: 候选 tid → 站内 MI 文本（fetcher 抓详情页现场收集）
//
// 返回: 仲裁胜出候选（①MI 一致 or ②降级放行），nil = 无可放行候选。
// AudioCodec canonical 比较（Atmos 不参与）；任一侧解析空值 = 证据不足不判拒（走②）。
func ResolveAudioConflict(candidates []*L2MatchResult, sourceLocalMI string, candidateMI map[string]string) *L2MatchResult {
	if len(candidates) == 0 {
		return nil
	}
	// ① 源 local MI 可得 → 仲裁路径
	srcAudio := ""
	if sourceLocalMI != "" {
		srcAudio = titleparser.ExtractMediaInfo(sourceLocalMI).AudioCodec
	}
	if sourceLocalMI != "" && srcAudio != "" {
		var fallback *L2MatchResult
		for _, c := range candidates {
			mi := candidateMI[c.TorrentID]
			if mi == "" {
				if fallback == nil {
					fallback = c // 候选 MI 缺失 → ②降级（记首个，一致者优先后被覆盖）
				}
				continue
			}
			candAudio := titleparser.ExtractMediaInfo(mi).AudioCodec
			if candAudio == "" {
				if fallback == nil {
					fallback = c // 解析空值 = 证据不足 → ②
				}
				continue
			}
			if strings.EqualFold(srcAudio, candAudio) {
				return c // MI 一致 → 同资源，确定性胜出
			}
			// MI 不一致 → 该候选出局（真不同版本）
		}
		return fallback
	}
	// ② 源 local MI 不可得 → skipAudio 盲放行（v0.0.644 行为，首个候选）
	return candidates[0]
}

func SearchAndVerifyMatch(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, error) {
	m, _, err := SearchAndVerifyMatchWithResults(ctx, adapter, config, keyword, groupName, sourceSize, sourceTitle)
	return m, err
}

// SearchAndVerifyMatchWithResults §59.36 修订: 额外返回主轮（area=0）原始搜索结果，
// 供元数据获取层收集音频冲突候选做 MI 仲裁（主轮候选相关性最强，不可丢）。
// 辅种/注入路径用 SearchAndVerifyMatch（行为不变）。
func SearchAndVerifyMatchWithResults(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, []*model.SeedingSearchResult, error) {
	if keyword == "" {
		return nil, nil, nil
	}
	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		return nil, nil, err
	}
	match, _ := VerifyMatchWithTruncationCheckAndSource(results, groupName, sourceSize, sourceTitle)
	var allResults []*model.SeedingSearchResult
	if match == nil {
		allResults = results
	}

	// §59.26 CJK 降级重试：混合关键词（中文+英文）在部分站点（如 keepfrds
	// 标题=中文/副标题=英文）search_area=0 下跨字段无法命中。
	// 无结果且关键词同时含 CJK 和 ASCII 词时，分别用两种纯词重搜：
	//   ① 剥离 CJK（保留英文，适用标题=英文的站点）
	//   ② 剥离 ASCII（保留中文，适用标题=中文的站点，如 keepfrds 互换前）
	if match == nil && hasCJKWord(keyword) {
		for _, retryKW := range []string{stripCJKWords(keyword), stripASCIIWords(keyword)} {
			if retryKW == "" || retryKW == keyword {
				continue
			}
			// §59.196: 纯 CJK 重试词（站标题索引英文、中文副标题不在标题索引的
			// 形态——springsunday 侠女案）追加 area=1 简介搜索一轮：简介含中文
			// 片名可命中，严格验证（size+组名确认）把关。
			opts := []*model.SearchOptions{nil}
			if hasCJKWord(retryKW) && stripASCIIWords(retryKW) == retryKW {
				opts = append(opts, &model.SearchOptions{SearchArea: "1"})
			}
			for _, opt := range opts {
				retry, rErr := adapter.SearchTorrents(ctx, config, retryKW, opt)
				if rErr != nil {
					continue
				}
				allResults = append(allResults, retry...)
				if m, _ := VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle); m != nil {
					return m, nil, nil
				}
			}
		}
	}

	// §59.184 B 链内补线：B1 形态（keyword 无 CJK 且 sourceTitle 有 CJK——提取期
	// 剥过中文前缀的签名）→ 从 sourceTitle 提取前导中文段补一轮 area=0（中文标题
	// 站索引中文名）。英线命中短路则本线永不触发；与 area=1 正交。
	if match == nil && !hasCJKWord(keyword) && hasCJKWord(sourceTitle) {
		if cnKW := leadingCJKSegment(sourceTitle); cnKW != "" && cnKW != keyword {
			if retry, rErr := adapter.SearchTorrents(ctx, config, cnKW, nil); rErr == nil && len(retry) > 0 {
				allResults = append(allResults, retry...)
				if m, _ := VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle); m != nil {
					return m, nil, nil
				}
			}
		}
	}

	// §59.30 area=1 降级重试：部分站点（keepfrds）英文种子名不在标题索引，
	// 纯英文关键词（剧集 WEB-DL 类）search_area=0 搜不到。用简介字段重搜一轮。
	if match == nil {
		if retry, rErr := adapter.SearchTorrents(ctx, config, keyword, &model.SearchOptions{SearchArea: "1"}); rErr == nil && len(retry) > 0 {
			allResults = append(allResults, retry...)
			if m, _ := VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle); m != nil {
				return m, nil, nil
			}
		}
	}
	return match, allResults, nil
}

// hasResolutionWord 判断标题是否含分辨率词（2160p/1080p/720p 等）。
func hasResolutionToken(s string) bool {
	lower := strings.ToLower(s)
	for _, kw := range []string{"2160p", "1080p", "1080i", "720p", "576p", "480p", "1440p", "4320p"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// SearchAndVerifyLoose §59.30: 元数据获取专用宽松验证——跳过 size 匹配，
// 仅按 组名 + 标题相关性 + TechProfile 冲突校验。适用站内同资源重发
// （REPACK 替换）导致 size 与本地不一致的场景。禁止用于注入决策
// （注入必须走 VerifyMatchWithTruncationCheckAndSource + ValidateInjection）。
// searchRoundsLoose §59.30/§59.477: 宽松轮关键词渐进降级 rounds——
// 候选筛选由 pick 注入（视频 loosePick / 音乐 musicLoosePick）。
func searchRoundsLoose(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword string, pick func([]*model.SeedingSearchResult) *L2MatchResult) *L2MatchResult {
	if keyword == "" {
		return nil
	}
	// §59.30 渐进降关键词：NexusPHP 多词 AND 语义，长关键词（含分辨率/媒介词）
	// 在简介搜索（area=1）下可能全词无法命中。逐轮缩短：
	//   ① 原关键词（area1）
	//   ② 剥离分辨率词（area1）
	//   ③ 短词：剥 CJK+媒介+年份（area0 → area1）
	// §59.30 频率收敛：keepfrds 等站对连续搜索静默限流（返回默认列表），
	// loose 轮请求上限 4 次，命中即停。
	orderedKWs := []string{keyword}
	if kw2 := stripResolutionTokens(keyword); kw2 != "" && kw2 != keyword {
		orderedKWs = append(orderedKWs, kw2)
	}
	short := StripYearToken(stripCJKWords(stripResolutionTokens(stripMediumTokensLoose(keyword))))
	if short == "" || short == keyword {
		if hasCJKWord(keyword) {
			short = stripCJKWords(keyword)
		}
	}
	// §59.196: 短词标题锚守卫——短词剥完仅剩技术词（无 CJK 且无 ≥4 字母词，
	// 如 "1970 1080p"）时中止该轮：光技术词 AND 搜索返回同年同规格任意种子
	// （侠女 1970×Valerie 1970 错挂形态），技术词不构成资源身份。
	if short != "" && short != keyword && !keywordHasTitleAnchor(short) {
		short = ""
	}
	if short != "" && short != keyword {
		orderedKWs = append(orderedKWs, short)
	}

	type round struct {
		kw  string
		opt *model.SearchOptions
	}
	rounds := []round{
		{keyword, &model.SearchOptions{SearchArea: "1"}},
	}
	if len(orderedKWs) > 1 {
		rounds = append(rounds, round{orderedKWs[1], &model.SearchOptions{SearchArea: "1"}})
	}
	if len(orderedKWs) > 2 {
		rounds = append(rounds, round{orderedKWs[2], nil})
		rounds = append(rounds, round{orderedKWs[2], &model.SearchOptions{SearchArea: "1"}})
	}
	for i, rd := range rounds {
		if i >= 4 {
			break
		}
		results, err := adapter.SearchTorrents(ctx, config, rd.kw, rd.opt)
		if err != nil || len(results) == 0 {
			continue
		}
		if m := pick(results); m != nil {
			return m
		}
	}
	return nil
}

// SearchAndVerifyLoose §59.30 宽松降级（视频）——size 豁免·标题相关性
// 放宽。"元数据与版本无关"假设=视频语义（同作品不同版本）。
func SearchAndVerifyLoose(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, groupName, sourceTitle string) *L2MatchResult {
	return searchRoundsLoose(ctx, adapter, config, keyword, func(results []*model.SeedingSearchResult) *L2MatchResult {
		return loosePick(results, groupName, sourceTitle)
	})
}

// SearchAndVerifyMusicLoose §59.477 音乐宽松轮——关键词降级 rounds 复用
// （§59.30 同款），pick 换音乐门：
//  1. 标题强匹配：词边界命中 ≥max(2, 过半词)——防 any-word 子串伪命中
//     （"time"⊆"Times" 单词放行 Prince 案实证）；CJK 段 ≥2 字任一命中
//     （中文专辑名）等价强匹配
//  2. size 带 [0.25x, 4x]：同专辑格式变体（FLAC/MP3 双版）放行·
//     跨专辑/分卷拒绝（61GB 源×Vol.41 0.36GB=170x 案）
//  3. 无锚词（无有效词且无 CJK）不进宽松——宁漏勿误
//
// 视频 SearchAndVerifyLoose 原样零触碰。
func SearchAndVerifyMusicLoose(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, sourceTitle string, sourceSize int64) *L2MatchResult {
	return searchRoundsLoose(ctx, adapter, config, keyword, func(results []*model.SeedingSearchResult) *L2MatchResult {
		return musicLoosePick(results, sourceTitle, sourceSize)
	})
}

// musicLoosePick §59.477 音乐宽松轮候选筛选（纯函数可测）。
func musicLoosePick(results []*model.SeedingSearchResult, sourceTitle string, sourceSize int64) *L2MatchResult {
	words := extractMeaningfulTitleWords(sourceTitle, "")
	cjk := extractCJKSubstrings(sourceTitle)
	if len(words) == 0 && len(cjk) == 0 {
		return nil // 无锚词不进宽松（宁漏勿误）
	}
	for _, r := range results {
		if r.TorrentID == "" || r.Size <= 0 {
			continue
		}
		// size 带 [0.25x, 4x]（源与候选双向）
		if sourceSize > 0 {
			if r.Size*4 < sourceSize || r.Size > sourceSize*4 {
				continue
			}
		}
		if !musicTitleStrongMatch(words, cjk, r.Title) {
			continue
		}
		return &L2MatchResult{TorrentID: r.TorrentID, Title: r.Title, Size: r.Size}
	}
	return nil
}

// musicTitleStrongMatch §59.477 音乐标题强匹配：词边界（防子串伪命中）
// + 命中数阈值 max(2, 过半)；CJK 段 ≥2 字任一命中等价强匹配。
func musicTitleStrongMatch(words []string, cjk []string, candidateTitle string) bool {
	candLower := strings.ToLower(candidateTitle)
	if len(words) > 0 {
		// §59.486: 门下限与关键词词数挂钩——单词关键词（厂牌/艺术家名
		// 下载器名就是一词·Monstercat 83.5GB 案）max(2,..) 恒 2 → 单词
		// 1 命中永拒全候选灭（iyuu fallback exhausted 终态）。单词整词
		// 边界命中=强标识（词边界已防子串伪命中·size 带兜底）；多词
		// 维持 max(2, 过半) 不变（§59.477 毒案防线零放松）。
		need := max(2, (len(words)+1)/2)
		if len(words) < need {
			need = len(words)
		}
		hits := 0
		for _, w := range words {
			if wordBoundaryIn(w, candLower) {
				hits++
			}
		}
		if hits >= need {
			return true
		}
	}
	for _, s := range cjk {
		if len([]rune(s)) >= 2 && strings.Contains(candidateTitle, s) {
			return true
		}
	}
	return false
}

// wordBoundaryIn §59.477: 词边界包含（w 已小写）——"time" 不命中
// "Times"（边界）·命中 "time" 独立词形。
func wordBoundaryIn(w, lowerText string) bool {
	if w == "" {
		return false
	}
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`).MatchString(lowerText)
}

// loosePick 宽松候选筛选（组名/标题相关/TechProfile 三层，各含标注差异豁免）。
func loosePick(results []*model.SeedingSearchResult, groupName, sourceTitle string) *L2MatchResult {
	for _, r := range results {
		if r.TorrentID == "" {
			continue
		}
		// §59.196: 双盲拒绝——源标题含 CJK×候选无 CJK×源无 meaningful 英文词
		// （纯中文名种子）：titleKeywordRelevant 尾部盲放行（return true）曾致
		// 同年同组不同电影错挂（侠女×Valerie 案）。loose 宁漏勿错——正确路径
		// 是主轮（纯 CJK area=1 补线 + size/组名确认）或 ①coverage_tid 直达。
		if hasCJKWord(sourceTitle) && !hasCJKWord(r.Title) &&
			len(extractMeaningfulTitleWords(sourceTitle, groupName)) == 0 {
			continue
		}
		// 组名（CSS 截断标题放宽，与主路径一致；
		// §59.30 纯中文标题（如 keepfrds 部分行 </a> 后无英文名）不含组名，
		// 候选含源标题 CJK 片段时视为同资源放行）
		if groupName != "" && !strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
			truncRelaxed := strings.Contains(r.Title, "..") && hasResolutionToken(r.Title)
			cjkRelaxed := false
			for _, cjk := range extractCJKSubstrings(sourceTitle) {
				if len([]rune(cjk)) >= 2 && strings.Contains(r.Title, cjk) {
					cjkRelaxed = true
					break
				}
			}
			if !truncRelaxed && !cjkRelaxed {
				continue
			}
		}
		// 标题相关性 + TechProfile
		// §59.30: 候选纯中文标题（无英文词）时 meaningfulWords 英文匹配必败，
		// 需补 CJK 分支判定（源中文名命中候选中文标题）
		if !titleKeywordRelevant(extractMeaningfulTitleWords(sourceTitle, groupName), extractCJKSubstrings(sourceTitle), sourceTitle, r.Title) {
			cjkHit := false
			for _, cjk := range extractCJKSubstrings(sourceTitle) {
				if len([]rune(cjk)) >= 2 && strings.Contains(r.Title, cjk) {
					cjkHit = true
					break
				}
			}
			if !cjkHit {
				continue
			}
		}
		// §59.30: CSS 截断标题（含 ..）的 token 不完整（如 DTS-H.. 解析成 DTS），
		// TechProfile 比较失真 → 跳过冲突检查，靠组名+标题相关性+size（主轮）兜底
		// §59.36: 音频冲突豁免（元数据获取场景）——站点数据源头的文件名/标题
		// 音轨标注不一致（FRDS 挽救计划案例：文件名 DTS-HD MA、站内标题 TrueHD7.1，
		// .torrent 实证同一资源，内容物实为 TrueHD+国配 AC-3）。音频规格差异
		// 不影响海报/声明/简介正确性；注入层 ValidateInjection 独立兜底。
		if !strings.Contains(r.Title, "..") {
			p := titleparser.ParseTitleTech(sourceTitle)
			if techProfileConflictFields(p, r.Title, true) {
				continue
			}
		}
		return &L2MatchResult{TorrentID: r.TorrentID, Title: r.Title, Size: r.Size}
	}
	return nil
}

// hasCJKWord 判断关键词是否含 CJK 词（触发降级重试的条件）。
func hasCJKWord(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// CompareSizeDisplay §59.184 v3: 显示舍入等价 + 1MB 绝对地板（替代 2% 容差）。
//
// 语义：站方显示值 = true_bytes 按其显示档（1024 基两位小数）舍入的结果，
// parseSizeStr 反解析出的 resultBytes 是舍入近似值；孤儿/本地震磁盘字节为真值。
// 同一种子两侧差的数学边界 = ½ 显示单位，按候选解析值量级重建显示档：
//
//	≥1TiB → ±0.005×1024⁴（±5.37GB）；≥1GiB → ±0.005×1024³（±5.37MB）；MB 档 → ±5KB
//
// 1MB 地板兜 2% 的真实成因（v0.0.484 考古勘误）：跨站种子文件集微差
// （站方 NFO ~52KB，647.31 vs 647.26 MiB 案）——附属文件场景不存在，此为真因。
// Just Mercy 实数锚：109323 差 5.09MB（窗内）/ 108076 差 5.38MB（窗外）。
// 假设：站方显示 1024 基与 parseSizeStr 同基；若个别站 1000 基会全量失配
// （偏差 7.4%，2% 时代同样失配）——非新增风险，全量失配时第一诊断项=查显示基。
func CompareSizeDisplay(sourceBytes, resultBytes int64) bool {
	if sourceBytes <= 0 || resultBytes <= 0 {
		return false
	}
	diff := sourceBytes - resultBytes
	if diff < 0 {
		diff = -diff
	}
	window := sizeDisplayWindow(resultBytes)
	if floor := int64(1024 * 1024); window < floor {
		window = floor
	}
	return diff <= window
}

// sizeDisplayWindow 按候选解析值量级重建站方显示档的舍入半窗（1024 基）。
func sizeDisplayWindow(parsedBytes int64) int64 {
	switch {
	case parsedBytes >= 1024*1024*1024*1024:
		return 1024 * 1024 * 1024 * 1024 / 200
	case parsedBytes >= 1024*1024*1024:
		return 1024 * 1024 * 1024 / 200
	default:
		return 1024 * 1024 / 200
	}
}
