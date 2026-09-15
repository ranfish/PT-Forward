package titleparser

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseTitleTech 从标题字符串解析出完整的 TechProfile（标题源，§56.34 步骤 2）。
//
// 在 ParseTitle（TitleComponents）基础上增强提取：
//   - EditionInfo（v1.05 其他信息 9 类优先级）
//   - SourceType + Specification（从 Medium 拆分）
//   - AudioChannels / AudioTechnology / AudioTracks（标题 fallback，MediaInfo 为准时被覆盖）
//
// ParseTitle 保持不变（兼容现有使用者）。本函数为 TechProfile 体系的增强解析器。
func ParseTitleTech(title string) TechProfile {
	tc := ParseTitle(title)
	p := TechProfileFromTitle(tc)

	p.EditionInfo = extractEditionInfo(title)
	// §59.76 步骤 3 / §59.184 G2: ReleaseVersion 并入 EditionInfo（空时补，
	// 不覆盖既有 edition token）。单点合并——BuildTechProfile 同款逻辑变幂等；
	// 验证路径 techProfileVersionDefined 规则 B 对无括号 REPACK 由此生效
	// （§59.181 挂账兑现：此前仅重组流合并，验证流 EditionInfo 恒空）。
	if p.EditionInfo == "" && p.ReleaseVersion != "" {
		p.EditionInfo = p.ReleaseVersion
	}
	p.SourceType, p.Specification = splitMedium(tc.Medium)
	// §59.226 附九: ST 反哺派生——流媒体平台词非空 ∧ ST 空 → ST="WEB"
	// （"Movie.1080p.NF.DDP5.1" 平台词直接当源形态——平台词本身证明 WEB
	// 语境）。电视台族（BBC/HBO 等）不派生（内容可 HDTV 直录可流媒体
	// 不确定——SPEC 有词走 splitMedium 既有规则）。
	if p.SourceType == "" && isStreamingPlatform(tc.SourcePlatform) {
		p.SourceType = "WEB"
	}
	p.AudioChannels = extractAudioChannelsFromTitle(title)
	p.AudioTechnology = extractAudioTechnologyFromTitle(title)
	p.AudioTracks = extractAudioTracksFromTitle(title)
	p.Stereo3D = extractStereo3D(title)
	p.FrameRate = tc.FrameRate // §59.226 附二十一: 标题 60fps 形态兜底（MI 合并时覆盖）

	return p
}

// extractStereo3D §59.197: 3D 封装词提取（仅显式形态参与反驳——"3D" 泛词
// 与省略形态返回空值不参与）。族归一：HSBS/H-SBS/Half-SBS→HSBS；
// HOU/H-OU/Half-OU→HOU；裸 SBS/OU 独立值（半幅/全幅不同封装）。
// 300勇士案：站内同资源 HSBS/HOU 两版同体积同组——无此字段则验证链全盲、
// 结果顺序定生死（HOU 在前即错配注入，piece hash 不匹配 recheck 失败）。
var reStereo3D = regexp.MustCompile(`(?i)\b(?:half[-_.\s]?|h[-_.\s]?)?(sbs|ou)\b`)

// streamingPlatformSet §59.226 附九: 流媒体平台 canonical 集（电视台族
// BBC/HBO/AMC 等不在内——HDTV 广播语义）。platform.json 的 requires="web"
// 词条即流媒体族——此处列高频 canonical（词表 223 条全量判定在 dict 层，
// 这里是 ST 反哺的高频快集）。
var streamingPlatformSet = map[string]bool{
	"NF": true, "AMZN": true, "HMAX": true, "Hulu": true, "DSNP": true,
	"ATVP": true, "iP": true, "Peacock": true, "PMTP": true, "STAN": true,
	"ZEE5": true, "SNYLIV": true, "JHS": true, "SNXT": true, "HOTSTAR": true,
	"VOOT": true, "AHA": true, "JIOT": true,
}

// isStreamingPlatform 流媒体平台判定（ST 反哺门）。
func isStreamingPlatform(platform string) bool {
	return platform != "" && streamingPlatformSet[platform]
}

