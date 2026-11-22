// Package publish 新发布执行器（§59.156 切片 2，2026-08-31）。
//
// §59.147 配置中心消费流：发布链 DB 优先——切读 sites.publish_form_config，
// 替代 §59.146 灰度 flag（tagConfigFromFlag/settings tag_applier_sites 随本切片退役）。
// 复用组件（§59.148 保留清单）：renderDescription / dedupByPiecesHash /
// CheckPublishEligibility / adapter UploadTorrent / pusher 加种链 / ResourceResolver。
//
// 铁律对齐：零拉取（源 .torrent 走下载器 ExportTorrent 本地导出；元数据走
// torrent_metadata 落库数据 + PTGen 缓存兜底）；pre-audit=推数据拿判定（发布动作前置组成）。
package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/metadata"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
)

// luckptChineseSubtitleHeuristic §59.273→§59.278 终版: 幸运中字启发式补标。
// 判据（用户定案演进）：§59.273 华语区产地门 → §59.278 硬字幕无条件化 →
// 终版产地忽略——副标题含 中字/中英字幕/硬字幕 任一即补 chinese_subtitle
// （MI 无中文字幕轨也补——硬字幕内嵌不体现为字幕轨）；"无中字"否定式剥除。
func luckptChineseSubtitleHeuristic(siteName string, meta *model.TorrentMetadata, tags []string) []string {
	if siteName != "幸运" || meta == nil {
		return tags
	}
	for _, t := range tags {
		if t == "chinese_subtitle" {
			return tags
		}
	}
	// §59.278 终版（用户定案：产地忽略）——副标题含 中字/中英字幕/硬字幕/
	// 特效字幕/特效中字/简繁/简中/繁中/简体/繁体 任一即推断中字（PT 语境
	// 发布者声明的特效字幕默认中文——§59.151 保守假设零案例支撑被推翻；
	// 简繁族=中文字形变体描述。裸"特效"不命中——§59.73 防误判保持）。
	// "无中字"先剥除（§59.277 Deep Water 案语义：否定式声明不得误配 中字 子串）。
	sub := strings.ReplaceAll(meta.Subtitle, "无中字", "")
	for _, kw := range []string{"中字", "中英字幕", "硬字幕", "特效字幕", "特效中字", "简繁", "简中", "繁中", "简体", "繁体"} {
		if strings.Contains(sub, kw) {
			return append(tags, "chinese_subtitle")
		}
	}
	return tags
}

// syncHDRTagFromProfile §59.278: HDR 标签与组装标题 HDR token 对齐。
// tp.HDR（标题/DOM 源——重组 hdr 槽数据源）非空而 MI 推断未产 HDR 族标签时
// 补位（十三猎杀案：标题 HDR10 重组输出含 HDR10，MI 无 HDR 行标签缺失——
// 幸运种审判据"标题包含 HDR10→勾 HDR10"）。已有 HDR 族标签（MI 铁证优先）
// 不覆盖；站方无对应选项时 applier 自然丢弃（跨站安全）。
func syncHDRTagFromProfile(hdr string, tags []string) []string {
	if hdr == "" || hdr == "SDR" {
		return tags
	}
	hdrFamily := map[string]bool{
		"hdr10": true, "hdr10_plus": true, "hdr_vivid": true, "hlg": true, "dolby_vision": true,
	}
	for _, t := range tags {
		if hdrFamily[t] {
			return tags
		}
	}
	var key string
	switch {
	case strings.Contains(hdr, "Vivid"):
		key = "hdr_vivid"
	case strings.Contains(hdr, "DoVi") || strings.Contains(hdr, "DV"):
		key = "dolby_vision"
	case strings.Contains(hdr, "HDR10+"):
		key = "hdr10_plus"
	case strings.Contains(hdr, "HDR"):
		key = "hdr10"
	case strings.Contains(hdr, "HLG"):
		key = "hlg"
	default:
		return tags
	}
	return append(tags, key)
}

// luckptEnglishTagValue §59.277: 幸运英语标签站规——无国语/粤语/中字且英语
// 音轨（english_audio/lucky_english_audio 推断 或 副标题"英语/无中字"声明）
// → 返回英语选项 value（Label 直配——auto:false 无 standard_keys 例外路径）。
// Deep Water 2022 AMZN WEB-DL 种审案（MI 无中文字幕 + 副标题[英语/无中字]）。
func luckptEnglishTagValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tags []string) string {
	if siteName != "幸运" || cfg == nil {
		return ""
	}
	// §59.326 附三补2: 证据判定读 stored 原始推断（meta.Tags）+ 过滤后双源
	// ——assembleTags 按 auto:false 滤掉 english_audio 后再问"有英语吗"
	// 恒 false（Alice 三案：tags=[dolby_vision,hdr10] 英语证据不可见）。
	stored := []string{}
	if meta != nil && meta.Tags != "" {
		if err := json.Unmarshal([]byte(meta.Tags), &stored); err != nil {
			stored = nil
		}
	}
	merged := append(append([]string{}, tags...), stored...)
	// §59.326 附七（用户三规则定案，取代附五"无条件注入"）：
	// 规则 1（人工审核）+规则 3（用户提出，机器实证支持——造雨人
	// 中字+英语同选 passed）：**音轨与字幕物理语义**——国语/粤语音轨
	// 是版本属性（国配/粤配版与英语原声版互斥）→ 硬排斥英语标签；
	// 中文字幕是叠加属性（英语原声+中字=最常见原声中字版，正交）→
	// 不排斥。注入条件=英语证据在场 ∧ 无 chinese_audio ∧ 无
	// cantonese_audio（附五全拦→全放的精细回归：只拦音轨放行字幕）。
	// §59.350（站方规则收紧——243 全检 61 簇冲突+用户实测审核脚本
	// 原文推翻附七字幕正交实证）：中文字幕同样排斥——"英语标签仅在
	// 既没有国语、粤语音轨，也没有中文字幕时才能勾选"。语义回归
	// auto=False 条件标签本义：英语=纯英文无中文内容专版标记（生肉版）。
	for _, t := range merged {
		if t == "chinese_audio" || t == "cantonese_audio" || t == "chinese_subtitle" {
			return "" // 国/粤语音轨或中文字幕在场——站方规则禁勾英语（§59.350）
		}
	}
	hasEnglish := false
	for _, t := range merged {
		if t == "english_audio" || t == "lucky_english_audio" {
			hasEnglish = true
			break
		}
	}
	// 副标题声明兜底：MI 无 Language 行时 english_audio 可能未推断，
	// "英语/无中字"声明是站方判据源
	if !hasEnglish && meta != nil {
		sub := meta.Subtitle
		if strings.Contains(sub, "英语") || strings.Contains(sub, "无中字") {
			hasEnglish = true
		}
	}
	if !hasEnglish {
		return ""
	}
	for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
		if m.Label == "英语" {
			return m.Value
		}
	}
	return ""
}

