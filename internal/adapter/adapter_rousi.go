package adapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ranfish/pt-forward/internal/description"

	"github.com/ranfish/pt-forward/internal/httpclient"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type RousiAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewRousiAdapter(doer *HTTPDoer, logger *zap.Logger) *RousiAdapter {
	return &RousiAdapter{
		GenericAdapter: NewGenericAdapter("rousi", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *RousiAdapter) Framework() string { return "rousi" }

func (a *RousiAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	// §59.457: perPage 改版后被忽略（默认 100 全量）；page_size 实测生效
	u := resolveBaseURL(config) + "/api/v1/torrents?page_size=20&page=1"
	if keyword != "" {
		// §59.457 附: 兼容层关键词参数名=keyword（PeerGo 源码 handleMoviePilotTorrentList
		// 权威：r.URL.Query().Get("keyword")；旧 search= 被忽略返回最新全量——
		// e2e 假阳性=Bluey 恰为最新第 3 条；keyword=Bluey 实测 total=3 精确过滤）
		u += "&keyword=" + url.QueryEscape(keyword)
	}
	if opts != nil && opts.Category != "" {
		u += "&category=" + opts.Category
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, searchError("构造搜索请求失败", err)
	}
	a.setAuthHeaders(req, rousiCredential(config))

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, searchError("搜索请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			Torrents []struct {
				ID       int    `json:"id"`
				UUID     string `json:"uuid"`
				Title    string `json:"title"`
				Size     int64  `json:"size"`
				Seeders  int    `json:"seeders"`
				Leechers int    `json:"leechers"`
				Category string `json:"category"`
			} `json:"torrents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析搜索结果失败", err)
	}

	var results []*model.SeedingSearchResult
	for _, t := range result.Data.Torrents {
		tid := t.UUID
		if tid == "" {
			tid = fmt.Sprintf("%d", t.ID)
		}
		results = append(results, &model.SeedingSearchResult{
			TorrentID: tid,
			Title:     t.Title,
			Size:      t.Size,
			Seeders:   t.Seeders,
			Leechers:  t.Leechers,
		})
	}
	return results, nil
}

// rousiCredential §59.457 附五: 肉丝凭据=APIKey（pgk_）单源。Passkey
// 已拆除（用户定案 2026-11-03：passkey=tracker 专用——PeerGo 源码
// TrackerCredential "only to construct one authenticated announce URL"；
// .torrent 下载时站方自动注入下载者 passkey，PT-Forward 全链只走
// API 通道，fallback 旧 passkey 调新 API 恒 401 无意义）。
func rousiCredential(config *model.SiteConfig) string {
	return config.APIKey
}

func (a *RousiAdapter) setAuthHeaders(req *http.Request, passkey string) {
	req.Header.Set("Authorization", "Bearer "+passkey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://rousi.pro")
	req.Header.Set("Referer", "https://rousi.pro/")
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
}

func (a *RousiAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := resolveBaseURL(config) + "/api/torrent/" + torrentID + "/download/" + rousiCredential(config)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	a.setAuthHeaders(req, rousiCredential(config))

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
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

func (a *RousiAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/api/v1/torrents/" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	a.setAuthHeaders(req, rousiCredential(config))

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求详情失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			UUID      string `json:"uuid"`
			Title     string `json:"title"`
			Subtitle  string `json:"subtitle"`
			Size      int64  `json:"size"`
			InfoHash  string `json:"info_hash"`
			Category    string `json:"category"`
			Seeders     int    `json:"seeders"`
			Leechers    int    `json:"leechers"`
			MediaInfo   string `json:"media_info"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析响应失败", err)
	}

	d := result.Data
	detail := &model.TorrentDetail{
		Title: d.Title,
		// §59.457 附三: Subtitle/Description 首版丢弃（struct 解析了 Subtitle
		// 未赋值/Description 未解析）——获取链完整性缺口，meta 副标题/描述
		// 列恒空（副标题含译名/集数——Tab1 显示+发布链副标题投递双受影响）
		Subtitle:    d.Subtitle,
		Size:        d.Size,
		InfoHash:    strings.ToLower(d.InfoHash),
		Category:    d.Category,
		MediaInfo:   d.MediaInfo,
		Description: d.Description,
	}
	// 从结构化字段提取禁转标记（§56.37 合规修复）
	detail.Flags = extractFlagsFromStructured(detail.Title, detail.Subtitle, detail.Tags)
	return detail, nil
}

func (a *RousiAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/api/v1/torrents/" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	a.setAuthHeaders(req, rousiCredential(config))

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求优惠信息失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Promotion *struct {
				IsActive       bool    `json:"is_active"`
				Type           int     `json:"type"`
				DownMultiplier float64 `json:"down_multiplier"`
				UpMultiplier   float64 `json:"up_multiplier"`
			} `json:"promotion"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析优惠信息失败", err)
	}

	promo := result.Data.Promotion
	if promo == nil || !promo.IsActive {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}

	down := promo.DownMultiplier
	up := promo.UpMultiplier

	if down == 0 && up >= 2 {
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: up}, nil
	}
	if down == 0 {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if down == 0.5 && up >= 2 {
		return &model.DiscountResult{Level: model.Discount2x50}, nil
	}
	if down == 0.5 {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}
	if down == 0.3 {
		return &model.DiscountResult{Level: model.DiscountPercent30}, nil
	}
	if up >= 2 {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: up}, nil
	}
	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *RousiAdapter) DetectHR(_ context.Context, _ *model.SiteConfig, _ string) (*model.HRResult, error) {
	return &model.HRResult{HasHR: false}, nil
}

func (a *RousiAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	result := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		sl, err := a.GetPreciseSLData(ctx, config, id)
		if err != nil {
			a.logger.Warn("failed to get SL data", zap.String("torrentID", id), zap.Error(err))
			continue
		}
		result[id] = sl
	}
	return result, nil
}

func (a *RousiAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := resolveBaseURL(config) + "/api/v1/torrents/" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return &model.SLData{}, nil
	}
	a.setAuthHeaders(req, rousiCredential(config))

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.SLData{}, nil
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Seeders  int `json:"seeders"`
			Leechers int `json:"leechers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return &model.SLData{}, nil
	}

	return &model.SLData{Seeders: result.Data.Seeders, Leechers: result.Data.Leechers}, nil
}