func extractStereo3D(title string) string {
	for _, m := range reStereo3D.FindAllStringSubmatch(title, -1) {
		full := strings.ToLower(m[0])
		switch strings.ToUpper(m[1]) {
		case "SBS":
			if strings.Contains(full, "half") || strings.Contains(full, "h") {
				return "HSBS"
			}
			return "SBS"
		case "OU":
			if strings.Contains(full, "half") || strings.Contains(full, "h") {
				return "HOU"
			}
			return "OU"
		}
	}
	return ""
}

// editionPattern 版本信息匹配规则。
type editionPattern struct {
	re   *regexp.Regexp
	name string
}

// editionPatterns v1.05 其他信息 9 类优先级（长 pattern 在前，避免短 pattern 子串重复匹配）。
// 分隔符用 [-_.\s]* 支持 PT 标题常见的点分隔/空格分隔/连字符。
var editionPatterns = []editionPattern{
	// 剪辑版本（优先级 1）
	{regexp.MustCompile(`(?i)\bDirector'?s?[-_.\s]*Cut\b`), "Director's Cut"},
	{regexp.MustCompile(`(?i)\bUltimate[-_.\s]*(?:Cut|Edition)\b`), "Ultimate Cut"},
	{regexp.MustCompile(`(?i)\bExtended[-_.\s]*Cut\b`), "Extended Cut"},
	{regexp.MustCompile(`(?i)\bTheatrical[-_.\s]*Cut\b`), "Theatrical Cut"},
	{regexp.MustCompile(`(?i)\bUncensored\b`), "Uncensored"},
	{regexp.MustCompile(`(?i)\bUnrated\b`), "Unrated"},
	{regexp.MustCompile(`(?i)\bUncut\b`), "Uncut"},
	{regexp.MustCompile(`(?i)\bExtended\b`), "Extended"},
	{regexp.MustCompile(`(?i)\bTheatrical\b`), "Theatrical"},
	// 2in1（优先级 2）
	{regexp.MustCompile(`(?i)\b[23][-_\s]?in[-_\s]?1\b`), "2in1"},
	// 版本（优先级 3）
	{regexp.MustCompile(`(?i)\b\d+(?:st|nd|rd|th)?[-_.\s]*Anniversary(?:[-_.\s]*Edition)?\b`), "Anniversary Edition"},
	{regexp.MustCompile(`(?i)\b4K[-_.\s]*Remaster(?:ed)?\b`), "4K Remaster"},
	{regexp.MustCompile(`(?i)\bRemastered\b`), "Remastered"},
	{regexp.MustCompile(`(?i)\bRemaster\b`), "Remaster"},
	{regexp.MustCompile(`(?i)\bLimited[-_.\s]*Edition\b`), "Limited Edition"},
	{regexp.MustCompile(`(?i)\bSpecial[-_.\s]*Edition\b`), "Special Edition"},
	{regexp.MustCompile(`(?i)\bCollector'?s?[-_.\s]*Edition\b`), "Collector's Edition"},
	{regexp.MustCompile(`(?i)\bCriterion[-_.\s]*Collection\b`), "Criterion Collection"},
	{regexp.MustCompile(`(?i)\bCriterion\b`), "Criterion"},
	// §59.76: v1.05 W.9 碟片发行商品牌（原盘类内容分发方）——CC 已由 Criterion 覆盖
	{regexp.MustCompile(`(?i)\bMoC\b`), "MoC"},
	// §59.219: BFI（British Film Institute 碟标——霸王别姬案：曾误入
	// platform.json 流媒体槽致候选 SourcePlatform=BFI × 源"BFI修复版"粘连
	// 提取失败 → 单侧 platform 反驳假杀）
	{regexp.MustCompile(`(?i)(?:^|[.\s\-_])BFI(?:[.\s\-_]|$)`), "BFI"},
	{regexp.MustCompile(`(?i)\bWAC\b`), "WAC"},
	{regexp.MustCompile(`(?i)\bMasters[-_.\s]*of[-_.\s]*Cinema\b`), "MoC"},
	{regexp.MustCompile(`(?i)\bWarner[-_.\s]*Archive(?:[-_.\s]*Collection)?\b`), "WAC"},
	// 特殊比例（优先级 4）
	{regexp.MustCompile(`(?i)\bIMAX[-_.\s]*Enhanced\b`), "IMAX Enhanced"},
	{regexp.MustCompile(`(?i)\bIMAX\b`), "IMAX"},
	{regexp.MustCompile(`(?i)\bOpen[-_.\s]*Matte\b`), "Open Matte"},
	// §59.226 附十九: MAR（Modified Aspect Ratio——v1.05 特殊比例第三词）
	{regexp.MustCompile(`(?i)(?:^|[.\s\-_])MAR(?:[.\s\-_]|$)`), "MAR"},
	// Hybrid（优先级 5）
	{regexp.MustCompile(`(?i)\bHybrid\b`), "Hybrid"},
	// §59.190 ①: 中文版式词——中英同词异形（本地"2K修复版" vs 站方"2K.REMASTERED"
	// 同一版本，爱情万岁 tid=328261 三连误杀实证）归一英文 canonical，
	// 规则 B"候选有源无"不再误触发（双侧均有值即不反驳）
	{regexp.MustCompile(`修复版|修復版|重制版|重製版|数字修复|數字修復`), "Remastered"},
	// §59.191: 版式词族补全（守望者 终极剪辑版/月光光 加长版案）——b 方案后
	// 反驳不依赖，展示/重组单通道用
	{regexp.MustCompile(`终极剪辑版?|終極剪輯版?`), "Ultimate Cut"},
	{regexp.MustCompile(`导演剪辑版?|導演剪輯版?`), "Director's Cut"},
	{regexp.MustCompile(`加长版?|加長版?`), "Extended"},
	// MiniBD（优先级 9）
	{regexp.MustCompile(`(?i)\bMiniBD\b`), "MiniBD"},
	// CC = Criterion Collection 缩写，PT 标题中出现在年份后片源前（如 1972.CC.BluRay）
	{regexp.MustCompile(`(?i)(?:^|[.\s\-_])CC(?:[.\s\-_]|$)`), "Criterion Collection"},
}