// csptGenreTagValue §59.400: 财神题材标签站规——description ◎类　　别
// 行含"喜剧"→注入财神"喜剧"标签（Label 直配——label-only 无
// standard_keys 例外路径，luckptEnglishTagValue 站规注入同款先例）。
// 缩小人生案（243 财神种审报"标签与简介类别不一致(喜剧)"）：喜剧是
// PTGen 题材细分（category 列只存大类 movie、canonical 无 tag.comedy
// 键——fetch 推断永不产出）；站方审种脚本用简介◎类别行校验——发布
// 同源消费（确定性正则提取非语义推断，§59.319 附二十七"只消费不
// 推断"合规）。儿童(24)题材标签零案例不预改（按需生长纪律）。
// 正则注：类/别间是全角空格（U+3000）——Go \s 只匹配 ASCII 空白，
// 字符类显式含　。
var reGenreLine = regexp.MustCompile(`◎类[　\s]*别[　\s]*([^\n]+)`)

func csptGenreTagValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) string {
	if siteName != "财神" || cfg == nil || meta == nil || meta.Description == "" {
		return ""
	}
	m := reGenreLine.FindStringSubmatch(meta.Description)
	if len(m) < 2 || !strings.Contains(m[1], "喜剧") {
		return ""
	}
	for _, t := range cfg.ValueMappings[model.FieldDomainTags] {
		if t.Label == "喜剧" {
			return t.Value
		}
	}
	return ""
}

// §59.427 签名扩展：siteName+meta（好多油 TV 双轨分发——站方媒介
// 电影/剧集分立项，分发依据 category=tv_series）。
func (e *PublishExecutor) mediumMappingOf(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	stdKey := ""
	switch strings.ToLower(tp.Specification) {
	case "remux":
		stdKey = "medium.remux"
	case "web-dl", "webdl":
		stdKey = "medium.webdl"
	case "hdtv":
		stdKey = "medium.hdtv"
	case "bdrip", "dvdrip", "tvrip", "encode", "webrip":
		// WEBRip→Encode 槽（§59.166 站方 TYPE_MISMATCH 对照定案·站方媒介槽
		// 体系无 WEBRip 独立槽的翻译映射）。§59.536: 仅源站原生 WEBRip 触发
		// ——MI 改写已撤（源站 WEB-DL→WEB-DL 槽不再落入本分支）。
		// §59.549: 源分立站（劳改所式 WEBRiP/TVRip 独立槽）的精确分流在
		// mediumStdKeyOf 调用后统一改写（见 applyMediumSourceFidelity）。
		stdKey = "medium.encode"
	}
	// §59.380: stdKey 推导抽单源（codec 压制形态联动复用——防双实现漂移）
	stdKey = mediumStdKeyOf(tp, stdKey)
	// §59.549 媒介源保真：源分立站（值集按源分立项——劳改所 BDRip/WEBRiP/
	// TVRip/DVDRiP 四源分立·与通用「Encode 大类」体系不同）优先精确源键·
	// 值集无该槽回退 §59.166 通用翻译（Encode→BDRip·适配期定案「蓝光压制
	// 要求 BDRip」保持——BD 源/无源血统不在改写面）。判据双通道：spec 显式
	// （webrip/tvrip）∨ 血统隐式（encode 判定+SourceType=WEB——标题无
	// webrip 词但血统 WEB·AMZN x265 式标注）。Quacks 案：WEB 源压制被通用
	// 翻译投 BDRip·站方标题检测 WEBRiP=TYPE_MISMATCH。
	stdKey = e.applyMediumSourceFidelity(cfg, tp, stdKey)
	// §59.427: 好多油 TV 双轨分发——站方媒介 Remux/Encode/WEB-DL 各带
	// TV 后缀分立项（Remux TV 12/UHD Remux TV 16/Encode TV 14/WEB-DL TV
	// 13）：剧集（tv_series）种分发 TV 项（电影形态主键归 Remux/Encode/
	// WEB-DL——TV 项 label-only 特化独占，直查永不命中防撞键）。
	// 分发依据=stdKey（媒介单源同源）；非 tv_series 零变化。
	if v := hduptTVMediumOption(cfg, siteName, meta, stdKey); v != nil {
		return v
	}
	// §59.325: MinBD 特例优先——有 MiniBD 精确选项的站点映 medium.minibd，
	// miss 时 fallback 原链（无该选项的站点走 Encode/原盘原语义）
	if hasMinBDToken(tp.RawTitle) {
		if m := e.lookupByStdKey(cfg, model.FieldDomainMedium, "medium.minibd"); m != nil {
			return m
		}
	}
	// §59.407: DIY 原盘媒介联动——原盘族（bluray/uhd_bluray）+ DIY 信号在场
	// 且站方有 DIY 媒介细分项时优先匹配（六站受益：库非/壹吧/咖啡/五二/
	// 冬樱/海的宝；触发条件动态检测非站名硬编码——未来站点有 DIY 项自动
	// 生效零新代码，§59.380 联动同款通用投资）。DIY 信号=tags 列 tag.diy
	// OR 标题 DIY 词形（双源）；DIY Remux 不触发（用户定案——六站均无
	// Remux DIY 项，信号+Remux 键照旧选 Remux）；无 DIY 项/无信号/
	// 非原盘媒介三重零触发面。
	if v := diyMediumOption(cfg, tp, stdKey); v != nil {
		return v
	}
	m := e.lookupByStdKey(cfg, model.FieldDomainMedium, stdKey)
	// §59.419: 压制媒介按分辨率细分匹配（直查 miss 后 fallback——
	// 站方按分辨率细分压制项：红豆饭 20/24/25/26 式四分）
	if m == nil {
		m = resolutionEncodeOption(cfg, tp, stdKey)
	}
	return m
}

// idNameWithMode type 域带 mode（幸运 data-mode='4'）。
type idNameWithMode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
}

// crossSeasonTagRe §59.177: 跨季标题形态（S01-S02 / 第N-M季）。
var crossSeasonTagRe = regexp.MustCompile(`(?i)(?:[.\s]S\d{1,2}-S?\d{1,2}\b|第\d+-\d+季)`)

// multiWorkCollectionRe §59.177: 多作品合集信号（副标题）。
var multiWorkCollectionRe = regexp.MustCompile(`(合集|集全|期全)`)

// completeSignalRe §59.177: 完结信号（副标题）。§59.424 扩英文词形
// （用户定案中英双语——\bcomplete\b 词边界防误匹配 incomplete）。
var completeSignalRe = regexp.MustCompile(`(\b(?i:COMPLETE)\b|全集|全季|已完结|完结|全\d+集|全剧终)`)

// mainTitleCompleteRe §59.424: 主标题完结信号（中英双语词形——
// Complete Seasons/Series/Collection 全形态经 \bcomplete\b 命中）。
var mainTitleCompleteRe = regexp.MustCompile(`(\b(?i:COMPLETE)\b|完结|全集|全季)`)

// seasonPackFormRe §59.424: 合集形态季集（纯季 S01 / 跨季 S01-S02 /
// 第N-M季——无 E 后缀）。归一通道（§59.308）命中的完整季已是纯季号。
var seasonPackFormRe = regexp.MustCompile(`^S\d{1,2}(-S?\d{1,2})?$`)

// singleEpisodeRe §59.424: 单集形态（S01E02——无区间后缀）。§59.514
// 刀1: 裸 Exx 兼容（(?:S\d{1,2})? 可选——Quacks E01 案形态；无信号裸
// 单集=真分集该拦·此前裸形态提取失明走「无 S/E 信号」豁免漏拦）。
var singleEpisodeRe = regexp.MustCompile(`^(?:S\d{1,2})?E(\d{1,4})$`)

