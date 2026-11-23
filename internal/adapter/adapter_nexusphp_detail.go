package adapter

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// GetTorrentDetail §59.162 外层：keepfrds 禁转增强 merge（内层原逻辑零改动）。
// §59.340 A: enrich 复用主链 HTML——原二次 fetchDetailsHTML(同 tid 同 URL) 是
// 纯重复请求（keepfrds 流控流量的 1/3 浪费——§59.340 探究实验背景）。
func (a *NexusPHPAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	detail, html, err := a.getTorrentDetailInner(ctx, config, torrentID)
	if err != nil || detail == nil {
		return detail, err
	}
	// §59.266: 解析真空双守卫（公共化三防线——Engine/legacy/OpenCD 全路径收口）
	if err := CheckDetailVacuum(detail); err != nil {
		return nil, err
	}
	if strings.Contains(config.Domain, "keepfrds") {
		a.enrichKeepfrdsTransferFlags(ctx, config, torrentID, detail, html)
	}
	// §59.167 憨憨：标签明示禁转（无限时态——用户权威；详情页 Tailwind 徽章）
	if strings.Contains(config.Domain, "hhanclub") {
		a.enrichHhanTransferFlags(ctx, config, torrentID, detail, html)
	}
	// §59.510 瓷器：share_rule 结构化转载规则列（列表页独有·详情页无此标记）
	if strings.Contains(config.Domain, "hdwing") {
		a.enrichHdwingTransferFlags(ctx, config, torrentID, detail, html)
	}
	return detail, nil
}

// enrichHhanTransferFlags §59.167: 憨憨禁转标签采集——详情页"标签"字段区块直提。
// 判据演进（双正例对照实证 tid=173490 真禁转 vs tid=211544 非禁转）：
//
//	①纯文本（否）：顶部筛选器固定含"禁转"选项（checkbox+label 内包 a 链接，
//	  tag_id1=1 形态两区同现）→ 全量误判；②CSS 写法差异（弃）：样式巧合；
//	③class font-bold（次优）：表现层语义；④终案（字段区块定位）：详情页字段
//	  区块顺序固定"标题→副标题→标签→基本信息"——提取 >标签</div> 至
//	  >基本信息</div> 之间内容（该区块只含种子自身标签链接），区块内判
//	  "禁转"字符。字段区块名是页面数据结构语义（样式无关——用户结构化分析定案）。
//
// 首发/转载标签不消费——禁转才是拦发布语义。
// §59.340 A: mainHTML 变参复用（同 keepfrds enrich——省憨憨站二次抓取）
func (a *NexusPHPAdapter) enrichHhanTransferFlags(ctx context.Context, config *model.SiteConfig, torrentID string, detail *model.TorrentDetail, mainHTML ...string) {
	var html string
	if len(mainHTML) > 0 {
		html = mainHTML[0]
	}
	if html == "" {
		var err error
		html, err = a.fetchDetailsHTML(ctx, config, torrentID)
		if err != nil {
			return
		}
	}
	tagSec := reHhanTagSection.FindStringSubmatch(html)
	if tagSec != nil && strings.Contains(tagSec[1], "禁转") {
		for _, f := range detail.Flags {
			if f == "禁转" {
				return
			}
		}
		detail.Flags = append(detail.Flags, "禁转")
	}
}

// reHhanTagSection 详情页"标签"字段区块（标题→副标题→标签→基本信息——区块
// 内容只含种子自身标签，筛选器在页面顶部 top-search 区不会混入）。
var reHhanTagSection = regexp.MustCompile(`(?s)>标签</div>(.*?)>基本信息</div>`)

