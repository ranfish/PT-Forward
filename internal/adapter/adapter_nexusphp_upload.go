package adapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	htmllib "html"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (a *NexusPHPAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	domain := config.BaseURL
	if domain == "" {
		domain = config.Domain
	}
	if err := httpclient.GlobalDownloadLimiter.AcquireWithLimit(domain, config.DownloadHourlyLimit); err != nil {
		return nil, downloadError("下载限流: "+err.Error(), nil)
	}

	var u string
	var method = "GET"
	switch config.DownloadMode {
	case "signed":
		// 先例形态：download.php?id={tid}&t=N&sign=hex（POST）
		signedURL, err := a.resolveSignedDownloadURL(ctx, config, torrentID)
		if err != nil {
			return nil, err
		}
		u = signedURL
		method = "POST"
	case "signed_hash":
		// §59.510 hdwing（瓷器）形态：download.php?hash=base64(1\ntid\nuid).签名
		// （GET·标准 id= 形态站方 404 禁用——hash 签名链接列表/详情页均可提取）
		hashURL, err := a.resolveHashDownloadURL(ctx, config, torrentID)
		if err != nil {
			return nil, err
		}
		u = hashURL
	default:
		if config.DownloadURLTemplate != "" {
			u = buildTemplateURL(config, config.DownloadURLTemplate, torrentID)
		} else {
			// §59.159: NP 族公共单点（download_url.go——双副本收敛）
			u = BuildNexusDownloadURL(baseURLOf(config), torrentID, config.Passkey)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "download")
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return nil, &model.AppError{Code: 15001, Message: "返回了 HTML 页面而非种子文件，下载链接可能有误"}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, downloadError("读取种子数据失败", err)
	}
	if len(data) == 0 {
		return nil, &model.AppError{Code: 15001, Message: "种子数据为空"}
	}
	return data, nil
}