// haidanCrossSeasonRe §59.472: 跨季形态（S01-S02 / S01-02 尾季无 S 短形态·
// 单位数 S1-2）——海胆 collages 合集勾选判定。
var haidanCrossSeasonRe = regexp.MustCompile(`^S\d{1,2}-S?\d{1,2}$`)

// episodeReleaseBlocked §59.424: 系统级禁发剧集分集（用户定案——所有
// 站点仅允许合集/完结/全集形态发布；与"转存发布禁勾首发"同层行为铁律，
// 纯代码规则零配置）。
// 范围：category ∈ {tv_series, animation, tv_shows}（②定案；动漫电影
// 形态无 S/E 信号天然豁免；短剧/音乐/MV/体育不在范围——边界定案④）。
// 放行（任一）：
//   - 无分集形态（无 S/E 信号，或 S01 纯季/跨季合集形态——归一通道
//     §59.308 命中的完整季此时已是纯季号）
//   - 完结信号任一（①定案宁放勿拦）：meta.Tags 含 complete（源站采集
//     标签①）∨ 主标题中英词形② ∨ 副标题中英词形直查③（P2 定案：
//     直查不依赖 fetch 推断产物——历史种/推断 miss 兜底）
//   - 全集一集：单集 E01 ∧ PTGen episodes=1（BBC 单剧形态——定案不禁）
//
// 拦截：单 E / 归一后仍是区间（非 E01 起或无完结信号的 E01 起区间）且
// 无任何完结信号。
// 边界（漏判放行·定案）：S01 标题权威（内容半季不判读文件清单）/
// 多区间复杂形态按第一组锚/源站误标 movie 等范围外豁免。
func episodeReleaseBlocked(meta *model.TorrentMetadata, tp *titleparser.TechProfile) (bool, string) {
	if meta == nil || tp == nil {
		return false, ""
	}
	switch meta.Category {
	case "category.tv_series", "category.animation", "category.tv_shows":
	default:
		return false, "" // 范围外（电影/短剧/音乐/MV/体育等）——豁免
	}
	se := tp.SeasonEpisode
	if se == "" {
		return false, "" // 无 S/E 信号（含动漫电影/OVA 形态）——豁免
	}
	if seasonPackFormRe.MatchString(se) {
		return false, "" // S01 纯季 / S01-S02 跨季——合集形态放行
	}
	// 单集/区间形态：完结信号集（任一即放——宁放勿拦）
	if strings.Contains(meta.Tags, `"complete"`) { // ①源站采集标签（归一 tag.complete）
		return false, ""
	}
	if mainTitleCompleteRe.MatchString(meta.Title) { // ②主标题中英词形
		return false, ""
	}
	if completeSignalRe.MatchString(meta.Subtitle) { // ③副标题中英词形直查（P2）
		return false, ""
	}
	// 全集一集：单集 E01 ∧ PTGen episodes=1（整部剧就一集——完整全集）
	if m := singleEpisodeRe.FindStringSubmatch(se); m != nil && m[1] == "01" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			if v, perr := strconv.Atoi(strings.TrimSpace(src.Episodes)); perr == nil && v == 1 {
				return false, ""
			}
		}
	}
	return true, fmt.Sprintf("剧集分集形态（%s）——系统规则仅允许合集/完结/全集发布（放行信号：源站完结标签 / 主·副标题 Complete·完结词形 / 全集数对照）", se)
}

// crossSeasonCollectionSites §59.177: 覆写组——跨季=合集（Session Pack 语义）。
// 藏宝阁: 多季合集规范（S01-SXX 专门命名规范）; 优堡: 整季包/跨季包;
// 野马 §59.518 用户定案: 完结(12)=单季完结状态·完结全集(18)=多季打包。
// 新站实测冲突时加条目（两层架构——通用=幸运，差异按站扩展）。
// §59.518 键修正："ubits"（英文）→"优堡"（name 列中文真相）——原键自
// v0.0.895 起恒 miss，覆写组在优堡从未生效（Quacks/Ambassadors 站方审核
// 「完结剧集请添加合集标签」实锤）。
var crossSeasonCollectionSites = map[string]bool{
	"藏宝阁": true,
	"优堡":  true,
	"野马":  true,
}

// adjustTagsForSite §59.177: 发布时标签按目标站调整（两层架构的站点扩展层）。
// 通用规则=幸运语义：合集=多部不同作品的集合；S01-S02=单作品系列→完结。
// 覆写组（藏宝阁/优堡/野马）：S01-S02=多季合集→合集（去完结）。
// 用户意图保护：副标题含"合集/集全/期全"（真多作品合集）时调整层不动。
//
// §59.518 优堡特化：站方无「完结」标签（值集只有连载23/合集20）——合集=
// Session Pack（整季包/跨季包），完结剧一律转合集。complete 产出词形=剧终
// 语境（全集/全季/完结/全N集/全剧终/COMPLETE），电影副标不含→不误触发。
func adjustTagsForSite(tags []string, siteName, title, subtitle string) []string {
	has := func(t string) bool {
		for _, x := range tags {
			if x == t {
				return true
			}
		}
		return false
	}
	remove := func(t string) []string {
		out := make([]string, 0, len(tags))
		for _, x := range tags {
			if x != t {
				out = append(out, x)
			}
		}
		return out
	}

	// 用户意图保护——真多作品合集不调整（§59.518 前置：原在跨季闸门后，
	// 前置后非跨季场景语义等价——两分支都不动；优堡单季分支同受保护）
	if multiWorkCollectionRe.MatchString(subtitle) {
		return tags
	}

	// §59.518 优堡特化——完结剧→合集（站方审核实锤+值集无完结）。
	// 跨季+complete 在场也走此分支（去完结加合集·与覆写组结果收敛）；
	// 跨季+complete 缺席则落到下方覆写组补合集——双路径收敛。
	if siteName == "优堡" && has("complete") {
		adjusted := remove("complete")
		if !has("collection") {
			adjusted = append(adjusted, "collection")
		}
		return adjusted
	}

	if !crossSeasonTagRe.MatchString(title + " " + subtitle) {
		return tags
	}

	if crossSeasonCollectionSites[siteName] {
		// 覆写组：跨季=合集（去完结，加合集）
		adjusted := remove("complete")
		if !has("collection") {
			adjusted = append(adjusted, "collection")
		}
		return adjusted
	}
	// 通用（幸运语义）：跨季=单作品系列（去合集，加完结）
	adjusted := remove("collection")
	if !has("complete") && completeSignalRe.MatchString(subtitle) {
		adjusted = append(adjusted, "complete")
	}
	return adjusted
}

