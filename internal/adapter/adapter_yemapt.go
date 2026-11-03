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
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type YemaptAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewYemaptAdapter(doer *HTTPDoer, logger *zap.Logger) *YemaptAdapter {
	return &YemaptAdapter{
		GenericAdapter: NewGenericAdapter("yemapt", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *YemaptAdapter) Framework() string { return "yemapt" }

func (a *YemaptAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	baseURL := resolveBaseURL(config)

	dlKey, err := a.generateDownloadKey(ctx, config, baseURL, torrentID)
	if err != nil {
		return nil, downloadError("生成下载密钥失败", err)
	}

	dlURL := baseURL + "/api/torrent/download1?token=" + url.QueryEscape(dlKey)
	req, err := http.NewRequestWithContext(ctx, "GET", dlURL, nil)
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

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/json") {
		body, _ := readBody(resp)
		return nil, &model.AppError{Code: 15001, Message: fmtES("返回了非种子文件: %s", string(body[:min(len(body), 200)]))}
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

func (a *YemaptAdapter) generateDownloadKey(ctx context.Context, config *model.SiteConfig, baseURL, torrentID string) (string, error) {
	// §野马OpenAPI: 优先用 POST /openApi/torrent/generateDownloadKey.json（APIKey 认证）
	if config.APIKey != "" {
		u := baseURL + "/openApi/torrent/generateDownloadKey.json?id=" + url.QueryEscape(torrentID)
		req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", config.APIKey)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			a.logger.Debug("openApi generateDownloadKey failed, trying web api", zap.Error(err))
		} else {
			defer func() { drainBody(resp) }()
			body, _ := readBody(resp)
			var result struct {
				Success      bool        `json:"success"`
				ErrorMessage string      `json:"errorMessage"`
				ErrorCode    int         `json:"errorCode"`
				Data         interface{} `json:"data"`
			}
			if json.Unmarshal(body, &result) == nil && result.Success {
				if s, ok := result.Data.(string); ok {
					return s, nil
				}
			}
			a.logger.Debug("openApi generateDownloadKey failed, trying web api",
				zap.String("body", string(body)))
		}
	}

	// fallback: GET /api/torrent/generateDownloadKey（Cookie 认证）
	u := baseURL + "/api/torrent/generateDownloadKey?id=" + url.QueryEscape(torrentID)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}

	var result struct {
		Success      bool        `json:"success"`
		ErrorMessage string      `json:"errorMessage"`
		ErrorCode    int         `json:"errorCode"`
		Data         interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("API error %d: %s", result.ErrorCode, result.ErrorMessage)
	}

	switch v := result.Data.(type) {
	case string:
		return v, nil
	case map[string]interface{}:
		if key, ok := v["key"].(string); ok {
			return key, nil
		}
		if token, ok := v["token"].(string); ok {
			return token, nil
		}
	}
	return "", fmt.Errorf("unexpected download key format: %T", result.Data)
}

func (a *YemaptAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	stats := &model.UserStatsResult{}

	if config.APIKey != "" {
		if err := a.fetchUserStatsOpenAPI(ctx, config, stats); err != nil {
			a.logger.Debug("openApi stats failed, trying web api", zap.Error(err))
		} else {
			a.enrichSeedingInfo(ctx, config, stats)
			return stats, nil
		}
	}

	if config.Cookie != "" {
		if err := a.fetchUserStatsWebAPI(ctx, config, stats); err != nil {
			return nil, err
		}
		a.enrichSeedingInfo(ctx, config, stats)
		return stats, nil
	}

	return nil, configError("野马需要 API Key 或 Cookie")
}

func (a *YemaptAdapter) fetchUserStatsOpenAPI(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) error {
	baseURL := resolveBaseURL(config)
	u := baseURL + "/openApi/user/fetchBasicInfo.json"

	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", config.APIKey)
	setCommonHeaders(req, "")

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID                    int    `json:"id"`
			Name                  string `json:"name"`
			Level                 int    `json:"level"`
			Bonus                 int64  `json:"bonus"`
			PromotionUploadSize   int64  `json:"promotionUploadSize"`
			PromotionDownloadSize int64  `json:"promotionDownloadSize"`
			Status                string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("API 返回失败")
	}

	stats.Username = result.Data.Name
	stats.UploadBytes = result.Data.PromotionUploadSize
	stats.DownloadBytes = result.Data.PromotionDownloadSize
	stats.BonusPoints = float64(result.Data.Bonus)
	if result.Data.PromotionDownloadSize > 0 {
		stats.Ratio = float64(result.Data.PromotionUploadSize) / float64(result.Data.PromotionDownloadSize)
	}

	levelNames := map[int]string{
		0: "乱民", 1: "小卒", 2: "教谕", 3: "登仕郎", 4: "修职郎",
		5: "文林郎", 6: "忠武校尉", 7: "承信将军", 8: "武毅将军",
		9: "武节将军", 10: "显威将军", 11: "宣武将军",
	}
	if name, ok := levelNames[result.Data.Level]; ok {
		stats.UserClass = name
	}

	return nil
}

