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
	reGazelleUserBonus  = regexp.MustCompile(`(?i)积分:\s*([\d,]+)`)
	reGazelleSeedingNum = regexp.MustCompile(`(?i)当前做种[^<]*<[^>]*>(\d+)`)
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
	baseURL := resolveBaseURL(config)
	u := baseURL + "/torrents.php?action=download&id=" + url.QueryEscape(torrentID)
	if config.AuthKey != "" {
		u += "&authkey=" + url.QueryEscape(config.AuthKey)
	}
	if config.RSSKey != "" {
		u += "&torrent_pass=" + url.QueryEscape(config.RSSKey)
	} else if config.Passkey != "" {
		u += "&passkey=" + url.QueryEscape(config.Passkey)
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

func (a *GazelleAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	if config.APIKey != "" {
		return a.detailViaAPI(ctx, config, torrentID)
	}
	return a.detailViaWeb(ctx, config, torrentID)
}

func (a *GazelleAdapter) detailViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	baseURL := resolveBaseURL(config)
	u := baseURL + "/ajax.php?action=torrent&id=" + torrentID

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
	return parseGazelleTorrentDetail(body)
}

// parseGazelleTorrentDetail §59.462: sections/ajax/torrent.php 响应解析
// （2026-11-04 实测双层结构：group=影片级 name/subName/year/releaseType/
// categoryId/categoryName/imdbId/doubanId/conver[封面]/tags + torrent=
// 版本级 codec/resolution/container/processing/source/remasterTitle/
// scene/size/seeders/freeTorrent/mediainfos[数组]/fileName/description）。
func parseGazelleTorrentDetail(body []byte) (*model.TorrentDetail, error) {
	var result struct {
		Status   string `json:"status"`
		Response struct {
			Group struct {
				Name        string   `json:"name"`
				SubName     string   `json:"subName"`
				Year        int      `json:"year"`
				ReleaseType string   `json:"releaseType"`
				CategoryID  int      `json:"categoryId"`
				CategoryNm  string   `json:"categoryName"`
				ImdbID      string   `json:"imdbId"`
				DoubanID    string   `json:"doubanId"`
				Conver      string   `json:"conver"`
				Tags        []string `json:"tags"`
				Desc        string   `json:"description"`
			} `json:"group"`
			Torrent struct {
				InfoHash    string   `json:"infoHash"`
				Size        int64    `json:"size"`
				Seeders     int      `json:"seeders"`
				Leechers    int      `json:"leechers"`
				Snatched    int      `json:"snatched"`
				Codec       string   `json:"codec"`
				Resolution  string   `json:"resolution"`
				Container   string   `json:"container"`
				Processing  string   `json:"processing"`
				Source      string   `json:"source"`
				RemasterTtl string   `json:"remasterTitle"`
				Description string   `json:"description"`
				Mediainfos  []string `json:"mediainfos"`
				FileName    string   `json:"fileName"`
				FreeTorrent bool     `json:"freeTorrent"`
			} `json:"torrent"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, parseError("failed to parse API response", err)
	}
	if result.Status != "success" {
		return nil, parseError("API 返回状态: "+result.Status, nil)
	}

	g := result.Response.Group
	t := result.Response.Torrent
	detail := &model.TorrentDetail{
		// Gazelle 双层：Title=组名（影片名）——种子名在 fileName（不重组）
		Title:        g.Name,
		Subtitle:     g.SubName,
		Size:         t.Size,
		InfoHash:     strings.ToLower(t.InfoHash),
		Codec:        t.Codec,
		Resolution:   t.Resolution,
		Source:       t.Source,
		ReleaseGroup: t.Processing, // processing 非制作组——版本处理（Encode/Remux/碟规格），暂放此槽供发布反查
		Description:  t.Description,
		Tags:         g.Tags,
		PosterURL:    g.Conver,
		IMDbID:       g.ImdbID,
		DoubanID:     g.DoubanID,
	}
	if g.Year > 0 {
		detail.Subtitle = fmt.Sprintf("%s (%d)", detail.Subtitle, g.Year)
	}
	// mediainfos 数组（多文件逐条）→ 取首条（单文件种子常态）
	if len(t.Mediainfos) > 0 {
		detail.MediaInfo = t.Mediainfos[0]
	}
	if t.Description != "" {
		detail.Screenshots = extractScreenshotsFromBBCode(t.Description)
	}
	return detail, nil
}

func (a *GazelleAdapter) detailViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	baseURL := resolveBaseURL(config)
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

	if imdbURL := reIMDbURL.FindString(html); imdbURL != "" {
		detail.IMDbURL = imdbURL
	}
	if doubanURL := reDoubanURL.FindString(html); doubanURL != "" {
		detail.DoubanURL = doubanURL
	}
	detail.Flags = extractFlagsFromStructured(detail.Title, "", detail.Tags)

	return detail, nil
}

func (a *GazelleAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	baseURL := resolveBaseURL(config)
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
	baseURL := resolveBaseURL(config)
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
	baseURL := resolveBaseURL(config)
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
			a.logger.Warn("failed to get SL data", zap.String("torrentID", id), zap.Error(err))
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
	baseURL := resolveBaseURL(config)
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
	baseURL := resolveBaseURL(config)
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

	baseURL := resolveBaseURL(config)
	// §59.462: GPW 发布走 api.php?action=upload（API 通道——黄金规则
	// "须通过 API"+API Key 认证免 auth 字段）。Web upload.php 保留为
	// API Key 缺失时的 cookie 降级（旧实现字段名已重写，此路径暂弃——
	// GPW 配置要求 API Key）。
	if config.APIKey == "" {
		return nil, authError("海豹发布需要 API Key（站点设置 → API 区块获取）", nil)
	}
	uploadURL := baseURL + "/api.php?api_key=" + url.QueryEscape(config.APIKey) + "&action=upload"

	// 2a 查组：ExtraFields[gpw_groupid] 命中 → 挂组；miss → 建组
	groupID := req.ExtraFields["gpw_groupid"]

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

	// ===== 2b-1 影片层（isNewGroup 时必填）=====
	if groupID == "" {
		// name=影片名（英文优先）/subname=中文/year/releasetype/tags/imdb
		fw.writeField("name", firstNonEmpty(req.ExtraFields["gpw_name"], req.Title))
		fw.writeField("subname", req.ExtraFields["gpw_subname"])
		fw.writeField("year", req.ExtraFields["gpw_year"])
		fw.writeField("releasetype", firstNonEmpty(req.ExtraFields["gpw_releasetype"], "1")) // 默认长片
		if tags := req.ExtraFields["gpw_tags"]; tags != "" {
			fw.writeField("tags", tags)
		}
		fw.writeField("imdb", firstNonEmpty(req.IMDbLink, req.ExtraFields["gpw_imdb"]))
		// image=封面（白名单域 URL——executor Rehost 转投后）
		if img := req.ExtraFields["gpw_image"]; img != "" {
			fw.writeField("image", img)
		}
		// desc=组描述（PTGen 影片简介）
		if d := req.ExtraFields["gpw_desc"]; d != "" {
			fw.writeField("desc", d)
		}

		// ===== 2c 剧组四件套（平行数组——至少 1 导演）=====
		for _, art := range req.ExtraArtists {
			fw.writeField("artists[]", art.Name)
			fw.writeField("importance[]", art.Role)
			fw.writeField("artist_ids[]", art.ID)
			fw.writeField("artists_sub[]", art.SubName)
		}
	} else {
		fw.writeField("groupid", groupID)
	}

	// ===== 2b-2 版本层（表单裸名——§59.462 源码论证）=====
	// source/codec/resolution/container/processing：经 executor 翻译层
	// 填入 FormFields（标准键→GPW 字符串值）
	if v := req.FormFields["source"]; v != "" {
		fw.writeField("source", v)
	}
	if v := req.FormFields["codec"]; v != "" {
		fw.writeField("codec", v)
	}
	if v := req.FormFields["resolution"]; v != "" {
		fw.writeField("resolution", v)
	}
	if v := req.FormFields["container"]; v != "" {
		fw.writeField("container", v)
	}
	if v := req.FormFields["processing"]; v != "" {
		fw.writeField("processing", v)
	}
	// processing_other（碟规格——原盘时 processing=Untouched+规格双投）
	if v := req.FormFields["processing_other"]; v != "" {
		fw.writeField("processing_other", v)
	}

	// release_desc（种子描述——必填）
	if req.Description != "" {
		fw.writeField("release_desc", req.Description)
	}

	// mediainfo[]（数组——单文件种子一条）
	if req.MediaInfo != "" {
		fw.writeField("mediainfo[]", req.MediaInfo)
	}

	// subtitle_type（1=混合/2=硬字/3=无字——必填 radio）
	fw.writeField("subtitle_type", firstNonEmpty(req.ExtraFields["gpw_subtitle_type"], "3"))

	// subtitles[]（语言码数组——chinese_simplified 等）
	for _, sub := range req.SubtitleLangs {
		if sub != "" {
			fw.writeField("subtitles[]", sub)
		}
	}

	// remaster_title（版本标签英文键——EditionInfo 映射后）
	if v := req.ExtraFields["gpw_remaster_title"]; v != "" {
		fw.writeField("remaster_title", v)
	}

	// ===== 特性 checkbox 族（§59.462——mergeAdvanceFeature 服务端合并）=====
	for _, cb := range []string{"10_bit", "hdr10", "hdr10plus", "dolby_vision", "dolby_atmos", "dts_x", "chinese_dubbed", "special_effects_subtitles"} {
		if req.ExtraFields["gpw_"+cb] == "1" {
			fw.writeField(cb, "1")
		}
	}

	// anonymous（匿名发布——checkbox on/off）
	if req.Anonymous {
		fw.writeField("anonymous", "1")
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
	httpReq.Header.Set("Accept", "application/json")
	setCommonHeaders(httpReq, "") // API 通道——免 cookie（api_key 认证）

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	// 响应 JSON：{"status":200,"response":{"torrent_id":N}} 或错误
	var result struct {
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Response struct {
			TorrentID interface{} `json:"torrent_id"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, parseError("解析上传响应失败: "+string(body[:min(200, len(body))]), err)
	}
	if result.Status == 200 {
		tid := toStr(result.Response.TorrentID)
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  tid,
			DetailURL:  baseURL + "/torrents.php?torrentid=" + tid,
			TargetSite: config.Domain,
		}, nil
	}
	errMsg := firstNonEmpty(result.Error, result.Message, fmt.Sprintf("HTTP %d", resp.StatusCode))
	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

// firstNonEmpty 首个非空串。
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func min(a, b int) int { if a < b { return a }; return b }

// SearchTorrents §59.462: browse AJAX 搜索（sections/ajax/browse.php——
// 黄金规则"须通过 API"禁页面抓取，GenericAdapter 的 HTML 解析违规）。
// 响应按组聚合（一组一条+torrents 数组）——展平为种子级结果（组内每颗
// 一条，tid=种子 id）。searchstr 全文（组名/imdb/年份联合索引）。
func (a *GazelleAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, _ *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	baseURL := resolveBaseURL(config)
	u := baseURL + "/ajax.php?action=browse&searchstr=" + url.QueryEscape(keyword)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, searchError("搜索请求失败", err)
	}
	defer func() { drainBody(resp) }()
	body, readErr := readBody(resp)
	if readErr != nil {
		return nil, readErr
	}

	var result struct {
		Status   string `json:"status"`
		Response struct {
			Results []struct {
				GroupID   int    `json:"groupId"`
				GroupName string `json:"groupName"`
				GroupYear int    `json:"groupYear"`
				Torrents  []struct {
					TorrentID interface{} `json:"torrentId"`
					Size      interface{} `json:"size"`
					Seeders   int         `json:"seeders"`
					Leechers  int         `json:"leechers"`
				} `json:"torrents"`
			} `json:"results"`
		} `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, parseError("解析搜索结果失败", err)
	}
	if result.Status != "success" {
		return nil, parseError("搜索 API 状态: "+result.Status, nil)
	}

	var all []*model.SeedingSearchResult
	for _, g := range result.Response.Results {
		for _, t := range g.Torrents {
			all = append(all, &model.SeedingSearchResult{
				TorrentID: toStr(t.TorrentID),
				Title:     fmt.Sprintf("%s (%d)", g.GroupName, g.GroupYear),
				Size:      toInt64(t.Size),
				Seeders:   t.Seeders,
				Leechers:  t.Leechers,
			})
		}
	}
	return all, nil
}

// toStr interface{} → string（browse 响应 id 形态容错——数字或字符串）。
func toStr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	}
	return ""
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
	if config.AuthKey == "" || config.RSSKey == "" {
		a.scrapeGazelleAuthKeys(ctx, config, stats)
	}
	return stats, nil
}

func (a *GazelleAdapter) fetchUserStatsAPI(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	baseURL := resolveBaseURL(config)
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
	baseURL := resolveBaseURL(config)
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
				Uploaded     interface{} `json:"uploaded"`
				Downloaded   interface{} `json:"downloaded"`
				Ratio        float64     `json:"ratio"`
				Class        string      `json:"class"`
				BonusPoints  interface{} `json:"bonusPoints"`
				SeedingCount int         `json:"seedingCount"`
				SeedingSize  interface{} `json:"seedingSize"`
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
	baseURL := resolveBaseURL(config)
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
	baseURL := resolveBaseURL(config)
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
	baseURL := resolveBaseURL(config)
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
	baseURL := resolveBaseURL(config)
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

func (a *GazelleAdapter) scrapeGazelleAuthKeys(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	baseURL := resolveBaseURL(config)
	pageURL := baseURL + "/torrents.php?page=1"

	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
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

	reAuthkey := regexp.MustCompile(`authkey=([a-f0-9]{32})`)
	reTorrentPass := regexp.MustCompile(`torrent_pass=([a-f0-9]{32})`)

	if m := reAuthkey.FindStringSubmatch(html); len(m) >= 2 {
		stats.AuthKey = m[1]
	}
	if m := reTorrentPass.FindStringSubmatch(html); len(m) >= 2 {
		stats.RSSKey = m[1]
	}
}