// mteamCategoryValue §59.451: 馒头（api.m-team.cc）type 槽三维融合路由
// （typeMappingOf 第十站——好大模式同族：category 树=类型×媒介×分辨率
// 融合，22 非成人叶中影视族 9 叶按媒介/分辨率细分）。API 值集 2026-11-02
// 实时拉取（List API 八端点——team 3 增 1 删外其余零漂移）：
//   - 电影族（parent=100）：401 SD/419 HD/420 DVDiSo/421 BluRay/439 Remux
//   - 影剧/综艺族（parent=105，含剧集）：402 HD/403 SD/435 DVDiSo/438 BluRay
//     ——影剧族无 Remux 叶（Remux 归 402 HD——站方树实态）
//   - 动画 405+453 BluRay（原盘特化——discCategoryRoute 单类形态）
//   - 其余 13 类标准键直配（通用链：404 纪录/406 演唱/407 运动/434 无损/
//     409 其他/422 软件/427 电子书/442 有声书/451 教育/423 PC 游戏）。
//
// 媒介判定源=mediumMappingOf 结果的 StandardKeys（§59.380 单源同判定链
// ——首版误用 mediumStdKeyOf 弱判定：SourceType 归一损失致 REMUX 标题判
// bluray→421 误路由（龙PT 归一损失同款），DryRun 形态三/四抓出即修）：
//
//	原盘族（bluray/uhd_bluray/hddvd 键）→电影 421/影剧 438；dvd/dvdr→
//	420/435；remux/uhd_remux→439/402（影剧无 Remux 叶归 HD）；
//	分发形态（encode/webdl/webrip/hdtv/其它/miss）→分辨率 sd→401/403，
//	≥720p→419/402（default 兜底）。
//
// 影视族 9 叶+453 全 label-only（路由独占——通用链永不命中影视叶）；成人
// 15 叶不录（发布层成人拦截在前 §59.222 不可达）。
func (e *PublishExecutor) mteamCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "馒头" || cfg == nil || meta == nil {
		return nil
	}
	// 媒介族判定：mediumMappingOf 判定结果（与 medium 槽同源——归一损失
	// 免疫：REMUX 规格优先路径正确判 remux）
	mediumKey := ""
	if m := e.mediumMappingOf(cfg, siteName, meta, tp); m != nil {
		for _, k := range m.StandardKeys {
			if strings.HasPrefix(k, "medium.") {
				mediumKey = k
				break
			}
		}
	}
	// 动画×原盘族→453 BluRay（非原盘动画回退通用链 405）
	switch meta.Category {
	case "category.animation":
		switch mediumKey {
		case "medium.bluray", "medium.uhd_bluray", "medium.hddvd":
			for i := range cfg.ValueMappings[model.FieldDomainType] {
				if cfg.ValueMappings[model.FieldDomainType][i].Value == "453" {
					return &cfg.ValueMappings[model.FieldDomainType][i]
				}
			}
		}
		return nil
	}
	// 影视族×媒介×分辨率
	isMovie := meta.Category == "category.movie"
	isTV := meta.Category == "category.tv_series" || meta.Category == "category.tv_shows"
	if !isMovie && !isTV {
		return nil
	}
	var target string
	switch mediumKey {
	case "medium.bluray", "medium.uhd_bluray", "medium.hddvd":
		target = "421" // BluRay 原盘
		if isTV {
			target = "438"
		}
	case "medium.dvd", "medium.dvdr":
		target = "420" // DVDiSo
		if isTV {
			target = "435"
		}
	case "medium.remux", "medium.uhd_remux":
		target = "439" // Remux（影剧族无 Remux 叶→402 HD）
		if isTV {
			target = "402"
		}
	default:
		// 分发形态（encode/webdl/webrip/hdtv/其它/miss）——分辨率二分
		target = "419" // HD
		if isTV {
			target = "402"
		}
		if extract.LookupStandardKey("resolution", tp.Resolution) == "resolution.sd" {
			target = "401" // SD
			if isTV {
				target = "403"
			}
		}
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == target {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hdtimeCategoryValue §59.426: 时光（hdtime.org）type 槽原盘形态路由
// （typeMappingOf 第七站——媒介维度决定 type，好大模式同族）。站方 424
// Blu-Ray原盘为独立分类（非质量选项）——站内实态仲裁（424 列表页 62 种
// 全原盘标题实证：Blu-ray AVC 原盘资源发 424 而非 401+medium Blu-ray）。
// 路由：影视类 ∧ 原盘族媒介（bluray/uhd_bluray/hddvd——mediumStdKeyOf
// §59.380 单源同款；Remux/Encode 非原盘不触发）→424；其余回退通用链
// （401 电影/402 剧集/... 标准键直配）。音乐 CD/Track 等非"原盘形态"
// 不触发（CD=音乐介质）。424 label-only（特化独占——通用链永不命中）。
// discCategoryRoute §59.428: 原盘独立分类公共路由（时光 424/分享站 414——
// 两站同构形态：站方"原盘/蓝光"为独立 type 分类，站内实态全原盘标题）。
// 判定：影视类 ∧ 原盘族（bluray/uhd_bluray/hddvd——mediumStdKeyOf §59.380
// 单源；Remux/Encode 非原盘不触发）→ target 分类；其余回退通用链。
func discCategoryRoute(cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile, target string) *model.FormValueMapping {
	if cfg == nil || meta == nil {
		return nil
	}
	switch meta.Category {
	case "category.movie", "category.tv_series", "category.animation",
		"category.documentary", "category.documentaries", "category.tv_shows",
		"category.sports", "category.mv", "category.music_video":
	default:
		return nil // 非影视类（音乐/游戏/软件/文档等）回退通用链
	}
	switch mediumStdKeyOf(tp, "") {
	case "medium.bluray", "medium.uhd_bluray", "medium.hddvd":
		// 原盘族（含 DIY 原盘——站方无 DIY 媒介细分；Remux 非原盘不触发）
	default:
		return nil
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == target {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// mteamLabelsBits §59.451: 馒头 labels 位掩码翻译（tags canonical 键 →
// int 位掩码——站方 API 文档 TorrentUploadForm.labels 定义：bit0=DIY/
// bit1=中配（国语配音）/bit2=中字）。键集白名单精确匹配（diy/
// chinese_audio/chinese_subtitle）——其余标签（HDR/DV 等）馒头无对应
// bit 不翻译（tags 自定义字符串字段本发布链不投递）。
func mteamLabelsBits(tags []string) int {
	bits := 0
	for _, t := range tags {
		switch t {
		case "diy":
			bits |= 1 << 0
		case "chinese_audio":
			bits |= 1 << 1
		case "chinese_subtitle":
			bits |= 1 << 2
		}
	}
	return bits
}

// rousiRegionSlug §59.458: 肉丝 region label→slug 全表（categories API
// 2026-11-03 实时权威 18 项）。
var rousiRegionSlug = map[string]string{
	"大陆": "mainland-china", "香港": "hong-kong", "台湾": "taiwan",
	"日本": "japan", "韩国": "south-korea", "美国": "usa", "英国": "uk",
	"法国": "france", "德国": "germany", "意大利": "italy", "西班牙": "spain",
	"俄罗斯": "russia", "新西兰": "new-zealand", "加拿大": "canada",
	"印度": "india", "泰国": "thailand", "澳大利亚": "australia",
	"其它": "other",
}

// rousiGenreSet §59.458: 站方 genre 词表（29 项中文——categories API
// 权威；与豆瓣题材词表同源，实测 60 样本 214 词全命中）。
var rousiGenreSet = map[string]bool{
	"剧情": true, "喜剧": true, "动作": true, "爱情": true, "科幻": true,
	"悬疑": true, "惊悚": true, "恐怖": true, "犯罪": true, "动画": true,
	"奇幻": true, "冒险": true, "灾难": true, "战争": true, "传记": true,
	"历史": true, "运动": true, "音乐": true, "歌舞": true, "家庭": true,
	"儿童": true, "纪录": true, "短片": true, "真人秀": true, "脱口秀": true,
	"西部": true, "武侠": true, "古装": true, "其它": true,
}

// hdcitySubtitleFallback 副标题空态拼装退化（"译名 | MainTitle | Year"
// ——68754"骨湖 | Bone Lake | 2024"形态；译名空退化"MainTitle | Year"）。
func hdcitySubtitleFallback(meta *model.TorrentMetadata, tp titleparser.TechProfile) string {
	parts := []string{}
	if cn := chineseTitleOf(meta.Title); cn != "" {
		parts = append(parts, cn)
	}
	if tp.MainTitle != "" {
		parts = append(parts, tp.MainTitle)
	}
	if tp.Year != "" {
		parts = append(parts, tp.Year)
	}
	return strings.Join(parts, " | ")
}

// hdcityCodecOption §59.454: codec 域音频类切换（好大 blurayCodecOption
// 同款站方特化）——城市 codec_sel 混视频+音频 17 项单槽（§2.7 站方
// 编码-媒介对应表权威锚：FLAC→CD/Track、TrueHD/Atmos→BD/Remux/Encode）。
// 影视类走通用视频编码链零变化；音频类（type=408 Audio 或 medium 键
// CD/Track 族）改用音频编码数据源查同一 codec 域值集。
// hdcityCodecOption §59.454 附: codec 域音频类切换实现（e 方法——
// audioMappingOf 域参数化复用：城市 codec_sel 混音视频 17 项，音频类
// （category.music/lossless_music——408 Audio）改用音频编码数据源查
// 同一 codec 域；影视类返回 nil 走通用视频链零变化。
// 首版为残废函数（恒 nil）——回归审核抓出，AGENTS.md/docs 虚报即正。
func (e *PublishExecutor) hdcityCodecOption(cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.music" && meta.Category != "category.lossless_music" {
		return nil
	}
	return e.audioMappingOf(cfg, model.FieldDomainCodec, tp.AudioCodec, tp.AudioTechnology)
}

// hdcityTag1ingValue §59.454: 城市 tag1ing 题材注入（财神
// csptGenreTagValue 增强版）——◎类别 行 100% 必有（29 库 315/315），
// 行内多题材（"剧情 / 喜剧 / 动作"）按序取首个命中映射表的题材。
// PTGen 题材词→城市 tag1ing label 映射（27 项——"奇幻"→"魔幻/Fantasy"
// 语义对齐；"恐怖"城市无对应项跳过取行内下一词）。
var hdcityGenreMap = map[string]string{
	"动作": "动作/Action", "科幻": "科幻/Sci-Fi", "爱情": "爱情/Romance",
	"剧情": "剧情/Drama", "喜剧": "喜剧/Comedy", "家庭": "家庭/Family",
	"伦理": "伦理/Ethics", "惊悚": "惊悚/Thriller", "文艺": "文艺/Artistic",
	"冒险": "冒险/Adventure", "奇幻": "魔幻/Fantasy", "灾难": "灾难/Disaster",
	"犯罪": "犯罪/Crime", "悬疑": "悬疑/Mystery", "纪录": "记录/Documentary",
	"战争": "战争/War", "历史": "历史/History", "传记": "传记/Biography",
	"歌舞": "歌舞/Music", "动画": "动画/Animation", "西部": "西部/Western",
	"古装": "古装/Costume", "武侠": "武侠/仙侠", "运动": "运动/Sports",
	"微电影": "微电影/MiniVideo", "少儿不宜": "少儿不宜/Adult",
}

func hdcityTag1ingValue(cfg *model.PublishFormConfig, meta *model.TorrentMetadata) string {
	if meta == nil || meta.Description == "" {
		return ""
	}
	m := reGenreLine.FindStringSubmatch(meta.Description)
	if len(m) < 2 {
		return ""
	}
	// 行内题材词序列（"剧情 / 喜剧 / 动作"→[剧情,喜剧,动作]）
	for _, word := range strings.FieldsFunc(m[1], func(r rune) bool {
		return r == '/' || r == '、' || r == ' ' || r == '　' || r == ',' || r == '，'
	}) {
		word = strings.TrimSpace(word)
		if label, ok := hdcityGenreMap[word]; ok {
			// label → 城市 value（tag1ing 值=label 本身——字符串 value 下拉）
			return label
		}
	}
	return ""
}

// hdcityProcessingOption §59.454: 城市 processing_sel=3D 类型特化
// （用户定案）——tp.Stereo3D 映射：HOU→1(3D H-OU)/HSBS→2(3D H-SBS)/
// SBS·OU 全幅→5(3D Alt)/空→不投递（站方默认不选）。
// 交织(3)/红蓝(4)无标题提取词形——预览人工（罕见封装）。
func hdcityProcessingOption(cfg *model.PublishFormConfig, tp titleparser.TechProfile) *model.FormValueMapping {
	if cfg == nil {
		return nil
	}
	var target string
	switch tp.Stereo3D {
	case "HOU":
		target = "1"
	case "HSBS":
		target = "2"
	case "SBS", "OU":
		target = "5"
	default:
		return nil
	}
	for i := range cfg.ValueMappings["processing"] {
		if cfg.ValueMappings["processing"][i].Value == target {
			return &cfg.ValueMappings["processing"][i]
		}
	}
	return nil
}

// hdcityTagDependentFill §59.454: 城市站标签依赖注入——字幕
// chnize checkbox（用户定案：消费 Tab1 中字标签）。
// 直读 meta.Tags JSON（不经 assembleTags allowed 过滤——城市站无
// tags 域配置全滤实测首版抓出）。
func hdcityTagDependentFill(form map[string]string, tagsJSON string) {
	if form == nil || tagsJSON == "" {
		return
	}
	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return
	}
	for _, t := range tags {
		if t == "chinese_subtitle" {
			form["chnize"] = "1"
			return
		}
	}
}

// hdtimeCategoryValue §59.426（实现收敛 discCategoryRoute 公共——§59.428
// 分享站同构复用抽公共，两站 target 值差异参数化）。
func hdtimeCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "时光" {
		return nil
	}
	return discCategoryRoute(cfg, meta, tp, "424")
}

// itzmxCategoryValue §59.428: 分享站（pt.itzmx.com）type 槽原盘独立分类
// 路由（typeMappingOf 第八站——414 蓝光独立分类，站内实态仲裁：414 列表页
// 123 种全原盘标题 Blu-ray AVC/UHD Blu-ray DoVi——时光 424 同构形态，
// discCategoryRoute 公共复用）。414 label-only（特化独占）。
func itzmxCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "分享站" {
		return nil
	}
	return discCategoryRoute(cfg, meta, tp, "414")
}

// discfanRegionWords §59.425: 碟粉电影产区细分词表（比 classifyRegionWords
// §59.391 更细粒度——港/台分立、日/韩分立、泰单列：站方电影按地区 7 分）。
var (
	discfanMainlandWr = []string{"中国大陆", "大陆", "内地"}
	discfanHKWr       = []string{"中国香港", "香港"}
	discfanTWWr       = []string{"中国台湾", "台湾"}
	discfanJapanWr    = []string{"日本"}
	discfanKoreaWr    = []string{"韩国"}
	discfanThailandWr = []string{"泰国"}
)

// discfanCategoryValue §59.425: 碟粉（discfan.net）type 槽电影产区细分路由
// （typeMappingOf 第六站——用户定案）。站方电影按地区 7 分（大陆 401/港 404/
// 台 405/泰 402/日 403/韩 406/世界 410）——产区数据源=PTGenSourceJSON→
// src.Region（首值优先——葡萄定案）；列举地区 miss→410 世界兜底（亚洲小国
// 合法归 410——欧美已被 discfanPreflight 禁欧美上游拦，到路由的 410 承接
// =亚洲其它+罕见产区）。电影 7 地区+410 全部 label-only（必须经产区路由，
// 通用链不猜产区）；非电影类型（剧集/音乐/纪录/综艺/体育/动漫）回退通用
// 链（标准键直配——产区维度由 preflight 禁欧美统一把关）。
func discfanCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	if siteName != "碟粉" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.movie" {
		return nil // 非电影回退通用链
	}
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	// 产区匹配（首值优先+全数组兜底——葡萄定案）
	pick := func(words []string) bool {
		if len(regions) > 0 && util.ContainsAny(regions[0], words...) {
			return true
		}
		for _, r := range regions {
			if util.ContainsAny(r, words...) {
				return true
			}
		}
		return false
	}
	var want string
	switch {
	case pick(discfanMainlandWr):
		want = "401"
	case pick(discfanHKWr):
		want = "404"
	case pick(discfanTWWr):
		want = "405"
	case pick(discfanJapanWr):
		want = "403"
	case pick(discfanKoreaWr):
		want = "406"
	case pick(discfanThailandWr):
		want = "402"
	default:
		want = "410" // 世界兜底（欧美上游已拦——此处承接亚洲小国等）
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// discfanReleaseBlocked §59.425: 碟粉站点级发布前置双检查（用户定案）。
// ①五年规则（站规硬规则——仅允许五年以前的资源）：
//
//	year ≤ 当前年-5 放行；year > 当前年-5 拦；无年份放行（宁放——
//	29 库实证占比 0+审核门无 year 字段按需生长）。
//	年份信号=tp.Year（标题+PTGen §59.254 校正终态——Tab1 同源）。
//
// ②禁欧美（用户策略性定案——欧美资源一颗不发连候选流程都不走；
//
//	站规原文"欧美需候选"升级为我方直接禁）：
//	classifyRegionWords 复用——Western 拦/亚洲四类（大陆/港台/日韩/
//	亚洲其它）放/无产区防御性放行（29 库 315/315 Region 全覆盖实证
//	——实际不触发）。
//
// 挂载：Execute PushOnly 分支后（恒定检查同 §59.424 P1——补推豁免）。
func discfanReleaseBlocked(meta *model.TorrentMetadata, tp *titleparser.TechProfile) (bool, string) {
	if meta == nil || tp == nil {
		return false, ""
	}
	// ①五年规则
	if y, err := strconv.Atoi(strings.TrimSpace(tp.Year)); err == nil && y > time.Now().Year()-5 {
		return true, fmt.Sprintf("碟粉五年规则：%d 年资源（仅允许 %d 年及以前）——五年内资源禁止发布", y, time.Now().Year()-5)
	}
	// ②禁欧美
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	if len(regions) > 0 {
		cls := classifyRegionWords(regions[:1])
		if cls == regionNone {
			cls = classifyRegionWords(regions)
		}
		// classifyRegionWords 只分亚洲四类（§59.391——"其余=欧美"系消费端
		// 默认）：有产区证据但不中任何亚洲词表（regionNone）= 欧美/海外 → 拦。
		// regions 空 = 无产区证据 → 防御性放行（29 库 315/315 全覆盖实证不触发）。
		if cls == regionNone {
			return true, "碟粉站点规则：欧美产区资源禁发（用户策略——亚洲资源直发/欧美不走候选）"
		}
	}
	return false, ""
}

// hdskyCategoryValue §59.423: 天空（hdsky.me）type 槽分集/合集×国产/海外
// 双维路由（typeMappingOf 第五站——用户定案：恒合集+产区路由）。站规
// 明文"剧集分集仅官方发布员可发"+我方发布恒整季合集（v1.05）——
// 402/412/414 分集项 label-only 防撞；本特化只服务合集侧产区路由：
//
//	剧集 tv_series：陆港澳台→411 国产合集 ｜ 其余/无产区→413 海外合集
//	综艺 tv_shows：陆港澳台→403 国产综艺 ｜ 其余/无产区→415 海外合集
//
// 产区数据源=meta.PTGenSourceJSON→src.Region（Tab1"作品信息-产地"同源
// ——葡萄先例复用；首值优先+全数组兜底）。无产区证据默认海外（海外
// 剧集大盘——预览可手改）。403 配 [tv_shows] 键：综艺特化 miss 时
// 通用链兜底命中国产综艺（错分好过 miss——预览可手改）；411/413
// label-only（必须经产区路由，通用链不猜国籍）。
// 电影/纪录/动漫/短剧等其余类型回退通用链。
func hdskyCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	if siteName != "天空" || cfg == nil || meta == nil {
		return nil
	}
	var want string
	switch meta.Category {
	case "category.tv_series":
		want = "413"
		if hdskyIsDomesticRegion(meta) {
			want = "411"
		}
	case "category.tv_shows":
		want = "415"
		if hdskyIsDomesticRegion(meta) {
			want = "403"
		}
	default:
		return nil
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hdskyIsDomesticRegion 陆港澳台=国产判定（天空双维路由消费——
// classifyRegionWords §59.391 复用：regionMainland+regionHKTW）。
func hdskyIsDomesticRegion(meta *model.TorrentMetadata) bool {
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	if len(regions) == 0 {
		return false
	}
	cls := classifyRegionWords(regions[:1]) // 首值优先（葡萄定案）
	if cls == regionNone {
		cls = classifyRegionWords(regions)
	}
	return cls == regionMainland || cls == regionHKTW
}

// hdhomeCategoryValue §59.420: 家园（hdhome.org）type 槽三维融合特化
// （typeMappingOf 第四站——好大模式第三例放大版）。站方 type 种子区
// 63 项=类型×媒介×分辨率三维编码（label 结构 "{Prefix} {Medium/Res}"
// 统一），form_config 单键单值不可表达——**label 词形路由**（非硬编码
// value：站方改值不破+错拼容错 "TVSereis"(453)+"TVShows"(452) 复数+
// "Sport 8K"(511) 单数）。
// prefix（category 键→归一大写前缀）：
//
//	movie→MOVIES ｜ tv_series→TVSER ｜ tv_shows→TVSHOW ｜ documentary
//	(单复数)→DOC ｜ animation→ANIME ｜ music/lossless_music→MUSICS
//	（音频格式词 FLAC/APE 子路由）｜ sports→SPORT ｜ mv/music_video→
//	MUSICS+MV ｜ 其余（education/ebook/software/game/concert 等）→nil
//	（杂项站方概不接受/无独立项——通用链兜底）。
//
// suffix 路由矩阵（medium 单源+resolution 归一，label 归一去连字符
// 大写后 Contains 匹配）：
//
//	uhd_bluray: 8K(r4320p)→"8K"+"UHD BD" ｜ 其余→"UHD"+"BLURAY"
//	bluray→"BLURAY"（排除 UHD/8K）
//	uhd_remux: 8K→"8K"+"REMUX" ｜ 其余→"UHD"+"REMUX"
//	remux→"REMUX"（排除 UHD/8K）
//	encode/webdl/webrip/hdtv/minibd/hddvd/dvd/dvdr/other+分辨率：
//	  r4320p→"4320P" ｜ r2160p→"2160P" ｜ r1080p→"1080P" ｜ r1080i→
//	  "1080I" ｜ r720p→"720P" ｜ sd→"SD" ｜ 空分辨率→nil
//	music 类 cd/track 媒介→音频词形 FLAC/APE（tp.AudioCodec）
//
// 匹配：prefix 子集内 suffix 词形首中；REMUX 项由 remux 族分支独占
// （原盘分支排除 REMUX 词防 529 "Doc 8K UHD BD REMUX" 撞 8K 原盘）。
// nil=非家园/无 prefix/无分辨率（encode 族）——通用链零变化。
func hdhomeCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "家园" || cfg == nil || meta == nil {
		return nil
	}
	cat := meta.Category
	prefix, isMusic := "", false
	switch cat {
	case "category.movie":
		prefix = "MOVIES"
	case "category.tv_series":
		prefix = "TVSER" // 容错站方错拼 TVSereis(453)
	case "category.tv_shows":
		prefix = "TVSHOW" // 匹配单数 TVShow+复数 TVShows(452)
	case "category.documentary", "category.documentaries":
		prefix = "DOC"
	case "category.animation":
		prefix = "ANIME"
	case "category.music", "category.lossless_music":
		prefix, isMusic = "MUSICS", true
	case "category.sports":
		prefix = "SPORT" // 匹配单复数
	case "category.mv", "category.music_video":
		prefix = "MUSICS" // +MV 词形
	default:
		return nil
	}
	mediumKey := mediumStdKeyOf(tp, "")
	resKey := extract.LookupStandardKey("resolution", tp.Resolution)
	norm := func(s string) string {
		return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, "-", ""), " ", ""))
	}
	// 路由目标词形（norm 后 Contains 匹配）
	type wordMatch struct {
		words   []string
		exclude []string
	}
	var m wordMatch
	isMV := cat == "category.mv" || cat == "category.music_video"
	switch {
	case isMV:
		m = wordMatch{words: []string{"MV"}, exclude: nil}
	case isMusic && (mediumKey == "medium.cd" || mediumKey == "medium.track"):
		// 音乐音频格式子路由（FLAC/APE）
		ac := strings.ToUpper(tp.AudioCodec)
		switch {
		case strings.Contains(ac, "FLAC"):
			m = wordMatch{words: []string{"FLAC"}}
		case strings.Contains(ac, "APE"):
			m = wordMatch{words: []string{"APE"}}
		default:
			return nil
		}
	case mediumKey == "medium.uhd_bluray":
		if resKey == "resolution.r4320p" {
			m = wordMatch{words: []string{"8K", "UHDBD"}, exclude: []string{"REMUX"}}
		} else {
			m = wordMatch{words: []string{"UHDBLURAY"}, exclude: []string{"REMUX"}}
		}
	case mediumKey == "medium.bluray":
		m = wordMatch{words: []string{"BLURAY"}, exclude: []string{"UHD", "8K", "REMUX"}}
	case mediumKey == "medium.uhd_remux", mediumKey == "medium.remux":
		// 归一损失 UHD 维度（"UHD REMUX"→remux）——按分辨率分发：
		// 8K(4320p)→8K REMUX（529 Doc）；2160p→UHD REMUX（518/531）；
		// 其它→REMUX（415 等 BD Remux）
		switch resKey {
		case "resolution.r4320p":
			m = wordMatch{words: []string{"8K", "REMUX"}}
		case "resolution.r2160p":
			m = wordMatch{words: []string{"UHD", "REMUX"}, exclude: []string{"8K"}}
		default:
			m = wordMatch{words: []string{"REMUX"}, exclude: []string{"UHD", "8K"}}
		}
	default:
		// encode/webdl/webrip/hdtv/minibd/hddvd/dvd/dvdr/cd/track/other+分辨率
		switch resKey {
		case "resolution.r4320p":
			m = wordMatch{words: []string{"4320P"}}
		case "resolution.r2160p":
			m = wordMatch{words: []string{"2160P"}}
		case "resolution.r1080p":
			m = wordMatch{words: []string{"1080P"}}
		case "resolution.r1080i":
			m = wordMatch{words: []string{"1080I"}}
		case "resolution.r720p":
			m = wordMatch{words: []string{"720P"}}
		case "resolution.sd", "resolution.r480p", "resolution.r576p":
			m = wordMatch{words: []string{"SD"}}
		default:
			return nil // encode 族无分辨率不路由（预览人工）
		}
	}
	items := cfg.ValueMappings[model.FieldDomainType]
	for i := range items {
		label := norm(items[i].Label)
		if !strings.HasPrefix(label, prefix) {
			continue
		}
		ok, excluded := true, false
		for _, w := range m.words {
			if !strings.Contains(label, w) {
				ok = false
				break
			}
		}
		for _, x := range m.exclude {
			if strings.Contains(label, x) {
				excluded = true
				break
			}
		}
		if ok && !excluded {
			return &items[i]
		}
	}
	return nil
}