func (a *RousiAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	// §59.458-⑤: description BBCode→Markdown 禁图变体（站方 4001 硬校验）
	desc := description.BBCodeToMarkdownNoImage(req.Description)

	payload := map[string]interface{}{
		"torrent":     base64.StdEncoding.EncodeToString(req.TorrentData),
		"title":       req.Title,
		"description": desc,
		// §59.458-⑥: 链接顶层 URL 双形态（站方 legacyExternalIdentifiers
		// URL/编号任一——URL 直投零新域；优先豆瓣=站方规则）
		"anonymous": req.Anonymous,
		"price":     0,
	}
	if req.Subtitle != "" {
		payload["subtitle"] = req.Subtitle
	}
	if req.IMDbLink != "" {
		payload["imdb"] = req.IMDbLink
	}
	if req.DoubanLink != "" {
		payload["douban"] = req.DoubanLink
	}
	if req.MediaInfo != "" {
		payload["media_info"] = req.MediaInfo
	}
	if v := resolveField(req.FormFields, "cat", "category"); v != "" {
		payload["category"] = v
	}

	// §59.458-①④: attributes 嵌套组装
	attributes := map[string]interface{}{}
	if v := req.FormFields["resolution"]; v != "" {
		attributes["resolution"] = v
	}
	// medium 双槽分发（④矩阵——form_config value_mappings 配矩阵值：
	// rt 集{remux,web-dl,hdtv,dvdrip}→release-type；其余→source）
	if v := req.FormFields["medium"]; v != "" {
		switch v {
		case "remux", "web-dl", "hdtv", "dvdrip":
			attributes["release-type"] = v
		default:
			attributes["source"] = v
		}
	}
	// genre/region（executor §59.458-②③ 注入 ExtraFields——终态值直投）
	if v := req.ExtraFields["rs_genre"]; v != "" {
		var genres []string
		if err := json.Unmarshal([]byte(v), &genres); err == nil && len(genres) > 0 {
			attributes["genre"] = genres
		}
	}
	if v := req.ExtraFields["rs_region"]; v != "" {
		attributes["region"] = v
	}
	if len(attributes) > 0 {
		payload["attributes"] = attributes
	}

	// §59.458 窄例外: images[0]=PTGen 封面（PosterURL 单图下载→5M 门→
	// base64；失败降级警告不阻塞）；images[1:]=本地内容截图（路径形态
	// 读文件 base64；URL 形态跳过——白名单禁图床下载）
	images := rousiBuildImages(ctx, req, a.logger)
	if len(images) > 0 {
		payload["images"] = images
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, networkError("序列化请求失败", err)
	}

	u := resolveBaseURL(config) + "/api/v1/torrents"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// §59.458-⑦: 幂等头随机 UUID（retry 层同 req 重试自动同 key——
	// 幂等头标准用法；infohash 去重最终防线）
	httpReq.Header.Set("Idempotency-Key", uuid.New().String())
	a.setAuthHeaders(httpReq, rousiCredential(config))

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	respBody, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ID       int64  `json:"id"`
			UUID     string `json:"uuid"`
			InfoHash string `json:"info_hash"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &model.AppError{Code: 15001, Message: "解析响应失败", Cause: err}
	}

	if result.Code != 0 {
		return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("API 错误(code=%d): %s", result.Code, result.Message)}
	}

	// §59.458-⑧: 201+pending/approved 均为提交成功（审核流站方行为）；
	// TorrentID=数字 id（§9.4 数字权威——uuid 仍在响应不消费）
	tid := strconv.FormatInt(result.Data.ID, 10)
	return &model.PublishResponse{
		Success:    true,
		TorrentID:  tid,
		DetailURL:  resolveBaseURL(config) + "/torrents/" + tid,
		TargetSite: config.Domain,
	}, nil
}

// rousiPosterClient §59.458: 封面下载独立 client（15s 超时+浏览器 UA——
// 不走站点 doer：目标站代理配置不适用于外站图源）。
var rousiPosterClient = &http.Client{Timeout: 15 * time.Second}

// rousiBuildImages §59.458: images 组装——封面（ExtraFields["rs_poster"]
// 单图窄例外下载；≤5M 站方门；失败警告降级不阻塞）+本地截图（路径形态
// 读文件 base64；URL 形态跳过——内容截图不开放下载）。
// rousiBuildImages §59.458/§59.459 附二: images 组装——封面+Tab3 截图
// 全程内存态现算即弃（用户终案：截图临时使用无需保存；mpv 临时文件发布
// 时已清，Tab3 持久态=图床 URL——现场下载回本地→base64→投递完即弃）。
// 数量门：封面+截图合计 ≤6（站方 MaxTorrentScreenshots——封面 1 优先
// 保+截图取前 5）；逐张失败降级跳过（部分投递优于零投递）。
func rousiBuildImages(ctx context.Context, req *model.PublishRequest, logger *zap.Logger) []string {
	const maxImages = 6
	// §59.462: 站方总计门 20MB（错误码 4006）——按最终 dataURL 编码后
	// 字节计（base64 膨胀 4/3 已含）；超限从最后一张逐张丢弃。数学安全：
	// 单张 5MB 门保证任何 3 张（1 封面+2 截图=站方最低要求）组合必然
	// ≤15MB < 20MB——降级永不破最低要求。
	const maxTotalBytes = 20 << 20
	images := make([]string, 0, maxImages)

	if poster := req.ExtraFields["rs_poster"]; poster != "" {
		if dataURL, ok := rousiDownloadImageAsDataURL(ctx, poster, logger); ok {
			images = append(images, dataURL)
		}
	}
	for _, shot := range req.Screenshots {
		if len(images) >= maxImages {
			break
		}
		shot = strings.TrimSpace(shot)
		// 仅 URL 形态（Tab3 图床链产物）现场下载；非 URL 形态跳过（防御）
		if !strings.HasPrefix(shot, "http://") && !strings.HasPrefix(shot, "https://") {
			continue
		}
		if dataURL, ok := rousiDownloadImageAsDataURL(ctx, shot, logger); ok {
			images = append(images, dataURL)
		}
	}
	// 总计门：超限从尾部（最后一张截图）逐张丢弃
	for totalLen(images) > maxTotalBytes && len(images) > 3 {
		if logger != nil {
			logger.Warn("rousi images total gate exceeded（丢弃最后一张）",
				zap.Int("images", len(images)-1),
				zap.Int("total_bytes", totalLen(images)))
		}
		images = images[:len(images)-1]
	}
	return images
}

func totalLen(ss []string) int {
	n := 0
	for _, s := range ss {
		n += len(s)
	}
	return n
}

// rousiDownloadImageAsDataURL §59.458 窄例外: 封面/截图单图下载→dataURL。
// 失败/超限返回 ok=false（调用方降级——发布不因外站可用性失败）。
// §59.462: 站方 API 明确要 JPEG data URL——png 输入（截图格式可选化后
// Tab3 可能是 png 产物）内存转码 jpeg（q85 与旧默认一致）；jpeg 直通
// 零开销（默认用户完全不经过转码路径）。
func rousiDownloadImageAsDataURL(ctx context.Context, url string, logger *zap.Logger) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	resp, err := rousiPosterClient.Do(req)
	if err != nil {
		if logger != nil {
			logger.Warn("rousi poster download failed（降级无封面）", zap.String("url", url), zap.Error(err))
		}
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		if logger != nil {
			logger.Warn("rousi poster download non-200（降级无封面）", zap.String("url", url), zap.Int("status", resp.StatusCode))
		}
		return "", false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil || len(data) == 0 || len(data) > 6<<20 {
		if logger != nil {
			logger.Warn("rousi poster size gate（降级无封面）", zap.String("url", url), zap.Int("bytes", len(data)))
		}
		return "", false
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" || mime == "application/octet-stream" {
		// Content-Type 缺失时魔数兜底（明确声明时信任服务器——避免对
		// 声明 jpeg 的 body 做过严魔数校验误杀直通）
		mime = sniffImageMIME(data)
	}
	// §59.462: png → jpeg 内存转码（站方 API 仅收 JPEG base64）——仅对
	// 判定为 png 的输入（声明 png 或嗅探 png）；jpeg 声明直通零开销
	if mime == "image/png" {
		if jpegData, jErr := pngToJPEG(data, 85); jErr == nil {
			data = jpegData
			mime = "image/jpeg"
		} else if logger != nil {
			logger.Warn("rousi png→jpeg transcode failed（降级跳过）", zap.String("url", url), zap.Error(jErr))
			return "", false
		}
	}
	if !strings.HasPrefix(mime, "image/") {
		mime = "image/jpeg"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), true
}

// sniffImageMIME 魔数嗅探（Content-Type 缺失时的兜底判定）。
func sniffImageMIME(data []byte) string {
	if len(data) > 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if len(data) > 3 && data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	return "image/jpeg"
}

// pngToJPEG §59.462: png bytes → jpeg 编码（肉丝站方 API 仅收 JPEG；
// 转码有损 q85 与旧 jpeg 默认一致——肉丝消费端画质回到 JPG 时代水平）。
func pngToJPEG(pngData []byte, quality int) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("png decode: %w", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("jpeg encode: %w", err)
	}
	return buf.Bytes(), nil
}

func (a *RousiAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail == nil || detail.InfoHash == "" {
		return "", notFoundError("未找到 info_hash")
	}
	return detail.InfoHash, nil
}

func resolveField(fields map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := fields[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

func (a *RousiAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
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

func (a *RousiAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	baseURL := resolveBaseURL(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/profile?include_fields[user]=seeding_leeching_data", nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	a.setAuthHeaders(req, rousiCredential(config))

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		respBody := string(body)
		if len(respBody) > 200 {
			respBody = respBody[:200]
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			ID                  int     `json:"id"`
			Username            string  `json:"username"`
			Level               int     `json:"level"`
			LevelText           string  `json:"level_text"`
			Uploaded            int64   `json:"uploaded"`
			Downloaded          int64   `json:"downloaded"`
			Ratio               float64 `json:"ratio"`
			Karma               float64 `json:"karma"`
			SeedingLeechingData struct {
				SeedingCount int   `json:"seeding_count"`
				SeedingSize  int64 `json:"seeding_size"`
			} `json:"seeding_leeching_data"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("API 错误: code=%d msg=%s", result.Code, result.Message)
	}

	stats := &model.UserStatsResult{
		Username:      result.Data.Username,
		UserClass:     result.Data.LevelText,
		UploadBytes:   result.Data.Uploaded,
		DownloadBytes: result.Data.Downloaded,
		Ratio:         result.Data.Ratio,
		BonusPoints:   result.Data.Karma,
		SeedingCount:  result.Data.SeedingLeechingData.SeedingCount,
		SeedingSize:   result.Data.SeedingLeechingData.SeedingSize,
	}

	return stats, nil
}