func (a *YemaptAdapter) fetchUserStatsWebAPI(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) error {
	baseURL := resolveBaseURL(config)
	u := baseURL + "/api/consumer/fetchSelfDetail"

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID                    int    `json:"id"`
			Name                  string `json:"name"`
			Level                 int    `json:"level"`
			Bonus                 int64  `json:"bonus"`
			UploadSize            int64  `json:"uploadSize"`
			DownloadSize          int64  `json:"downloadSize"`
			PromotionUploadSize   int64  `json:"promotionUploadSize"`
			PromotionDownloadSize int64  `json:"promotionDownloadSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("API 返回失败")
	}

	stats.Username = result.Data.Name
	stats.UploadBytes = result.Data.PromotionUploadSize
	stats.DownloadBytes = result.Data.PromotionDownloadSize
	stats.BonusPoints = float64(result.Data.Bonus)
	if result.Data.PromotionDownloadSize > 0 {
		stats.Ratio = float64(result.Data.PromotionUploadSize) / float64(result.Data.PromotionDownloadSize)
	}

	levelNames := map[int]string{
		0: "乱民", 1: "小卒", 2: "教谕", 3: "登仕郎", 4: "修职郎",
		5: "文林郎", 6: "忠武校尉", 7: "承信将军", 8: "武毅将军",
		9: "武节将军", 10: "显威将军", 11: "宣武将军",
	}
	if name, ok := levelNames[result.Data.Level]; ok {
		stats.UserClass = name
	}

	return nil
}

func (a *YemaptAdapter) enrichSeedingInfo(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	if config.Cookie == "" {
		return
	}

	baseURL := resolveBaseURL(config)
	u := baseURL + "/api/userTorrent/fetchSeedTorrentInfo"

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("{}"))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
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

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Num      int   `json:"num"`
			FileSize int64 `json:"fileSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}
	if result.Success {
		stats.SeedingCount = result.Data.Num
		stats.SeedingSize = result.Data.FileSize
	}
}

func (a *YemaptAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	return a.GenericAdapter.GetTorrentDetail(ctx, config, torrentID)
}

func (a *YemaptAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	return a.GenericAdapter.DetectDiscount(ctx, config, torrentID)
}

func (a *YemaptAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	return a.GenericAdapter.DetectHR(ctx, config, torrentID)
}

func (a *YemaptAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	return a.GenericAdapter.DetectHRAndDiscount(ctx, config, torrentID)
}

func (a *YemaptAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	return a.GenericAdapter.GetBatchSLData(ctx, config, torrentIDs)
}

func (a *YemaptAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	return a.GenericAdapter.GetPreciseSLData(ctx, config, torrentID)
}

func (a *YemaptAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	return a.GenericAdapter.GetTorrentInfoHash(ctx, config, torrentID)
}

func (a *YemaptAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	return a.uploadViaOpenAPI(ctx, config, req)
}