// enrichKeepfrdsTransferFlags §59.162: keepfrds 禁转两态采集增强——
// 详情页"发布于"时间原文（相对时间——天档以上=超 24h 窗）+ 标记区
// [禁转] 永久 / [限时禁转] 24h（用户站点经验权威）。
// 非侵入：塞 detail 附加字段，两态判定收口在 fetcher（extract_flags 消费）。
// §59.340 A: mainHTML 变参——复用主链 HTML（原二次抓同 URL 纯浪费）；空则
// 兜底自抓（独立调用方兼容）。
// §59.542 追记十三（用户定案·详情页优先）：h1 标题区 badge 为主判定——
// 详情页主链必达零额外请求，老种翻页后列表锚定 miss 不影响判定；h1 无
// badge 再走列表行兜底（形态演变第二通道——宁可多拦不可错放）。
func (a *NexusPHPAdapter) enrichKeepfrdsTransferFlags(ctx context.Context, config *model.SiteConfig, torrentID string, detail *model.TorrentDetail, mainHTML ...string) {
	html := ""
	if len(mainHTML) > 0 {
		html = mainHTML[0]
	}
	if html == "" {
		var err error
		html, err = a.fetchDetailsHTML(ctx, config, torrentID)
		if err != nil {
			return
		}
	}
	if m := reKeepfrdsAddedAt.FindStringSubmatch(html); len(m) > 1 {
		detail.AddedAtText = strings.TrimSpace(m[1])
	}
	// §59.542 追记十三: h1 badge 优先——命中即判定，不再抓列表页。
	if badge := keepfrdsH1Badge(html); badge != "" {
		detail.RawListRow = badge
		return
	}
	if row := a.fetchKeepfrdsListRow(ctx, config, torrentID); row != "" {
		detail.RawListRow = row
	}
}

// reKeepfrdsH1Zone h1 标题区间（badge 限定域——页面其它区域可能复用
// recommended class，仅标题区标记是权威信号）。锚定 NP 详情页标题标准
// id="top"（实拉单 h1；锚定后页头潜在 h1 干扰不影响——miss 则走列表兜底）。
var reKeepfrdsH1Zone = regexp.MustCompile(`(?s)<h1[^>]*id=["']top["'][^>]*>.*?</h1>`)

// reKeepfrdsH1Badge h1 标题区禁转 badge（§59.542 追记十三·朋友站 tid 2782403
// 实测）：`<b>[<font class='recommended'>禁转</font>]</b>`——与列表页行同
// badge 形态（限时禁转同 class·列表页实拉佐证）。折扣类用其它 class
// （halfdown 等）不命中；引号单双兼容；(?s) 防 h1 内嵌图标跨行。
var reKeepfrdsH1Badge = regexp.MustCompile(`<b>\s*\[\s*<font\s+class\s*=\s*['"]recommended['"]\s*>([^<]+)</font>\s*\]\s*</b>`)

// keepfrdsH1Badge 提取 h1 禁转 badge 段——仅 badge 文本进分类器（标题
// 文本不进：种子名含"禁转"字样不误判），词形两态判定收口 ClassifyNoTransfer。
func keepfrdsH1Badge(html string) string {
	zone := reKeepfrdsH1Zone.FindString(html)
	if zone == "" {
		return ""
	}
	m := reKeepfrdsH1Badge.FindStringSubmatch(zone)
	if len(m) > 1 {
		return "<b>[" + strings.TrimSpace(m[1]) + "]</b>"
	}
	return ""
}

var reKeepfrdsAddedAt = regexp.MustCompile(`由[^<]{0,30}发布于([^<]{2,25})`)

// reHdwingShareRule hdwing 列表行转载规则列：
// `<img src='/pic/share_rule_255.gif' title='其他情况(参考副标题及种子简介的标注语)'/>`
var reHdwingShareRule = regexp.MustCompile(`share_rule_(\d+)\.gif' title='([^']+)'`)

// reHdwingNoTransferWords 软规则（share_rule_255=其他情况）标注语词族——
// 扫描域限定副标题+简介两字段（§59.1406 教训：全页/声明文本扫描跨词伪命中；
// 站方语义"参考副标题及种子简介的标注语"=扫描域权威定义）。繁简变体齐备。
var reHdwingNoTransferWords = regexp.MustCompile(`禁转|禁止转载|谢绝转载|严禁转载|謝絕轉載|請勿轉載|禁止轉載`)

