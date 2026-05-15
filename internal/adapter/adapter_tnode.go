package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type TNodeAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewTNodeAdapter(doer *HTTPDoer, logger *zap.Logger) *TNodeAdapter {
	return &TNodeAdapter{
		GenericAdapter: NewGenericAdapter("tnode", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *TNodeAdapter) Framework() string { return "tnode" }

func (a *TNodeAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := buildDomainURL(config, "/download.php", torrentID, config.Passkey)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: cookie 可能已过期"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		return nil, &model.AppError{Code: 15001, Message: "返回了 HTML 页面而非种子文件"}
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (a *TNodeAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := buildDomainURL(config, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求详情页失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError("读取响应失败", err)
	}

	html := string(body)
	detail := &model.TorrentDetail{}

	if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}

	if m := regexp.MustCompile(`(?i)info_hash.*?<td[^>]*>([a-fA-F0-9]{40})`).FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}

	if m := regexp.MustCompile(`(?i)(?:大小|Size|体积)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}

	if m := regexp.MustCompile(`(?i)(?:分类|Category|类型)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}

	detail.Tags = extractTags(html)

	descRe := regexp.MustCompile(`(?s)(?:简介|Description|描述)</h\d?>.*?<div[^>]*>(.*?)</div>`)
	if m := descRe.FindStringSubmatch(html); len(m) > 1 {
		detail.Description = strings.TrimSpace(m[1])
	}

	return detail, nil
}

func (a *TNodeAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := buildDomainURL(config, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求优惠信息失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	if strings.Contains(html, "pro_free2up") || strings.Contains(html, "2x免费") {
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_2up") || strings.Contains(html, "2x上传") {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_free") || strings.Contains(html, "免费") {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if strings.Contains(html, "pro_50p") || strings.Contains(html, "50%") {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}
	if strings.Contains(html, "pro_30p") || strings.Contains(html, "30%") {
		return &model.DiscountResult{Level: model.DiscountPercent30}, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *TNodeAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	u := buildDomainURL(config, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求HR信息失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	hasHR := strings.Contains(html, "hit and run") ||
		strings.Contains(html, "hit&run") ||
		strings.Contains(html, "h&r") ||
		strings.Contains(html, "考核") ||
		strings.Contains(html, "hitandrun")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}
	return result, nil
}

func (a *TNodeAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := buildDomainURL(config, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)
	sl := &model.SLData{}

	if m := regexp.MustCompile(`(?i)(?:做种|Seeders?|保种)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`(?i)(?:下载|Leechers?)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

func (a *TNodeAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未在详情页找到 info_hash")
	}
	return detail.InfoHash, nil
}

func (a *TNodeAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := config.Domain
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://" + baseURL
	}

	csrfToken, err := a.fetchCSRFToken(ctx, config, baseURL+"/torrent/upload")
	if err != nil {
		return nil, authError("获取 CSRF token 失败", err)
	}

	uploadURL := baseURL + "/api/torrent/upload"

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("torrent", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		fw.writeField("title", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("subtitle", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("note", req.Description)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}
	if len(req.Screenshots) > 0 {
		fw.writeField("screenshot", strings.Join(req.Screenshots, "\n"))
	}
	if req.Anonymous {
		fw.writeField("anonymous", "true")
	}
	fw.writeField("confirm", "true")
	fw.writeField("zwex", "0")

	if req.IMDbLink != "" {
		if tmdbID, ok := req.ExtraFields["tmdbid"]; ok {
			fw.writeField("tmdbid", tmdbID)
		}
		if tmdbType, ok := req.ExtraFields["tmdbtype"]; ok {
			fw.writeField("tmdbtype", tmdbType)
		}
	}

	fieldMapping := map[string]string{
		"cat":            "category",
		"medium_sel":     "medium",
		"codec_sel":      "videoCoding",
		"standard_sel":   "resolution",
		"audiocodec_sel": "",
		"category":       "category",
		"medium":         "medium",
		"codec":          "videoCoding",
		"resolution":     "resolution",
	}
	for srcType, targetName := range fieldMapping {
		if targetName == "" {
			continue
		}
		if v, ok := req.FormFields[srcType]; ok {
			fw.writeField(targetName, v)
		}
	}

	if tags, ok := req.FormFields["tags"]; ok {
		fw.writeField("tags", tags)
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "standard_sel", "audiocodec_sel", "tags",
			"category", "medium", "codec", "resolution":
			continue
		}
		fw.writeField(k, v)
	}

	if err := fw.hasError(); err != nil {
		return nil, fmt.Errorf("write form field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, networkError("关闭 multipart writer 失败", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("x-csrf-token", csrfToken)
	httpReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")
	httpReq.Header.Set("Referer", baseURL+"/torrent/upload")
	httpReq.Header.Set("Origin", baseURL)
	setCommonHeaders(httpReq, config.Cookie)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result struct {
		Status int `json:"status"`
		Data   struct {
			Code string `json:"code"`
			ID   string `json:"id"`
		} `json:"data"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(body, &result); err == nil {
		if result.Data.Code == "UPLOAD_SUCCESS" || result.Status == 200 {
			torrentID := result.Data.ID
			return &model.PublishResponse{
				Success:    true,
				TorrentID:  torrentID,
				DetailURL:  baseURL + "/torrent/info/" + torrentID,
				TargetSite: config.Domain,
			}, nil
		}
		if result.Code == "TORRENT_ALREADY_UPLOAD" {
			return nil, &model.AppError{Code: 14001, Message: "种子已存在"}
		}
		errMsg := result.Message
		if errMsg == "" {
			errMsg = result.Code
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)
		}
		return nil, &model.AppError{Code: 15001, Message: errMsg}
	}

	html := string(body)
	if strings.Contains(html, "成功") || strings.Contains(html, "UPLOAD_SUCCESS") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)}
}

func (a *TNodeAdapter) fetchCSRFToken(ctx context.Context, config *model.SiteConfig, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", networkError("请求页面失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}
	html := string(body)

	re := regexp.MustCompile(`<meta\s+name="x-csrf-token"\s+content="([^"]+)"`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return m[1], nil
	}

	re2 := regexp.MustCompile(`(?i)csrf[_-]?token["\s:=]+["']?([a-zA-Z0-9_-]+)`)
	if m := re2.FindStringSubmatch(html); len(m) > 1 {
		return m[1], nil
	}

	return "", notFoundError("未在页面中找到 CSRF token")
}

func buildDomainURL(config *model.SiteConfig, path, torrentID, passkey string) string {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	u += path + "?id=" + torrentID
	if passkey != "" {
		u += "&passkey=" + passkey
	}
	return u
}

func (a *TNodeAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	results, err := a.SearchTorrents(ctx, config, torrentID, nil)
	if err != nil {
		return false, nil
	}
	for _, r := range results {
		if r.TorrentID == torrentID {
			return true, nil
		}
	}
	return false, nil
}