// uploadViaOpenAPI §59.459-①: 野马上传——POST /openApi/torrent/
// uploadTorrent.json multipart（wiki 2026-08-13 权威）。认证=裸 AuthKey
// （Authorization 无 Bearer）。level<6 候选制：success=提交成功（data=
// 种子ID；是否直发由站方权限决定——审核是站方行为）。
func (a *YemaptAdapter) uploadViaOpenAPI(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

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

	// 必填五件：file ✓/showName/shortDesc/categoryId/uploadUserAnonymous
	fw.writeField("showName", req.Title)
	fw.writeField("shortDesc", yemaShortDesc(req))
	if v := req.FormFields["categoryId"]; v != "" {
		fw.writeField("categoryId", v)
	}
	// §59.422 匿名默认开启——野马 y/n 形态
	if req.Anonymous {
		fw.writeField("uploadUserAnonymous", "y")
	} else {
		fw.writeField("uploadUserAnonymous", "n")
	}

	// ⑤ 描述：BBCode→Markdown 禁图变体（野马同 Markdown+禁 data:image）
	if req.Description != "" {
		fw.writeField("longDesc", description.BBCodeToMarkdownNoImage(req.Description))
	}
	// ⑥ 编号：imdb 去 tt 前缀纯数字/douban 数字（从 URL 提取尾部编号）
	if id := extractNumericID(req.IMDbLink, "tt"); id != "" {
		fw.writeField("imdb", id)
	}
	if id := extractDoubanID(req.DoubanLink); id != "" {
		fw.writeField("douban", id)
	}
	// ④ 封面 URL 直投（零网络透传）
	if poster := req.ExtraFields["yema_poster"]; poster != "" {
		fw.writeField("picture", poster)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediaInfo", req.MediaInfo)
	}
	// 质量槽（FormFields 数字值——executor 通用链产出）
	for domain, field := range map[string]string{
		"medium": "medium", "standard": "standard", "codec": "codec",
		"audiocodec": "audiocodec", "team": "team",
	} {
		if v := req.FormFields[domain]; v != "" {
			fw.writeField(field, v)
		}
	}
	// ②③ regionList 数组（executor 特化注入——同名重复字段 ≤3）
	if v := req.ExtraFields["yema_regions"]; v != "" {
		var regions []string
		if json.Unmarshal([]byte(v), &regions) == nil {
			for _, r := range regions {
				fw.writeField("regionList", r)
			}
		}
	}
	// tags（FormFields["tags"]=逗号分隔数字值）
	if v := req.FormFields["tags"]; v != "" {
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" {
				fw.writeField("tagList", t)
			}
		}
	}
	// ④ 截图 URL 透传（≤20）
	for _, shot := range req.Screenshots {
		if shot = strings.TrimSpace(shot); shot != "" {
			fw.writeField("screenshotList", shot)
		}
	}
	// HR 考察默认关（转种不开）
	fw.writeField("hrPunishEnable", "false")

	if err := writer.Close(); err != nil {
		return nil, networkError("封装表单失败", err)
	}

	uploadURL := resolveBaseURL(config) + "/openApi/torrent/uploadTorrent.json"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	// 裸 AuthKey（无 Bearer——wiki 明确）
	httpReq.Header.Set("Authorization", config.APIKey)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Success      bool   `json:"success"`
		ErrorCode    int    `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
		Data         int64  `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &model.AppError{Code: 15001, Message: "解析响应失败", Cause: err}
	}
	if !result.Success {
		return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("上传失败(code=%d): %s", result.ErrorCode, result.ErrorMessage)}
	}
	// 候选制：success=提交成功（level<6 进审核流——data=种子ID 不变）
	tid := strconv.FormatInt(result.Data, 10)
	return &model.PublishResponse{
		Success:    true,
		TorrentID:  tid,
		DetailURL:  resolveBaseURL(config) + "/torrent/detail/" + tid,
		TargetSite: config.Domain,
	}, nil
}

// yemaShortDesc §59.459-⑧: 副标题必填兜底链——副标题→PTGen 中文译名→
// 标题（站方 ≤512 无换行）。
func yemaShortDesc(req *model.PublishRequest) string {
	for _, cand := range []string{req.Subtitle, req.ExtraFields["yema_translated"], req.Title} {
		cand = strings.TrimSpace(strings.ReplaceAll(cand, "\n", " "))
		if cand != "" {
			if len([]rune(cand)) > 512 {
				cand = string([]rune(cand)[:512])
			}
			return cand
		}
	}
	return req.Title
}

// extractNumericID 从 IMDb URL/纯编号提取去 tt 前缀数字（野马要纯数字）。
func extractNumericID(link, prefix string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	// 纯编号形态（tt0111161 / 0111161）直出
	if strings.HasPrefix(link, prefix) && len(link) > len(prefix) && isAllDigits(link[len(prefix):]) {
		return link[len(prefix):]
	}
	if isAllDigits(link) {
		return link
	}
	// URL 形态取尾段编号（剥尾斜杠再取末段）
	link = strings.TrimRight(link, "/")
	if i := strings.LastIndex(link, "/"); i >= 0 {
		tail := strings.TrimPrefix(link[i+1:], prefix)
		if isAllDigits(tail) {
			return tail
		}
	}
	return ""
}