// enrichHdwingTransferFlags §59.510 hdwing（瓷器）share_rule 三态禁转采集。
// 站方独有结构化转载规则列（列表行 img title·100 行采样三态分布 2%/32%/66%）：
//
//	share_rule_1 = 禁止转载 → 硬禁转（Flags += "禁转"）
//	share_rule_3 = 欢迎转载 → 可转（无操作）
//	share_rule_255 = 其他情况(参考副标题及种子简介的标注语) → 软规则——
//	  副标题+简介扫词族（命中=禁转·未命中=可转）
//
// 判定收口：Flags += "禁转"（publish_torrents_handler 词集合消费契约同 hhan）。
// 列表页锚定行（keepfrds 同构·批次缓存 10min·tid 不在第一页=无标记不误伤）。
func (a *NexusPHPAdapter) enrichHdwingTransferFlags(ctx context.Context, config *model.SiteConfig, torrentID string, detail *model.TorrentDetail, mainHTML ...string) {
	row := a.fetchHdwingListRow(ctx, config, torrentID)
	if row == "" {
		return
	}
	m := reHdwingShareRule.FindStringSubmatch(row)
	if m == nil {
		return
	}
	ruleText := m[2]
	noTransfer := false
	switch {
	case strings.Contains(ruleText, "禁止转载"), strings.Contains(ruleText, "不得转载"), strings.Contains(ruleText, "未经允许"):
		// share_rule_1=禁止转载 / share_rule_2=未经允许不得转载（站内实测
		// 837787/822791——share_rule_2 首版漏判·禁转词族显式列举防未知编号误伤）
		noTransfer = true
	case strings.Contains(ruleText, "其他情况"):
		// share_rule_255 软规则：站方语义"参考副标题及种子简介的标注语"。
		// §59.511 回归修正：扫描域收窄 Subtitle-only——正例四形态实测全部
		// 在副标题（自购禁转/未允禁转/授权转载禁转/【禁转】），简介正文
		// 正例零实证；简介=声明/致谢混杂区，系统禁转判定设计不读简介
		// 声明（extractFlags 两道防线：§59.39 quote 剥离+「禁转PTT」定向
		// 豁免——首版裸扫 Subtitle+Description 偏离该设计）。站内出现
		// 简介级标注实证再扩（扩展时复用系统防线不裸扫）。
		if reHdwingNoTransferWords.MatchString(detail.Subtitle) {
			noTransfer = true
		}
	default:
		// share_rule_3=欢迎转载及其它未知编号——不误伤
	}
	if !noTransfer {
		return
	}
	for _, f := range detail.Flags {
		if f == "禁转" {
			return
		}
	}
	detail.Flags = append(detail.Flags, "禁转")
}

