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

type GazelleAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewGazelleAdapter(doer *HTTPDoer, logger *zap.Logger) *GazelleAdapter {
	return &GazelleAdapter{
		GenericAdapter: NewGenericAdapter("gazelle", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *GazelleAdapter) Framework() string { return "gazelle" }

func (a *GazelleAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/torrents.php?action=download&id=" + torrentID
	if config.Passkey != "" {
		u += "&passkey=" + config.Passkey
	} else if config.AuthKey != "" {
		u += "&authkey=" + config.AuthKey
	}

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

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (a *GazelleAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	if config.APIKey != "" {
		return a.detailViaAPI(ctx, config, torrentID)
	}
	return a.detailViaWeb(ctx, config, torrentID)
}

func (a *GazelleAdapter) detailViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/ajax.php?action=torrent&id=" + torrentID

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", config.APIKey)
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

	var result struct {
		Response struct {
			Torrent struct {
				FilePath string `json:"filePath"`
				Size     int64  `json:"size"`
				InfoHash string `json:"infoHash"`
				Seeders  int    `json:"seeders"`
				Leechers int    `json:"leechers"`
			} `json:"torrent"`
			Group struct {
				Name     string `json:"name"`
				Category struct {
					Name string `json:"name"`
				} `json:"category"`
			} `json:"group"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, parseError("解析 API 响应失败", err)
	}

	t := result.Response.Torrent
	g := result.Response.Group
	return &model.TorrentDetail{
		Title:    g.Name,
		Size:     t.Size,
		InfoHash: strings.ToLower(t.InfoHash),
		Category: g.Category.Name,
	}, nil
}

func (a *GazelleAdapter) detailViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/torrents.php?torrentid=" + torrentID

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

	detail := &model.TorrentDetail{}
	if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(html); len(m) > 1 {
		parts := strings.SplitN(m[1], " :: ", 2)
		detail.Title = strings.TrimSpace(parts[0])
	}
	if m := regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`).FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}
	if m := regexp.MustCompile(`(?i)(?:size|大小)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}

	return detail, nil
}

func (a *GazelleAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/torrents.php?torrentid=" + torrentID

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

	if strings.Contains(html, "freeleech") || strings.Contains(html, "free") {
		if strings.Contains(html, "double upload") || strings.Contains(html, "2x") {
			return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
		}
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if strings.Contains(html, "double upload") || strings.Contains(html, "2x upload") {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "50%") || strings.Contains(html, "half download") {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *GazelleAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/torrents.php?torrentid=" + torrentID

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
		strings.Contains(html, "must seed")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}
	return result, nil
}

func (a *GazelleAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	if config.APIKey != "" {
		return a.slViaAPI(ctx, config, torrentID)
	}
	return a.slViaWeb(ctx, config, torrentID)
}

func (a *GazelleAdapter) slViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/ajax.php?action=torrent&id="+torrentID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", config.APIKey)
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
	var result struct {
		Response struct {
			Torrent struct {
				Seeders  int `json:"seeders"`
				Leechers int `json:"leechers"`
			} `json:"torrent"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, parseError("解析响应失败", err)
	}
	return &model.SLData{Seeders: result.Response.Torrent.Seeders, Leechers: result.Response.Torrent.Leechers}, nil
}

func (a *GazelleAdapter) slViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/torrents.php?torrentid="+torrentID, nil)
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

	if m := regexp.MustCompile(`(?i)(?:seeders?|做种)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`(?i)(?:leechers?|下载)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

func (a *GazelleAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := resolveBase(config)
	uploadURL := baseURL + "/upload.php"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("file_input", "upload.torrent")
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
		fw.writeField("release_desc", req.Description)
	}

	gazelleFields := map[string]string{
		"cat":            "releasetype",
		"medium_sel":     "media",
		"codec_sel":      "format",
		"audiocodec_sel": "bitrate",
		"category":       "releasetype",
		"medium":         "media",
		"codec":          "format",
		"audioCodec":     "bitrate",
	}
	for srcType, targetName := range gazelleFields {
		if v, ok := req.FormFields[srcType]; ok {
			fw.writeField(targetName, v)
		}
	}

	if year, ok := req.ExtraFields["year"]; ok {
		fw.writeField("year", year)
	}
	if recordLabel, ok := req.ExtraFields["record_label"]; ok {
		fw.writeField("record_label", recordLabel)
	}
	if catalogueNumber, ok := req.ExtraFields["catalogue_number"]; ok {
		fw.writeField("catalogue_number", catalogueNumber)
	}
	if image, ok := req.ExtraFields["image"]; ok {
		fw.writeField("image", image)
	}
	if tags, ok := req.ExtraFields["tags"]; ok {
		fw.writeField("tags", tags)
	}
	if albumDesc, ok := req.ExtraFields["album_desc"]; ok {
		fw.writeField("album_desc", albumDesc)
	}

	if remaster, ok := req.ExtraFields["remaster"]; ok && remaster == "true" {
		fw.writeField("remaster", "true")
		if ry, ok := req.ExtraFields["remaster_year"]; ok {
			fw.writeField("remaster_year", ry)
		}
		if rt, ok := req.ExtraFields["remaster_title"]; ok {
			fw.writeField("remaster_title", rt)
		}
		if rrl, ok := req.ExtraFields["remaster_record_label"]; ok {
			fw.writeField("remaster_record_label", rrl)
		}
		if rcn, ok := req.ExtraFields["remaster_catalogue_number"]; ok {
			fw.writeField("remaster_catalogue_number", rcn)
		}
	}

	if v, ok := req.ExtraFields["vanity_house"]; ok && v == "true" {
		fw.writeField("vanity_house", "on")
	}
	if v, ok := req.ExtraFields["scene"]; ok && v == "true" {
		fw.writeField("scene", "on")
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "audiocodec_sel":
			continue
		}
		fw.writeField(k, v)
	}
	for k, v := range req.ExtraFields {
		switch k {
		case "year", "record_label", "catalogue_number", "image", "tags",
			"album_desc", "remaster", "remaster_year", "remaster_title",
			"remaster_record_label", "remaster_catalogue_number",
			"vanity_house", "scene":
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
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足"}
	}

	if idMatch := regexp.MustCompile(`torrents\.php\?torrentid=(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/torrents.php?torrentid=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if idMatch := regexp.MustCompile(`torrents\.php\?id=(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/torrents.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "success") || strings.Contains(html, "成功") || strings.Contains(html, "uploaded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := regexp.MustCompile(`class="error"[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	} else if m := regexp.MustCompile(`<p[^>]*>([^<]*(?:error|fail|already|duplicate|exist)[^<]*)</p>`).FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *GazelleAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未找到 info_hash")
	}
	return detail.InfoHash, nil
}

func (a *GazelleAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
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