// extractDoubanID 豆瓣 URL/纯数字提取（subject/ 尾数字）。
func extractDoubanID(link string) string {
	return extractNumericID(link, "")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (a *YemaptAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	return a.GenericAdapter.VerifyExists(ctx, config, torrentID)
}

func (a *YemaptAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	baseURL := resolveBaseURL(config)

	searchBody := fmt.Sprintf(`{"keyword":%q,"pageParam":{"current":1,"pageSize":20},"sorter":{}}`, keyword)

	// §野马OpenAPI: 优先用 /openApi/ 端点（APIKey），fallback /api/ 端点（Cookie）
	var u string
	if config.APIKey != "" {
		u = baseURL + "/openApi/torrent/fetchOpenTorrentList.json"
	} else {
		u = baseURL + "/api/torrent/fetchOpenTorrentList"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(searchBody))
	if err != nil {
		return nil, searchError("构造搜索请求失败", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", config.APIKey)
	} else {
		setCommonHeaders(req, config.Cookie)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, searchError("搜索请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 { snippet = snippet[:200] }
		return nil, httpError(fmtES("HTTP %d: %s", resp.StatusCode, snippet), nil)
	}

	a.logger.Info("yemapt search response",
		zap.String("url", u),
		zap.Int("status", resp.StatusCode),
		zap.Int("body_len", len(body)),
		zap.Int("cookie_len", len(config.Cookie)),
		zap.String("body", string(body[:min(500, len(body))])))

	var apiResp struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, searchError("解析搜索结果失败", err)
	}
	if !apiResp.Success {
		return nil, searchError("API 返回失败", nil)
	}

	var results []*model.SeedingSearchResult

	var dataMap map[string]json.RawMessage
	if json.Unmarshal(apiResp.Data, &dataMap) == nil {
		for _, key := range []string{"list", "torrents", "rows", "records"} {
			if raw, ok := dataMap[key]; ok {
				results = parseYeMPTList(raw)
				if len(results) > 0 { break }
			}
		}
	}
	if len(results) == 0 {
		results = parseYeMPTList(apiResp.Data)
	}

	return results, nil
}

func parseYeMPTList(raw json.RawMessage) []*model.SeedingSearchResult {
	var items []map[string]interface{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	var results []*model.SeedingSearchResult
	for _, item := range items {
		r := &model.SeedingSearchResult{}
		r.TorrentID = toString(item["id"])
		r.Title = toString(item["showName"])
		if r.Title == "" {
			r.Title = toString(item["title"])
		}
		if r.Title == "" {
			r.Title = toString(item["name"])
		}
		r.Size = toInt64(item["fileSize"])
		if r.Size == 0 {
			r.Size = toInt64(item["size"])
		}
		r.Seeders = toInt(item["seeders"])
		if r.TorrentID != "" {
			results = append(results, r)
		}
	}
	return results
}

// SupportsSearchByPiecesHash §野马OpenAPI: 野马提供 pieces_hash 查询（/openApi/torrent/fetchTorrentIdWithPiecesHash.json）
func (a *YemaptAdapter) SupportsSearchByPiecesHash() bool { return true }

// SearchByPiecesHash 通过 pieces_hash 批量查询种子 ID（野马 OpenAPI）
func (a *YemaptAdapter) SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error) {
	if len(piecesHashes) == 0 {
		return map[string]int{}, nil
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("野马 pieces_hash 查询需要 APIKey")
	}

	baseURL := resolveBaseURL(config)
	u := baseURL + "/openApi/torrent/fetchTorrentIdWithPiecesHash.json"

	// 最多 100 个/次
	result := make(map[string]int)
	for i := 0; i < len(piecesHashes); i += 100 {
		end := i + 100
		if end > len(piecesHashes) {
			end = len(piecesHashes)
		}
		batch := piecesHashes[i:end]

		body, _ := json.Marshal(map[string][]string{"piecesHashList": batch})
		req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(string(body)))
		if err != nil {
			return nil, fmt.Errorf("构造请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", config.APIKey)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("pieces_hash 请求失败: %w", err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		var result_struct struct {
			Success      bool              `json:"success"`
			ErrorMessage string            `json:"errorMessage"`
			Data         map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &result_struct); err != nil {
			return nil, fmt.Errorf("pieces_hash 响应解析失败: %w", err)
		}
		if !result_struct.Success {
			return nil, fmt.Errorf("pieces_hash 查询失败: %s", result_struct.ErrorMessage)
		}

		for hash, v := range result_struct.Data {
			id := 0
			switch n := v.(type) {
			case float64:
				id = int(n)
			case string:
				fmt.Sscanf(n, "%d", &id) //nolint:gosec // 解析失败 → id 零值走 400
			}
			if id > 0 {
				result[hash] = id
			}
		}
	}
	return result, nil
}