// fetchHdwingListRow 抓 hdwing 列表首页（裸 torrents.php=第一页 100 行）按
// tid 锚定行块——与 keepfrds 同构但 URL 不同（page=0 参数该站未实证·裸路径
// 摸底实测 200），缓存独立字段防单例互踩。
func (a *NexusPHPAdapter) fetchHdwingListRow(ctx context.Context, config *model.SiteConfig, torrentID string) string {
	base := config.Domain
	if !strings.HasPrefix(base, "http") {
		base = "https://" + base
	}
	u := strings.TrimRight(base, "/") + "/torrents.php"

	a.hdwCache.mu.Lock()
	cachedHTML, cachedAt := a.hdwCache.html, a.hdwCache.expires
	a.hdwCache.mu.Unlock()
	if cachedHTML != "" && time.Now().Before(cachedAt) {
		return anchorListRow(cachedHTML, torrentID)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return ""
	}
	setCommonHeaders(req, config.Cookie)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { drainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := readBody(resp)
	if err != nil {
		return ""
	}
	html := string(body)
	a.hdwCache.mu.Lock()
	a.hdwCache.html = html
	a.hdwCache.expires = time.Now().Add(keepfrdsListCacheTTL)
	a.hdwCache.mu.Unlock()
	return anchorListRow(html, torrentID)
}

// fetchKeepfrdsListRow 抓列表页首页按 tid 定位的行原始文本（新种限时窗内必在
// 首页——NP 默认时间倒序；老种[标记已消失]拿不到行=无标记语义，不误伤）。
// §59.340 B: 列表页批次缓存——page=0 的 URL 对每颗种子完全相同（代码实证
// 一字不差），批量 71 颗=71 次重复下载同一页。10min TTL 批次级缓存，
// 每颗本地锚定自己的 tid 行（锚定失败=种子不在第一页，与现状行为等价）。
const keepfrdsListCacheTTL = 10 * time.Minute

type keepfrdsListCache struct {
	mu      sync.Mutex
	html    string
	expires time.Time
}

func (a *NexusPHPAdapter) fetchKeepfrdsListRow(ctx context.Context, config *model.SiteConfig, torrentID string) string {
	base := config.Domain
	if !strings.HasPrefix(base, "http") {
		base = "https://" + base
	}
	u := strings.TrimRight(base, "/") + "/torrents.php?page=0"

	// 批次缓存命中 → 零网络本地锚定
	a.listCache.mu.Lock()
	cachedHTML, cachedAt := a.listCache.html, a.listCache.expires
	a.listCache.mu.Unlock()
	if cachedHTML != "" && time.Now().Before(cachedAt) {
		return anchorListRow(cachedHTML, torrentID)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return ""
	}
	setCommonHeaders(req, config.Cookie)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { drainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := readBody(resp)
	if err != nil {
		return ""
	}
	html := string(body)
	// 写入批次缓存（失败请求不缓存）
	a.listCache.mu.Lock()
	a.listCache.html = html
	a.listCache.expires = time.Now().Add(keepfrdsListCacheTTL)
	a.listCache.mu.Unlock()
	return anchorListRow(html, torrentID)
}

// anchorListRow 从列表页 HTML 锚定 tid 行块（<tr> 边界粗切）
func anchorListRow(html, torrentID string) string {
	anchor := "details.php?id=" + torrentID
	i := strings.Index(html, anchor)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(html[:i], "<tr")
	end := strings.Index(html[i:], "</tr>")
	if start < 0 || end < 0 {
		return ""
	}
	return html[start : i+end+5]
}

// §59.340 A: 返回 (detail, html, err)——html 供 enrich 复用（省同 URL 二次抓）
func (a *NexusPHPAdapter) getTorrentDetailInner(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, string, error) {
	html, err := a.fetchDetailsHTML(ctx, config, torrentID)
	if err != nil {
		return nil, "", err
	}
	// §59.162: keepfrds 发布时间提取收口在 enrichKeepfrdsTransferFlags（单点）

	// §56.34 OpenCD 专用提取（音乐站 DOM 与标准 NexusPHP 差异大）
	if isOpenCDDomain(config.Domain) {
		return extractOpenCDDetail(html), html, nil
	}

	// §56.13 方案 B: 优先用 Engine 提取（更精准：goquery + 站点 hook + 标准化键映射）
	if a.engine != nil {
		detail, ok := a.extractWithEngine(html, config, torrentID)
		if ok {
			a.fillMediaFromRawBlock(html, detail)
			a.fillPublishedAt(html, detail)
			// §59.469 附五: 乌托邦音乐字段后处理（enrich 模式·内部域判定）
			a.enrichUtopiaMusic(config, html, detail)
			return detail, html, nil
		}
		if a.logger != nil {
			a.logger.Warn("nexusphp: engine extraction not meaningful, fallback to legacy regexp")
		}
	}

	// fallback: 老 regexp 提取（保留作 Engine 失败时的兜底）
	detail := a.legacyExtractDetail(html)
	a.fillMediaFromRawBlock(html, detail)
	a.fillPublishedAt(html, detail)
	// §59.469 附五: 乌托邦音乐字段后处理（enrich 模式·内部域判定）
	a.enrichUtopiaMusic(config, html, detail)
	return detail, html, nil
}

// rePublishedAtTitle §59.369 附二: NP 新版 details-inline-time 的发布时间
// 绝对戳——"发布于<span title='2026-10-29 19:35:30'>4时6分钟前</span>"
// （相对文本是展示态，title 才是精确源——深度站实证；比 keepfrds 相对
// 时间解析（ParseRelativeAge）更优，形态不变不受站方展示措辞影响）。
var rePublishedAtTitle = regexp.MustCompile(`发布于<span[^>]*title="(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})"`)

// fillPublishedAt §59.369 附二: 详情页发布时间提取（已有值不覆盖）。
func (a *NexusPHPAdapter) fillPublishedAt(html string, detail *model.TorrentDetail) {
	if detail == nil || detail.PublishedAt != nil {
		return
	}
	m := rePublishedAtTitle.FindStringSubmatch(html)
	if len(m) < 2 {
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", m[1], time.Local)
	if err != nil {
		return
	}
	detail.PublishedAt = &t
}

// fillMediaFromRawBlock §59.366: 结构化字段区 MI/BDInfo 补取——
// 深度站（dstudio.me）等 NP 站把 BDInfo 放在字段区（rowhead
// "MediaInfo/BDInfo" 的 rowfollow 内 nexus-media-info-raw div 的
// <pre>，锚 DISC INFO:），不在 kdescr 简介区——Description 通道提不到。
// 条件：双空才补（已有值不覆盖——简介区优先）；IsLikelyBDInfoText
// 判型填 BDInfo（原盘站主形态），否则按 MI 判据填 MediaInfo。
func (a *NexusPHPAdapter) fillMediaFromRawBlock(html string, detail *model.TorrentDetail) {
	if detail.MediaInfo != "" || detail.BDInfo != "" {
		return
	}
	loc := strings.Index(html, `class="nexus-media-info-raw"`)
	if loc < 0 {
		return
	}
	rest := html[loc:]
	preStart := strings.Index(rest, "<pre")
	if preStart < 0 {
		return
	}
	preEnd := strings.Index(rest[preStart:], "</pre>")
	if preEnd < 0 {
		return
	}
	raw := rest[preStart : preStart+preEnd]
	// 剥 HTML 标签取纯文本（<pre> 内可能嵌 font/b 等）
	text := stripTagsForPre(raw)
	text = htmlEntityDecode(text)
	if len(text) < 100 {
		return
	}
	if isLikelyBDInfoReport(text) {
		detail.BDInfo = text
	} else if isLikelyMIRawText(text) {
		detail.MediaInfo = text
	}
}

// isLikelyBDInfoReport §59.366: BDInfo 报告形态锚（DISC INFO:/PLAYLIST
// REPORT:——nexus-media-info-raw 容器的原始 BDInfo 报告头）。
func isLikelyBDInfoReport(text string) bool {
	head := text
	if len(head) > 300 {
		head = head[:300]
	}
	return strings.Contains(head, "DISC INFO:") ||
		strings.Contains(head, "PLAYLIST REPORT:") ||
		strings.Contains(head, "VIDEO:")
}

func isLikelyMIRawText(text string) bool {
	return strings.Contains(text, "General") && strings.Contains(text, "Video")
}

// stripTagsForPre 剥 <pre> 块内的 HTML 标签（保留纯文本）。
func stripTagsForPre(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func htmlEntityDecode(s string) string {
	r := strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'")
	return r.Replace(s)
}

// fetchDetailsHTML 负责详情页 HTTP GET，返回原始 HTML。
// §59.336: site.browser_fallback=true 且 HTTP 失败时，用内置
// Chromium 兜底（keepfrds 案：CF 对详情页检测严于首页/RSS，
// azuretls Chrome 不足，真实浏览器 + Cookie 实测一次通过）。
func (a *NexusPHPAdapter) fetchDetailsHTML(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	u := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	html, httpErr := a.fetchDetailsHTMLViaHTTP(ctx, config, u)
	if httpErr == nil {
		return html, nil
	}

	// §59.336: 浏览器 fallback（开关开+获取器可用才走）
	if config.BrowserFallback && a.browser != nil {
		if a.logger != nil {
			a.logger.Info("nexusphp: detail HTTP failed, browser fallback",
				zap.String("domain", config.Domain),
				zap.String("torrent_id", torrentID),
				zap.Error(httpErr))
		}
		brHTML, brErr := a.browser.FetchHTML(ctx, u, config.Cookie, "", 45*time.Second)
		if brErr == nil && len(brHTML) > 1000 {
			return brHTML, nil
		}
		if a.logger != nil {
			a.logger.Warn("nexusphp: browser fallback failed",
				zap.String("domain", config.Domain),
				zap.String("torrent_id", torrentID),
				zap.Int("html_len", len(brHTML)),
				zap.Error(brErr))
		}
	}
	return "", httpErr
}

// fetchDetailsHTMLViaHTTP 原详情页 HTTP GET 逻辑（§59.336 从
// fetchDetailsHTML 拆出，零改动）。
func (a *NexusPHPAdapter) fetchDetailsHTMLViaHTTP(ctx context.Context, config *model.SiteConfig, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", networkError("请求详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return "", httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	// §59.266: 凭证拦截重定向检测（公共化三防线——不可杜 take2fa/异地登录案
	// 修复落错适配器教训：守卫必须在真实服务的适配器层生效）
	if err := CheckCredentialRedirect(resp); err != nil {
		return "", err
	}

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// extractWithEngine 用 Engine 提取详情页字段。
// 返回 (detail, true) 表示提取有意义；返回 (nil, false) 表示 Engine 失败需 fallback。
func (a *NexusPHPAdapter) extractWithEngine(html string, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, bool) {
	input := extract.Input{
		SiteCode:     deriveSiteCode(config.Domain),
		Domain:       config.Domain,
		SiteNickname: config.SiteName,
		BaseURL:      config.BaseURL,
		TorrentID:    torrentID,
		PageHTML:     html,
	}
	seed, meta := a.engine.Extract(input)
	if !seed.IsMeaningful() {
		return nil, false
	}
	detail := extract.SeedToDetail(seed)
	if detail == nil {
		return nil, false
	}
	detail.RawHTML = html
	if meta.ExtractorName != "" {
		detail.EngineExtractorName = meta.ExtractorName
	}
	return detail, true
}

// legacyExtractDetail 老 regexp 提取逻辑（原 GetTorrentDetail 主体）。
// 保留作 Engine 失败时的 fallback；Engine 成功时不会调用。
func (a *NexusPHPAdapter) legacyExtractDetail(html string) *model.TorrentDetail {
	detail := &model.TorrentDetail{}

	if m := reNexusTitle.FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}

	for _, row := range reNexusRow.FindAllString(html, -1) {
		rowText := cleanRowText(row)

		if (strings.Contains(rowText, "类别") || strings.Contains(rowText, "Category")) && !strings.Contains(rowText, "基本信息") {
			extractCategoryFromImg(row, detail)
		}

		if strings.Contains(rowText, "基本信息") || strings.Contains(rowText, "基本") {
			extractBasicInfoFields(row, detail)
		}

		if strings.Contains(rowText, "副标题") && !strings.Contains(rowText, "基本信息") {
			extractSubtitleFromRow(row, detail)
		}

		if (strings.Contains(rowText, "种子文件") || strings.Contains(rowText, "种子信息")) && detail.InfoHash == "" {
			if h := extractInfoHashFromRow(row); h != "" {
				detail.InfoHash = h
			}
		}
	}

	if detail.Size == 0 || detail.Category == "" {
		ddBlockRe := reNexusDDBlock
		if m := ddBlockRe.FindStringSubmatch(html); len(m) > 1 {
			extractBasicInfoFields(m[1], detail)
		}
		fallbackDivRe := reNexusFallbackDiv
		if m := fallbackDivRe.FindStringSubmatch(html); len(m) > 1 {
			extractBasicInfoFields(m[1], detail)
		}
	}

	if detail.Subtitle == "" {
		ddSubRe := reNexusSubtitleDD
		if m := ddSubRe.FindStringSubmatch(html); len(m) > 1 {
			text := stripTags(m[1])
			text = reNexusWhitespace.ReplaceAllString(text, " ")
			text = strings.TrimSpace(text)
			if text != "" {
				detail.Subtitle = text
			}
		}
	}

	if detail.InfoHash == "" {
		for _, re := range hashRowPatterns {
			if m := re.FindStringSubmatch(html); len(m) > 1 {
				detail.InfoHash = strings.ToLower(m[1])
				break
			}
		}
	}

	if detail.InfoHash == "" {
		if m := reNexusHashDD.FindStringSubmatch(html); len(m) > 1 {
			detail.InfoHash = strings.ToLower(m[1])
		}
	}

	if detail.InfoHash == "" {
		if m := reNexusInfoHash.FindStringSubmatch(html); len(m) > 1 {
			detail.InfoHash = strings.ToLower(m[1])
		}
	}

	if m := reNexusKdescr.FindStringSubmatch(html); len(m) > 1 {
		detail.Description = strings.TrimSpace(m[1])
	}

	detail.Tags = extractTags(html)
	detail.Category = NormalizeCategory(detail.Category)

	if detail.Description != "" {
		detail.Screenshots = extractScreenshotsFromBBCode(detail.Description)
		detail.MediaInfo = extractMediaInfoFromBBCode(detail.Description)
	}

	if len(detail.Screenshots) > 0 {
		detail.PosterURL = detail.Screenshots[0]
		detail.Screenshots = detail.Screenshots[1:]
	}

	// 只从结构化字段提取禁转标记（不扫描简介正文，避免误判）
	detail.Flags = extractFlagsFromStructured(detail.Title, detail.Subtitle, detail.Tags)
	if imdbURL := reIMDbURL.FindString(html); imdbURL != "" {
		detail.IMDbURL = imdbURL
	}
	if doubanURL := reDoubanURL.FindString(html); doubanURL != "" {
		detail.DoubanURL = doubanURL
	}

	detail.RawHTML = html
	detail.EngineExtractorName = "legacy_regexp"
	return detail
}

func cleanRowText(row string) string {
	text := stripTags(row)
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = reNexusWhitespace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractBasicInfoFields(row string, detail *model.TorrentDetail) {
	preprocessed := reNexusDTDDBoundary.ReplaceAllString(row, "：")
	text := cleanRowText(preprocessed)

	type keyMatch struct {
		key      string
		valStart int
		pos      int
	}
	var matches []keyMatch
	for _, key := range basicInfoKeyList {
		re := regexp.MustCompile(key + `[：:]`)
		loc := re.FindStringIndex(text)
		if loc != nil {
			matches = append(matches, keyMatch{key: key, valStart: loc[1], pos: loc[0]})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].pos < matches[j].pos })

	for i, m := range matches {
		valEnd := len(text)
		if i+1 < len(matches) {
			valEnd = matches[i+1].pos
		}
		val := strings.TrimSpace(text[m.valStart:valEnd])

		switch m.key {
		case "大小":
			if detail.Size == 0 {
				if sm := reNexusSizeValue.FindStringSubmatch(val); len(sm) > 2 {
					detail.Size = parseSizeStr(sm[1] + " " + sm[2])
				}
			}
		case "类型":
			if detail.Category == "" {
				detail.Category = val
			}
		case "编码":
			detail.Codec = val
		case "分辨率":
			detail.Resolution = val
		case "媒介", "来源", "质量":
			if detail.Source == "" {
				detail.Source = val
			}
		case "音频编码":
			detail.AudioCodec = val
		case "制作组":
			detail.ReleaseGroup = val
		case "地区":
			detail.Region = val
		case "处理":
			detail.Processing = val
		}
	}
}

func extractCategoryFromImg(row string, detail *model.TorrentDetail) {
	imgRe := reNexusImgAlt
	for _, m := range imgRe.FindAllStringSubmatch(row, -1) {
		alt := strings.TrimSpace(m[1])
		if alt == "" || alt == "Show/Hide" || alt == "显示/隐藏" {
			continue
		}
		detail.Category = alt
		return
	}
	if m := reNexusCatLink.FindStringSubmatch(row); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}
}

func extractSubtitleFromRow(row string, detail *model.TorrentDetail) {
	tdRe := reNexusRowFollow
	m := tdRe.FindStringSubmatch(row)
	if len(m) < 2 {
		return
	}
	text := stripTags(m[1])
	text = reNexusWhitespace.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if text != "" {
		detail.Subtitle = text
	}
}

func extractInfoHashFromRow(row string) string {
	for _, re := range hashRowPatterns {
		if m := re.FindStringSubmatch(row); len(m) > 1 {
			return strings.ToLower(m[1])
		}
	}
	return ""
}

func extractScreenshotsFromBBCode(text string) []string {
	seen := make(map[string]bool)
	var urls []string
	for _, m := range reBBCodeImg.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			u := strings.TrimSpace(m[1])
			if u != "" && !seen[u] && !isUnwantedImage(u) {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	for _, m := range reBBCodeHTMLImg.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			u := strings.TrimSpace(m[1])
			if u != "" && !seen[u] && !isUnwantedImage(u) {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	return urls
}

func extractMediaInfoFromBBCode(text string) string {
	for _, m := range reBBCodeQuote.FindAllStringSubmatch(text, -1) {
		label := ""
		if len(m) > 1 {
			label = strings.ToLower(m[1])
		}
		content := ""
		if len(m) > 2 {
			content = strings.TrimSpace(m[2])
		}
		if content == "" {
			continue
		}
		preview := content
		if len(preview) > 200 {
			preview = preview[:200]
		}
		if strings.Contains(label, "mediainfo") ||
			strings.Contains(label, "bdinfo") ||
			strings.Contains(label, "编码信息") ||
			strings.Contains(strings.ToLower(preview), "general") {
			return content
		}
	}
	return ""
}

// extractFlagsFromStructured 只从结构化字段（标题/副标题/标签）提取禁转标记。
// 不扫描简介正文，避免"禁转PTT"等误判（§56.37 合规修复）。
func extractFlagsFromStructured(title, subtitle string, tags []string) []string {
	combined := title + " " + subtitle
	for _, t := range tags {
		combined += " " + t
	}
	return extractFlagsFromText(combined)
}

// extractFlagsFromText legacy 正则兜底路径的禁转标记提取。
// §59.28 I1: 复用 extract 包两层检测（站点标记形态 + 跨词/定向排除），
// 与 Engine 主路径口径一致（原裸 Contains 会跨词伪命中"严禁转发"）。
func extractFlagsFromText(text string) []string {
	return extract.ExtractFlagsFromText(text)
}

var unwantedImagePatterns = []string{"ico", "logo", "banner", "icon", "emoji", "smiley", "rank_", "badge"}

func isUnwantedImage(url string) bool {
	lower := strings.ToLower(url)
	for _, pattern := range unwantedImagePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