// extractEditionInfo 从标题提取版本信息。
//
// 采用"消费式匹配"：长 pattern 先匹配并消费区域（替换为空格），
// 防止短 pattern 对同一文本的子串重复匹配（如 "Extended Cut" 被 "Extended" 重复提取）。
func extractEditionInfo(title string) string {
	var found []string
	seen := map[string]bool{}
	work := title
	for _, p := range editionPatterns {
		matches := p.re.FindAllStringIndex(work, -1)
		if len(matches) == 0 {
			continue
		}
		if !seen[p.name] {
			found = append(found, p.name)
			seen[p.name] = true
		}
		for _, loc := range matches {
			for i := loc[0]; i < loc[1]; i++ {
				work = work[:i] + " " + work[i+1:]
			}
		}
	}
	return strings.Join(found, " ")
}

// splitMedium 从 TitleComponents.Medium 组合值拆分片源类型 + 规格。
//
// v1.05 区分：
//   - 片源类型（source_type）：原盘带连字符（Blu-ray/UHD Blu-ray）vs 压制无连字符（BluRay/UHD BluRay）
//   - 规格（specification）：Remux/WEB-DL/WEBRip/HDTV/UHDTV
//
// 原盘类和 Encode（压制）类规格均为空（v1.05：Encode 规格为空，由片源写法区分）。
// WEB/HDTV 类不填片源类型（由 source_platform 标识来源）。
func splitMedium(medium string) (sourceType, specification string) {
	if medium == "" {
		return "", ""
	}
	upper := strings.ToUpper(medium)
	// 带分隔的 RAY → 原盘写法；纯 BLURAY → 压制写法（v1.05 原盘/压制以连字符区分）
	discSuffix := strings.Contains(upper, "BLU-RAY") || strings.Contains(upper, "BLU.RAY")

	switch {
	case strings.Contains(upper, "WEB-DL") || strings.Contains(upper, "WEBDL"):
		specification = "WEB-DL"
		// §59.226 附六 B-lite: ST 语义扩展——WEB-DL/WEBRip 落规格但片源
		// 恒空的不对称归属修正；流媒体平台词本身就是 WEB 语境证据
		// （"Movie.1080p.NF.DDP5.1" 无 WEB-DL 词形态的 ST 反哺同族）
		sourceType = "WEB"
	case strings.Contains(upper, "WEBRIP"):
		specification = "WEBRip"
		sourceType = "WEB" // §59.226 附六
	case strings.Contains(upper, "REMUX"):
		specification = "Remux"
	case strings.Contains(upper, "UHDTV"):
		specification = "UHDTV"
		sourceType = "UHDTV" // §59.226 附六: 源=规格合一（SPEC 保留——canonical 判定链吃 spec）
	case strings.Contains(upper, "HDTV"):
		specification = "HDTV"
		sourceType = "HDTV" // §59.226 附六: 同上——ST/SPEC 同词冗余为必要代价
	case strings.Contains(upper, "BDRIP"):
		specification = "BDRip"
	case strings.Contains(upper, "DVDRIP"):
		specification = "DVDRip"
	case strings.Contains(upper, "TVRIP"):
		specification = "TVRip"
	}

	switch {
	case strings.Contains(upper, "3D BLU"):
		if discSuffix {
			sourceType = "3D Blu-ray"
		} else {
			sourceType = "3D BluRay"
		}
	case strings.Contains(upper, "UHD BLU"):
		if discSuffix {
			sourceType = "UHD Blu-ray"
		} else {
			sourceType = "UHD BluRay"
		}
	case strings.Contains(upper, "BLU-RAY") || strings.Contains(upper, "BLURAY") || strings.Contains(upper, "BLU DIY"):
		if discSuffix {
			sourceType = "Blu-ray"
		} else {
			sourceType = "BluRay"
		}
	case strings.Contains(upper, "HDDVDRIP"):
		sourceType = "HD DVD"
		specification = "HDDVDRip"
	case strings.Contains(upper, "DVD"):
		sourceType = "DVD"
	}

	return sourceType, specification
}

