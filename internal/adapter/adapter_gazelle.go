package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

var (
	reGazelleTitle      = regexp.MustCompile(`<title>([^<]+)</title>`)
	reGazelleInfoHash   = regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`)
	reGazelleSize       = regexp.MustCompile(`(?i)(?:size|大小)[^<]*<[^>]*>([^<]+)`)
	reGazelleSeeders    = regexp.MustCompile(`(?i)(?:seeders?|做种)[^<]*<[^>]*>(\d+)`)
	reGazelleLeechers   = regexp.MustCompile(`(?i)(?:leechers?|下载)[^<]*<[^>]*>(\d+)`)
	reGazelleTorrentID  = regexp.MustCompile(`torrents\.php\?torrentid=(\d+)`)
	reGazelleGroupID    = regexp.MustCompile(`torrents\.php\?id=(\d+)`)
	reGazelleErrorClass = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
	reGazelleErrorP     = regexp.MustCompile(`<p[^>]*>([^<]*(?:error|fail|already|duplicate|exist)[^<]*)</p>`)
	reGazelleUserBonus   = regexp.MustCompile(`(?i)积分:\s*([\d,]+)`)
	reGazelleSeedingNum  = regexp.MustCompile(`(?i)当前做种[^<]*<[^>]*>(\d+)`)
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
	u := baseURL + "/torrents.php?action=download&id=" + url.QueryEscape(torrentID)
	if config.Passkey != "" {
		u += "&passkey=" + url.QueryEscape(config.Passkey)
	} else if config.AuthKey != "" {
		u += "&authkey=" + url.QueryEscape(config.AuthKey)
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
	defer func() { drainBody(resp) }()

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
	defer func() { drainBody(resp) }()

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
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	detail := &model.TorrentDetail{}
	if m := reGazelleTitle.FindStringSubmatch(html); len(m) > 1 {
		parts := strings.SplitN(m[1], " :: ", 2)
		detail.Title = strings.TrimSpace(parts[0])
	}
	if m := reGazelleInfoHash.FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}
	if m := reGazelleSize.FindStringSubmatch(html); len(m) > 1 {
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
	defer func() { drainBody(resp) }()

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
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	hasHR := detectHRFromHTML(html) ||
		strings.Contains(html, "must seed")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}
	return result, nil
}

func (a *GazelleAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/torrents.php?torrentid=" + torrentID

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, nil, networkError("请求种子页面失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, nil, err
	}
	html := strings.ToLower(string(body))

	hrResult := &model.HRResult{HasHR: detectHRFromHTML(html) ||
		strings.Contains(html, "must seed")}
	if hrResult.HasHR {
		hrResult.SeedTimeH = config.HR.SeedTimeH()
	}

	var discResult *model.DiscountResult
	if strings.Contains(html, "freeleech") || strings.Contains(html, "free") {
		if strings.Contains(html, "double upload") || strings.Contains(html, "2x") {
			discResult = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
		} else {
			discResult = &model.DiscountResult{Level: model.DiscountFree}
		}
	} else if strings.Contains(html, "double upload") || strings.Contains(html, "2x upload") {
		discResult = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	} else if strings.Contains(html, "50%") || strings.Contains(html, "half download") {
		discResult = &model.DiscountResult{Level: model.DiscountPercent50}
	} else {
		discResult = &model.DiscountResult{Level: model.DiscountNone}
	}

	return hrResult, discResult, nil
}

func (a *GazelleAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	result := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		sl, err := a.GetPreciseSLData(ctx, config, id)
		if err != nil {
			a.logger.Warn("获取SL数据失败", zap.String("torrentID", id), zap.Error(err))
			continue
		}
		result[id] = sl
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
	defer func() { drainBody(resp) }()

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
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)
	sl := &model.SLData{}

	if m := reGazelleSeeders.FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := reGazelleLeechers.FindStringSubmatch(html); len(m) > 1 {
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
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足"}
	}

	if idMatch := reGazelleTorrentID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/torrents.php?torrentid=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if idMatch := reGazelleGroupID.FindStringSubmatch(html); len(idMatch) > 1 {
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
	if m := reGazelleErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	} else if m := reGazelleErrorP.FindStringSubmatch(html); len(m) > 1 {
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
		return false, fmt.Errorf("search for verify exists %q: %w", torrentID, err)
	}
	for _, r := range results {
		if r.TorrentID == torrentID {
			return true, nil
		}
	}
	return false, nil
}

func (a *GazelleAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	var stats *model.UserStatsResult
	var err error
	if config.APIKey != "" {
		stats, err = a.fetchUserStatsAPI(ctx, config)
	} else {
		stats, err = a.fetchUserStatsCookie(ctx, config)
	}
	if err != nil {
		return nil, err
	}
	if stats.BonusPoints == 0 {
		a.fetchFromUserPHP(ctx, config, stats)
	}
	a.fetchSeedingFromCommunityStats(ctx, config, stats)
	if stats.SeedingSize == 0 {
		a.fetchSeedingSizeFromBPRates(ctx, config, stats)
	}
	return stats, nil
}

func (a *GazelleAdapter) fetchUserStatsAPI(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/ajax.php?action=index", nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Authorization", config.APIKey)
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	return parseGazelleStatsAPI(body)
}

func (a *GazelleAdapter) fetchUserStatsCookie(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/ajax.php?action=index", nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	return parseGazelleStatsAPI(body)
}

func parseGazelleStatsAPI(body []byte) (*model.UserStatsResult, error) {
	var result struct {
		Status   string `json:"status"`
		Response struct {
			Username  string `json:"username"`
			ID        int64  `json:"id"`
			Userstats struct {
				Uploaded   interface{} `json:"uploaded"`
				Downloaded interface{} `json:"downloaded"`
				Ratio      float64     `json:"ratio"`
				Class      string      `json:"class"`
				BonusPoints interface{} `json:"bonusPoints"`
				SeedingCount int        `json:"seedingCount"`
				SeedingSize interface{} `json:"seedingSize"`
			} `json:"userstats"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("API 返回状态: %s", result.Status)
	}

	stats := &model.UserStatsResult{
		Username:  result.Response.Username,
		UserClass: result.Response.Userstats.Class,
		Ratio:     result.Response.Userstats.Ratio,
	}

	stats.UploadBytes = toInt64(result.Response.Userstats.Uploaded)
	stats.DownloadBytes = toInt64(result.Response.Userstats.Downloaded)

	if bp := toInt64(result.Response.Userstats.BonusPoints); bp > 0 {
		stats.BonusPoints = float64(bp)
	}
	if sc := result.Response.Userstats.SeedingCount; sc > 0 {
		stats.SeedingCount = sc
	}
	if ss := toInt64(result.Response.Userstats.SeedingSize); ss > 0 {
		stats.SeedingSize = ss
	}

	return stats, nil
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i
		}
		if f, err := val.Float64(); err == nil {
			return int64(f)
		}
	case string:
		cleaned := strings.ReplaceAll(val, ",", "")
		if i, err := strconv.ParseInt(cleaned, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(cleaned, 64); err == nil {
			return int64(f)
		}
	}
	return 0
}