func (a *NexusPHPAdapter) resolveSignedDownloadURL(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detailURL := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return "", networkError("构造详情页请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", downloadError("获取详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", downloadError("读取详情页失败", err)
	}

	idEscaped := regexp.QuoteMeta(torrentID)
	pattern := regexp.MustCompile(`download\.php\?id=` + idEscaped + `&t=\d+&sign=[a-f0-9]+`)
	match := pattern.FindString(string(body))
	if match == "" {
		return "", configError("未在详情页找到签名下载链接")
	}

	if !strings.HasPrefix(match, "http") {
		base := config.Domain
		if !strings.HasPrefix(base, "http") {
			base = "https://" + base
		}
		match = base + "/" + strings.TrimLeft(match, "/")
	}

	return match, nil
}

// reHdwingHashDownload hdwing hash 签名下载链接：
// download.php?hash=MQo4MzgzMDMKMjY4NjE4.aRSFTOhrcUFQkQtO3b1fO8kJ2MCN54AEu9PAVLQPwZE
// 前段=URL-safe base64("1\n{tid}\n{uid}")，后段=站方签名（字母数字-_ 字符集）。
var reHdwingHashDownload = regexp.MustCompile(`download\.php\?hash=[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+`)

// resolveHashDownloadURL §59.510 hdwing（瓷器）：详情页提取 hash 签名下载链接。
// 站方禁用标准 download.php?id= 形态（404）——hash 链接列表/详情页均可提取，
// 走详情页（与 signed 模式同构·getTorrentDetailInner 主链同源 HTML 场景另议）。
func (a *NexusPHPAdapter) resolveHashDownloadURL(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detailURL := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return "", networkError("构造详情页请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", downloadError("获取详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", downloadError("读取详情页失败", err)
	}

	match := reHdwingHashDownload.FindString(string(body))
	if match == "" {
		return "", configError("未在详情页找到 hash 签名下载链接")
	}

	if !strings.HasPrefix(match, "http") {
		base := config.Domain
		if !strings.HasPrefix(base, "http") {
			base = "https://" + base
		}
		match = base + "/" + strings.TrimLeft(match, "/")
	}
	return match, nil
}

func (a *NexusPHPAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := config.Domain
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://" + baseURL
	}
	// §59.159: NP 上传 POST 端点=TakeUpload（takeupload.php 处理页）；Upload
	// （upload.php）是表单 GET 页——POST 只回显表单=实战静默失败之源。历史代码
	// 误用 Upload 且 TakeUpload 误限音乐类。优先级：TakeUpload > 显式 Upload 配置
	// > 默认 takeupload.php。
	uploadPath := config.Paths.TakeUpload
	if uploadPath == "" {
		uploadPath = config.Paths.Upload
	}
	if uploadPath == "" {
		uploadPath = "/takeupload.php"
	}

	uploadURL := baseURL + uploadPath

	// §59.494: NP 新版 CSRF _token（优堡案实证——表单 hidden _token·
	// 缺失被拒"请填写必填项目"）。通用提取：GET upload 页拿 token
	// （会话绑定）·有则带·无则不带（老 NP 站零影响——幸运/南洋/葡萄
	// 实证无 _token 也过）。上传低频·多一个 GET 成本可忽略。
	csrf := a.fetchUploadCSRFToken(ctx, config, baseURL+config.Paths.Upload)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		fw.writeField("name", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("small_descr", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		fw.writeField("url", req.IMDbLink)
	}
	if req.Anonymous {
		if csrf != "" {
			fw.writeField("_token", csrf) // §59.494: NP 新版 CSRF（优堡案）
		}
		fw.writeField("uplver", "yes") // §59.159: 幸运 checkbox value=yes（NP 存在即勾，值对齐站方）
	}

	npFieldMap := map[string]string{
		"category":   "type",
		"source":     "source_sel",
		"resolution": "standard_sel",
		"codec":      "codec_sel",
		"audioCodec": "audiocodec_sel",
		"medium":     "medium_sel",
		"team":       "team_sel",
		"processing": "processing_sel",
		"douban":     "douban",
	}

	for k, v := range config.Publish.FormFields {
		if _, exists := npFieldMap[k]; exists {
			npFieldMap[k] = v
		}
	}

	musicCat := ""
	if cat, ok := req.FormFields["category"]; ok {
		musicCat = cat
	}
	isMusic := isMusicCategory(musicCat)

	for k, v := range req.FormFields {
		if isMusic {
			if mapped, ok := npMusicFieldMap[k]; ok {
				fw.writeField(mapped, v)
			} else if mapped, ok := npFieldMap[k]; ok && k == "category" {
				fw.writeField(mapped, v)
			} else {
				fw.writeField(k, v)
			}
		} else {
			if mapped, ok := npFieldMap[k]; ok {
				fw.writeField(mapped, v)
			} else {
				fw.writeField(k, v)
			}
		}
	}

	// §59.159 断头修复：TagArrayFields 写入（checkbox 数组标签 tags[4][N]）。
	// 此前只有诊断日志无实际 writeField → tags 全丢 → LuckAudit
	// "未选择任何标签/粤语/中字标签"审核拒绝三连（实战私信实证）
	for _, kv := range req.TagArrayFields {
		fw.writeField(kv.Key, kv.Value)
	}

	if err := fw.hasError(); err != nil {
		return nil, fmt.Errorf("write form field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, networkError("关闭 multipart writer 失败", err)
	}
	// §59.159 诊断：multipart 字段清单（tags 投递实证——LuckAudit"未选择任何标签"排查）
	if a.logger != nil {
		a.logger.Info("upload multipart fields",
			zap.String("site", config.Domain),
			zap.Int("form_fields", len(req.FormFields)),
			zap.Int("tag_array_fields", len(req.TagArrayFields)),
			zap.Any("tag_sample", req.TagArrayFields[:min(6, len(req.TagArrayFields))]))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	setCommonHeaders(httpReq, config.Cookie)
	// §59.501: 好多油临时上传 Basic Auth（用户定案：确定性发送不探测——
	// 站方网关级 401 realm="PT Upload" 挑战；带凭证仍 401=cookie/权限问题
	// 诊断无歧义；空凭证零行为；站点放开限制后用户清空字段即终）
	if config.UploadAuth != "" && strings.Contains(config.Domain, "hdupt.com") {
		httpReq.Header.Set("Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(config.UploadAuth)))
	}

	// §59.159: 重定向 existed=1 捕获（NP 重复上传 302 details.php?id=N&existed=1 形态；
	// 幸运实测为 200 失败页形态——双形态兼容，transport 继承站点代理）
	existingID, finalID := "", ""
	uploadRedirectSuccess := false
	// §59.470: 上传专用超时 120s——慢站 takeupload 站方处理可 >30s（南洋案
	// 实证：09:42:57 发起→站方 09:43:44 完成入库，但 doer 默认 30s 超时先到
	// →假失败+人工重试险重复发布）。浅拷贝共享 Transport（连接池/smart dial
	// 复用）·快站零影响（超时是上限非等待——响应即返回；真死连接由 Transport
	// 层 dial 超时快失败）。
	uploadTimeout := 120 * time.Second
	redirectClient := &http.Client{
		Timeout:   uploadTimeout,
		Transport: a.doer.Client.Transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			id := req.URL.Query().Get("id")
			if req.URL.Query().Get("existed") == "1" && id != "" {
				existingID = id
				// §59.501 附三：成功/已存在形态 302 不再跟随——Go http
				// 跟随 redirect 不保留 Authorization 头 → 网关拦 details
				// GET 403 覆盖判定（243 tid=89170 实证上传已成功却报
				// "authentication failed (HTTP 403)"假失败）。existed=1
				// 即重复上传明确签名，无需页面佐证。
				return http.ErrUseLastResponse
			}
			if id != "" && strings.Contains(req.URL.Path, "details.php") {
				finalID = id
				// §59.501 附三：uploaded=1 是成功页权威签名（§59.159 形态①
				// ——成功页标志参数，推荐位链接不带），Location 即铁证，
				// 不跟随（同上 403 假失败）。
				if req.URL.Query().Get("uploaded") == "1" {
					uploadRedirectSuccess = true
					return http.ErrUseLastResponse
				}
			}
			return nil
		},
	}
	resp, err := redirectClient.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	html := string(body)

	if resp.StatusCode != http.StatusOK {
		// §59.501 附三：成功形态 302（未跟随）——先于错误分类判定
		if existingID != "" {
			return &model.PublishResponse{
				Success: true, IsExisting: true, ExistingID: existingID,
				DetailURL:  baseURL + "/details.php?id=" + existingID,
				TargetSite: config.Domain,
			}, nil
		}
		if finalID != "" && uploadRedirectSuccess {
			return &model.PublishResponse{
				Success: true, TorrentID: finalID,
				DetailURL:  baseURL + "/details.php?id=" + finalID,
				TargetSite: config.Domain,
				IDSource:   "redirect",
			}, nil
		}
		return nil, classifySiteError(resp, "upload")
	}

	// §59.159: 公共判定单点（upload_classify.go——双副本教训后收敛）
	if r := classifyUploadHTML(a.logger, config.Domain, baseURL, html, existingID, finalID); r != nil {
		return r, nil
	}

	// §59.289: 错误提取单点（h1+p 组合增强）+ 未知响应可观测化（响应体
	// 片段入日志——"未知响应"不再吞真凶）
	errMsg := ExtractUploadError(html)
	if errMsg == "" {
		errMsg = "上传失败: 未知响应"
		if a.logger != nil {
			a.logger.Error("upload unknown response body",
				zap.String("site", config.Domain),
				zap.String("html_head", html[:min(300, len(html))]))
		}
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

// reTorrentTableSig §59.209: 结果页结构签名（任意元素 class 含 torrents——
// 含 table 主题与 div 主题如 人人 class="page-torrents page-torrent-list"）。
// 有签名=真实搜索结果页（真空也合法）；无标记无签名=异常轻量页。
// 判别实证：spring 16 并发 503 页 0 字节无签名（保持 anomaly）×
// 人人真空页 78k 带签名（真实空）。
var reTorrentTableSig = regexp.MustCompile(`(?i)class="[^"]*torrents`)

func (a *NexusPHPAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}

	browsePaths := []string{"/browse.php", "/torrents.php"}
	searchParam := config.Paths.SearchParam

	if browsePaths[0] != config.Paths.Browse && config.Paths.Browse != "" {
		browsePaths = []string{config.Paths.Browse}
	} else if d, ok := getSiteSearchDefaults(config.Domain); ok {
		if d.Browse != "" {
			browsePaths = []string{d.Browse}
		}
		if searchParam == "" && d.Param != "" {
			searchParam = d.Param
		}
	}

	if searchParam == "" {
		searchParam = "search"
	}

	catParam := ""
	if opts != nil && opts.Category != "" {
		catParam = "&cat=" + opts.Category
	}
	// §59.30: search_area 可选（默认 0=标题；1=简介——keepfrds 等站英文种子名
	// 只出现在简介/副标题区，标题字段仅索引中文格式化名）
	areaParam := "0"
	if opts != nil && opts.SearchArea != "" {
		areaParam = opts.SearchArea
	}
	// §59.486: 翻页参数（NP page 0 起——皇后无排序功能（列头零排序链接
	// 实证）·order_by 方案废弃；大合集场景同体积真身在默认序 60+ 位·
	// 第一页 50 截断外·音乐第三轮翻页聚合覆盖）
	pageParam := ""
	if opts != nil && opts.Page > 0 {
		pageParam = "&page=" + fmt.Sprintf("%d", opts.Page)
	}

	var lastErr error
	for _, bp := range browsePaths {
		searchURL := u + bp + "?" + searchParam + "=" + url.QueryEscape(keyword) + "&search_area=" + areaParam + "&search_mode=0&incldead=1" + catParam + pageParam

		// §59.213: 全站搜索调试（debug 级）——audiences-only 时代产物 universal 化：
		// PT30 容器 vs 同容器探针结果分叉案（密阳 7 行 vs 0 行，同栈同 cookie
		// 同网络）需要请求级可观测收口。
		if a.logger != nil {
			a.logger.Debug("nexusphp search req",
				zap.String("domain", config.Domain),
				zap.String("url", searchURL))
		}

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			return nil, searchError("构造搜索请求失败", err)
		}
		setCommonHeaders(req, config.Cookie)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			lastErr = searchError(fmtES("search %s failed: %v", bp, err), nil)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			drainBody(resp)
			lastErr = httpError(fmtES("HTTP %d on %s", resp.StatusCode, bp), nil)
			continue
		}

		body, readErr := readBody(resp)
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = classifySiteError(resp, "search")
			continue
		}

		parsed := parseNexusPHPBrowseLimit(string(body), config, searchLimitOf(opts))
		if a.logger != nil {
			a.logger.Debug("nexusphp search resp",
				zap.String("domain", config.Domain),
				zap.String("bp", bp),
				zap.Int("status", resp.StatusCode),
				zap.Int("body_bytes", len(body)),
				zap.Int("parsed", len(parsed)))
		}

		// §59.199: 异常页检测——0 行且页面无空结果标记（真空页 NexusPHP 标准主题
		// 必含"没有种子/搜索结果"——springsunday 实测）且无结果表格结构签名
		// （§59.209: 部分站点真空页无文本标记但有完整 torrents 表格——人人
		// audiences.me 实测 78k 真空页零标记，纯标记判据误报 anomaly）＝页面
		// 异常形态：站点并发限流 503（24 并发实测 17×503）被 retry 中间件耗尽
		// 后返回的轻量错误页/降级页。此前静默当"确认无结果"——Phase-2 恒 0 行
		// 且零日志（生死格斗案：PT30 批次窗口 spring 搜索全盲）。按站点错误
		// 上报，交上层退避重试。
		if len(parsed) == 0 &&
			!strings.Contains(string(body), "没有种子") && !strings.Contains(string(body), "没有符合") &&
			!strings.Contains(string(body), "搜索结果") && !strings.Contains(string(body), "Nothing") &&
			!reTorrentTableSig.MatchString(string(body)) {
			if a.logger != nil {
				a.logger.Debug("nexusphp search: anomaly page (0 rows, no empty marker)",
					zap.String("domain", config.Domain),
					zap.Int("status", resp.StatusCode),
					zap.Int("body_bytes", len(body)))
			}
			lastErr = httpError(fmtES("search page anomaly on %s (status=%d, bytes=%d, no empty marker)", bp, resp.StatusCode, len(body)), nil)
			continue
		}

		// §59.30: 间歇性 session 失效防护——请求带 search 参数但返回页无搜索结果
		// 标记（NexusPHP 对失效 session 忽略参数返回默认列表），重试一次。
		if len(parsed) > 0 && !strings.Contains(string(body), "搜索结果") && !strings.Contains(string(body), "没有种子") && !strings.Contains(string(body), "Nothing") {
			if a.logger != nil {
				a.logger.Warn("nexusphp search: response is default list (session dropped?), retrying",
					zap.String("domain", config.Domain), zap.Int("parsed", len(parsed)))
			}
			retryReq, rErr := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
			if rErr == nil {
				setCommonHeaders(retryReq, config.Cookie)
				if retryResp, rrErr := a.doer.Client.Do(retryReq); rrErr == nil {
					retryBody, rbErr := readBody(retryResp)
					drainBody(retryResp)
					if rbErr == nil && retryResp.StatusCode == http.StatusOK {
						if strings.Contains(string(retryBody), "搜索结果") || strings.Contains(string(retryBody), "没有种子") {
							parsed = parseNexusPHPBrowse(string(retryBody), config)
						}
					}
				}
			}
		}

		if len(parsed) == 0 && (strings.Contains(config.Domain, "haidan") || strings.Contains(config.Domain, "hdcity")) {
			a.logger.Info("haidan debug", zap.String("bp", bp), zap.Int("body_len", len(body)))
		}
		if len(parsed) == 0 && strings.Contains(config.Domain, "audiences") {
			titleStr := "no title"
			if m := regexp.MustCompile(`<title>([^<]+)`).FindStringSubmatch(string(body)); len(m) > 1 {
				titleStr = m[1]
				if len(titleStr) > 50 {
					titleStr = titleStr[:50]
				}
			}
			a.logger.Info("audiences parse result",
				zap.String("bp", bp),
				zap.Int("body_len", len(body)),
				zap.String("title", titleStr))
		}
		return parsed, nil
	}

	return nil, lastErr
}

func (a *NexusPHPAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未在详情页找到 info_hash")
	}
	return detail.InfoHash, nil
}

func (a *NexusPHPAdapter) SupportsSearchByPiecesHash() bool { return true }

// SearchByPiecesHash calls NexusPHP pieces-hash API（§33.32 §33.34）。
//
// Authentication modes:
//   - passkey (standard NP, 52 sites): passkey in body, cookie optional
//   - cookie-only (keepfrds): cookie required, passkey ignored
//
// API paths (auto-fallback):
//   - /api/pieces-hash (standard, 52 sites)
//   - /api/torrents/pieces-hash (keepfrds variant)
//
// Response formats:
//   - standard NP: {"ret":0,"msg":"...","data":{"hash":tid}}
//   - keepfrds:    {"data":{"hash":tid}}  (no ret/msg fields)
//
// Max 100 hashes per request (NexusPHP TorrentRepository::getPiecesHashCache limit).
func (a *NexusPHPAdapter) SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error) {
	if len(piecesHashes) == 0 {
		return nil, nil
	}

	hasPasskey := config.Passkey != "" && len(config.Passkey) == 32
	hasCookie := config.Cookie != ""
	if !hasPasskey && !hasCookie {
		return nil, configError("pieces_hash API 需要 passkey 或 cookie 认证")
	}

	u := config.Domain
	if config.APIDomain != "" {
		u = config.APIDomain
	}
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}

	const maxBatch = 100
	hashes := piecesHashes
	if len(hashes) > maxBatch {
		hashes = hashes[:maxBatch]
	}

	bodyMap := map[string]any{
		"pieces_hash": hashes,
	}
	if hasPasskey {
		bodyMap["passkey"] = config.Passkey
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, parseError("构造 pieces_hash 请求失败", err)
	}

	paths := []string{"/api/pieces-hash", "/api/torrents/pieces-hash"}
	var lastErr error
	for _, path := range paths {
		result, err := a.doPiecesHashRequest(ctx, u+path, bodyBytes, config.Cookie)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isPiecesHash404(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func isPiecesHash404(err error) bool {
	appErr, ok := err.(*model.AppError)
	if !ok {
		return false
	}
	return strings.Contains(appErr.Message, "HTTP 404")
}

func (a *NexusPHPAdapter) doPiecesHashRequest(ctx context.Context, apiURL string, body []byte, cookie string) (map[string]int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, networkError("构造 pieces_hash 请求失败", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("pieces_hash API 请求失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("pieces_hash API HTTP %d", resp.StatusCode), nil)
	}

	respBody, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Ret  int            `json:"ret"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, parseError("解析 pieces_hash 响应失败", err)
	}

	if apiResp.Ret != 0 {
		return nil, authError(fmtES("pieces_hash API 返回错误: %s", apiResp.Msg), nil)
	}

	result := make(map[string]int, len(apiResp.Data))
	for ph, v := range apiResp.Data {
		switch n := v.(type) {
		case float64:
			result[ph] = int(n)
		case string:
			if id, err := strconv.Atoi(n); err == nil {
				result[ph] = id
			}
		}
	}
	return result, nil
}

func (a *NexusPHPAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	results, err := a.SearchTorrents(ctx, config, torrentID, nil)
	if err != nil {
		return false, fmt.Errorf("search for verify exists %q: %w", torrentID, err)
	}
	for _, r := range results {
		if r.TorrentID == torrentID {
			return true, nil
		}
	}
	return false, nil
}

// preflightUploadFormRe §59.176: 发布表单存在性信号——文件选择控件/takeupload
// 提交端点/常见文件字段名（hdfans 实证：有权限页必含表单；帮助文本含
// "权限/不允许"字样是噪音——表单存在性是唯一权威信号，不扫关键词）。
var preflightUploadFormRe = regexp.MustCompile(`(?i)<input[^>]+type=["']?file|takeupload\.php|name=["']?(tfile|file_upload|torrent)`)

// preflightLoginRe §59.176: 登录墙信号（NP 登录表单端点）。
var preflightLoginRe = regexp.MustCompile(`(?i)takelogin\.php`)

// PreflightPublish §59.176: NP 族发布前置检查。
// GET 发布表单页（upload.php）：表单在=允许；登录墙=凭证失效；
// 无表单=站点拒绝发布（提取正文文案——幸运"被拒种子数:10 达到上限"实测形态，
// 52movie 无权限"你没有发布种子的权限"标准形态——短文案页无表单噪音）。
func (a *NexusPHPAdapter) PreflightPublish(ctx context.Context, config *model.SiteConfig) (*model.PublishPreflightResult, error) {
	baseURL := config.Domain
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://" + baseURL
	}
	// 表单 GET 页路径：显式 Upload 配置（非 takeupload）优先，否则标准 /upload.php
	path := "/upload.php"
	if config.Paths.Upload != "" && !strings.Contains(config.Paths.Upload, "takeupload") {
		path = config.Paths.Upload
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+path, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求发布页失败", err)
	}
	defer func() { drainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}
	bodyBytes, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	if preflightLoginRe.MatchString(body) {
		return &model.PublishPreflightResult{
			Allowed: false,
			Reason:  "站点登录态失效（发布页返回登录表单）——请检查站点凭证/CookieCloud 同步",
		}, nil
	}
	if preflightUploadFormRe.MatchString(body) {
		return &model.PublishPreflightResult{Allowed: true}, nil
	}

	// 无表单：提取正文文案（短拒绝页——去标签取正文，裁剪导航残留）
	text := stripTagsForPreflight(body)
	return &model.PublishPreflightResult{
		Allowed: false,
		Reason:  "站点不允许发布：" + text,
	}, nil
}

// stripTagsForPreflight §59.176: 拒绝文案提取——去脚本样式标签、压缩空白、
// 截取核心段（拒绝页正文短；从首个中文/冒号句起截，跳过站点导航头部）。
func stripTagsForPreflight(body string) string {
	text := regexp.MustCompile(`(?s)<script.*?</script>`).ReplaceAllString(body, " ")
	text = regexp.MustCompile(`(?s)<style.*?</style>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	text = htmllib.UnescapeString(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	// 跳过导航残留：从"对不起/抱歉/您/你/当前/发布"等文案起点的最近处截取
	best := -1
	for _, kw := range []string{"对不起", "抱歉", "你没有", "您没有", "当前", "不允许", "禁止"} {
		if i := strings.Index(text, kw); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	if best > 0 {
		text = text[best:]
	}
	if len(text) > 160 {
		text = text[:160] + "…"
	}
	if text == "" {
		text = "（发布页无表单且未提取到文案——请登录站点查看）"
	}
	return text
}

// §59.494: NP 新版 CSRF _token 提取（优堡案——upload 表单 hidden
// _token·Laravel 式）。GET upload 页正则提取；页面缺失/请求失败返回空
// （调用方不带——老 NP 站兼容）。token 会话绑定·与 cookie 同栈请求。
var reNPUploadCSRF = regexp.MustCompile(`name="_token"[^>]*value="([a-f0-9]{20,})"`)

func (a *NexusPHPAdapter) fetchUploadCSRFToken(ctx context.Context, config *model.SiteConfig, uploadPageURL string) string {
	if uploadPageURL == "" {
		return ""
	}
	html, err := a.fetchDetailsHTMLViaHTTP(ctx, config, uploadPageURL)
	if err != nil {
		return ""
	}
	if m := reNPUploadCSRF.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}