// reAudioChannelsTitle 匹配声道数，后向边界用 [-_.\s]|$ 避免 "2015.1080p" 中 "5.1" 误匹配。
// 不用前向 \b 因为 "DDP5.1" 中 P 和 5 之间不是 \b。
var reAudioChannelsTitle = regexp.MustCompile(`(?i)(1\.0|2\.0|5\.1|7\.1)(?:[-_.\s]|$)`)

// extractAudioChannelsFromTitle 从标题提取声道数（如 "5.1"/"7.1"）。
//
// 值域约束：仅匹配 PT 圈常见的 1.0/2.0/5.1/7.1，避免误匹配版本号等。
func extractAudioChannelsFromTitle(title string) string {
	m := reAudioChannelsTitle.FindStringSubmatch(title)
	if m != nil {
		return m[1]
	}
	return ""
}

// extractAudioTechnologyFromTitle 从标题提取音频技术（对象信息，v1.05 :193）。
// 值域：Atmos / Auro3D（DTS:X 是编码即对象，不在此标）。
// Auro3D 变体：Auro3D / Auro-3D / Auro 3D（§59.41，对齐 MI Commercial name 形态）。
func extractAudioTechnologyFromTitle(title string) string {
	if regexp.MustCompile(`(?i)\bAtmos\b`).MatchString(title) {
		return "Atmos"
	}
	if regexp.MustCompile(`(?i)\bAuro[-. ]?3D\b`).MatchString(title) {
		return "Auro3D"
	}
	return ""
}

var reAudioTracksTitle = regexp.MustCompile(`(?i)\b(\d+)\s*Audios?\b`)

// extractAudioTracksFromTitle 从标题提取音轨数（如 "2Audios"/"3Audios"）。
func extractAudioTracksFromTitle(title string) int {
	m := reAudioTracksTitle.FindStringSubmatch(title)
	if m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return 0
}