func (a *GazelleAdapter) fetchFromUserPHP(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/user.php", nil)
	if err != nil {
		return
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := readBody(resp)
	if err != nil {
		return
	}

	html := string(body)
	if stats.BonusPoints == 0 {
		if m := reGazelleUserBonus.FindStringSubmatch(html); len(m) > 1 {
			bonusStr := strings.ReplaceAll(m[1], ",", "")
			if v, err := strconv.ParseFloat(bonusStr, 64); err == nil {
				stats.BonusPoints = v
			}
		}
	}
	if stats.SeedingCount == 0 {
		if m := reGazelleSeedingNum.FindStringSubmatch(html); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil {
				stats.SeedingCount = n
			}
		}
	}
}

func (a *GazelleAdapter) fetchSeedingFromCommunityStats(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	userID := a.getUserID(ctx, config)
	if userID == 0 {
		return
	}
	baseURL := resolveBase(config)
	u := fmt.Sprintf("%s/ajax.php?action=community_stats&userid=%d", baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return
	}
	if config.APIKey != "" {
		req.Header.Set("Authorization", config.APIKey)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return
	}

	var result struct {
		Status   string `json:"status"`
		Response struct {
			Seeding interface{} `json:"seeding"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return
	}
	if result.Status != "success" {
		return
	}
	if n := toInt64(result.Response.Seeding); n > 0 {
		stats.SeedingCount = int(n)
	}
}

func (a *GazelleAdapter) getUserID(ctx context.Context, config *model.SiteConfig) int64 {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/ajax.php?action=index", nil)
	if err != nil {
		return 0
	}
	if config.APIKey != "" {
		req.Header.Set("Authorization", config.APIKey)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return 0
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return 0
	}

	var result struct {
		Response struct {
			ID int64 `json:"id"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return 0
	}
	return result.Response.ID
}

var reGazelleBPRatesSize = regexp.MustCompile(`(?is)<thead>.*?大小.*?</thead>.*?<tbody>\s*<tr>\s*<td>\d+</td>\s*<td>([\d.,]+\s*(?:TB|GB|MB|TiB|GiB|MiB))</td>`)

func (a *GazelleAdapter) fetchSeedingSizeFromBPRates(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/bonus.php?action=bprates", nil)
	if err != nil {
		return
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return
	}

	html := string(body)
	if m := reGazelleBPRatesSize.FindStringSubmatch(html); len(m) > 1 {
		if sz := parseSizeString(m[1]); sz > 0 {
			stats.SeedingSize = sz
		}
	}
}