// cbgCategoryValue §59.411: 藏宝阁（cangbao.ge）type 槽动画/动漫分立
// 特化（typeMappingOf 第三站）。站方 404 动画（单集语义——电影/单集
// OVA）与 405 动漫（多集语义）分立且**双向规则校验**（动画分类检出
// 多集信号→站方错误/动漫分类无多集信号→站方错误）——form_config 单键
// 单值不可表达，404 配 animation 键仅覆盖单集形态，多集形态由本特化
// 分发：SeasonEpisode 非空（S01/E 多集信号）→ 405 动漫；空 → 404
// 动画（通用链兜底）。非藏宝阁/非动画类回退通用链零变化。
func cbgCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "藏宝阁" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.animation" || tp.SeasonEpisode == "" {
		return nil // 单集动画走通用链（404 配 animation 键兜底）
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == "405" {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hdareaCategoryValue §59.405: 好大（hdarea.club）type 槽媒介细分特化
// （用户五定案 2026-11-01）。站方 type 18 项=电影按媒介/分辨率细分
// 10 项（type×medium×resolution 三维融合——form_config 单键单值不可
// 表达，葡萄产区模式第二站）。**仅 category.movie 走特化**——非电影
// 类（动画 3D 归 405/MV/剧集等）回退通用链直配（416 命名 *Movies* 3D
// 只服务电影）。
// 路由矩阵（判定优先级自上而下，用户定案）：
//
//	① Stereo3D → 416（定案1：3D 最优先）
//	② uhd_bluray → 300 ｜ ③ bluray → 401 ｜ ④ remux/uhd_remux → 415
//	⑤ dvdr/dvd → 414 ｜ ⑥ webdl/webrip → 412（定案4：不看分辨率——
//	   4K WEB 也进，分辨率由 standard_sel 表达）｜ ⑦ hdtv → 413
//	⑧ 兜底（encode/minibd/hddvd/ipad 等）按分辨率（定案2/3）：
//	   2160p → 300 ｜ 1080p/1080i → 410 ｜ 720p → 411 ｜ SD/其它 → 414
//	   （414=DVD 分辨率宽语义=原盘/镜像/DVDRip/SD 重编码——用户定义）
//
// 数据源：mediumStdKeyOf 媒介单源+tp.Resolution+tp.Stereo3D（§59.380
// 联动同源）。
func hdareaCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "好大" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.movie" {
		return nil // 非电影类回退通用链（B 组直配——用户定案5）
	}
	var target string
	mediumKey := mediumStdKeyOf(tp, "")
	switch mediumKey {
	case "medium.uhd_bluray":
		target = "300"
	case "medium.bluray":
		target = "401"
	case "medium.remux", "medium.uhd_remux":
		target = "415"
	case "medium.dvdr", "medium.dvd":
		target = "414"
	case "medium.webdl", "medium.webrip":
		target = "412"
	case "medium.hdtv":
		target = "413"
	default:
		// 兜底按分辨率（encode/minibd/hddvd/ipad/其它——iPad 与
		// MiniBD 无专属分类，SD 并入 414 DVD 分辨率族——用户定案2/3）
		switch extract.LookupStandardKey("resolution", tp.Resolution) {
		case "resolution.r2160p", "resolution.r4320p":
			target = "300"
		case "resolution.r1080p", "resolution.r1080i":
			target = "410"
		case "resolution.r720p":
			target = "411"
		default:
			target = "414"
		}
	}
	if tp.Stereo3D != "" {
		target = "416" // 定案1：3D 最优先（Movies 3D——电影类专属）
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == target {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// sjtuRegionCategoryValue §59.391: 葡萄产区×类型映射。产区数据源=
// meta.PTGenSourceJSON→src.Region（Tab1"作品信息-产地"同源——
// §59.151 regionLabelsOfMeta 幸运判据消费先例）。**首值优先**
// （Region[0]=豆瓣展示的主产地序——复合产区二义消解，用户定案）；
// 首值 miss 再扫全数组；无产区证据 fallback 欧美（402/410/413——
// 资源主形态，预览可手改）。
func sjtuRegionCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	if siteName != "葡萄" || cfg == nil || meta == nil {
		return nil
	}
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	cls := regionNone
	if len(regions) > 0 {
		if c := classifyRegionWords(regions[:1]); c != regionNone { // 首值优先
			cls = c
		} else {
			cls = classifyRegionWords(regions) // 全数组兜底
		}
	}
	if cls == regionNone {
		cls = regionWestern // 无产区证据→欧美兜底
	}
	var want string
	switch meta.Category {
	case "category.movie":
		// 华语(401)/亚洲(403)/欧美(402)——日韩并入亚洲（电影无日韩项）
		switch cls {
		case regionMainland, regionHKTW:
			want = "401"
		case regionJapanKorea, regionAsianOther:
			want = "403"
		default:
			want = "402"
		}
	case "category.tv_series":
		// 港台(407)/亚洲(408)/大陆(409)/欧美(410)四分
		switch cls {
		case regionMainland:
			want = "409"
		case regionHKTW:
			want = "407"
		case regionJapanKorea, regionAsianOther:
			want = "408"
		default:
			want = "410"
		}
	case "category.tv_shows":
		// 大陆(411)/港台(412)/欧美(413)/日韩(414)四分——其它亚洲归欧美
		// （站方无"亚洲综艺"项）
		switch cls {
		case regionMainland:
			want = "411"
		case regionHKTW:
			want = "412"
		case regionJapanKorea:
			want = "414"
		default:
			want = "413"
		}
	default:
		return nil // 非产区类（动画/纪录/MV/体育…）→通用链
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hduptTVMediumOption §59.427: 好多油 TV 双轨分发（剧集媒介 TV 后缀项）。
func hduptTVMediumOption(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, stdKey string) *model.FormValueMapping {
	if siteName != "好多油" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.tv_series" {
		return nil // 电影/综艺等走通用键（电影形态主键）
	}
	var want string
	switch stdKey {
	case "medium.remux":
		want = "12" // Remux TV
	case "medium.uhd_remux":
		want = "16" // UHD Remux TV
	case "medium.encode":
		want = "14" // Encode TV
	case "medium.webdl", "medium.webrip":
		want = "13" // WEB-DL/WEBRip TV
	default:
		return nil // 原盘/CD 等 TV 无分立项——通用链
	}
	for i := range cfg.ValueMappings[model.FieldDomainMedium] {
		if cfg.ValueMappings[model.FieldDomainMedium][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainMedium][i]
		}
	}
	return nil
}

// haidanSeasonEpisodeInject §59.472: 海胆季/集/合集三字段注入（From S04 案——
// 纯季包 §59.424 合集形态放行但海胆站方要求剧集类必填季集信息·集数 0=全季）。
// 数据源 tp.SeasonEpisode 归一终态（§59.424 同源——挂载点同在归一后）：
//
//	S04 纯季 → season=4 episode=0（全季）；S01-S02 跨季 → 0/0+勾合集；
//	S04E01 单集（eps=1 全集豁免形态）→ season=4 episode=1；
//	剧集类无季信号（国产剧）→ 0/0（站方语义：不区分季+全季）；
//	电影等范围外（§59.424 豁免类）→ 不填（站方仅剧集类要求）。
//
// 未配置 season 键（其它站）零动作。
func haidanSeasonEpisodeInject(form map[string]string, cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp *titleparser.TechProfile) {
	sf, hasSeason := cfg.FormFields[model.FieldDomainSeason]
	if !hasSeason || sf == "" {
		return
	}
	// 剧集类判定（站方提示四类=电视剧/综艺/动画/纪录片——§59.424 同范围）
	switch meta.Category {
	case "category.tv_series", "category.animation", "category.tv_shows", "category.documentary", "category.documentaries":
	default:
		return // 电影/音乐等——站方不要求
	}
	se := ""
	if tp != nil {
		se = tp.SeasonEpisode
	}
	season, episode := "0", "0" // 默认：不区分季+全季（国产剧全集形态）
	collages := false
	trimLead0 := func(x string) string {
		t := strings.TrimLeft(x, "0")
		if t == "" {
			return "0"
		}
		return t
	}
	if m := singleEpisodeRe.FindStringSubmatch(se); m != nil {
		// S04E01 单集形态（§59.424 全集豁免放行——整部一集）——剥前导零。
		// §59.514: 裸 Exx 兼容（(?:S\d{1,2})? 可选——S 前缀从 se 尾剥）
		sPart := strings.TrimSuffix(se, "E"+m[1])
		season, episode = trimLead0(strings.TrimPrefix(sPart, "S")), trimLead0(m[1])
	} else if haidanCrossSeasonRe.MatchString(se) {
		// S01-S02 / S01-02 跨季——多季=站方合集（§59.471 回归审核修正：
		// 首版手写 Contains 条件 S1-2 单位数漏判 + S01E02-03 集区间误判——
		// 正则对齐 §59.424 seasonPackFormRe 形态域）
		collages = true
	} else if m := regexp.MustCompile(`^S(\d{1,2})$`).FindStringSubmatch(se); m != nil {
		// S04 纯季——全季（episode=0）——剥前导零
		season = trimLead0(m[1])
	}
	form[sf] = season
	if ef, ok := cfg.FormFields[model.FieldDomainEpisode]; ok && ef != "" {
		form[ef] = episode
	}
	if collages {
		if cf, ok := cfg.FormFields[model.FieldDomainCollages]; ok && cf != "" {
			form[cf] = "1"
		}
	}
}

// hdareaPreChecker 刀2：站方预查重能力接口（好大 API 独有·
// publish 包内私有——executor 局部变量 adapter 遮蔽 adapter 包名）。
type hdareaPreChecker interface {
	CheckBeforeUpload(ctx context.Context, config *model.SiteConfig, torrentData []byte) (*adapter.PreCheckResult, error)
}
