package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (a *MTeamAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	if config.APIKey != "" {
		return a.uploadViaAPI(ctx, config, req)
	}
	return a.uploadViaWeb(ctx, config, req)
}

func (a *MTeamAdapter) uploadViaAPI(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadURL := baseURL + "/api/torrent/createOredit"

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
		fw.writeField("smallDescr", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.Anonymous {
		fw.writeField("anonymous", "true")
	} else {
		// §59.451: 馒头 anonymous 为必填 boolean（spec required）——false
		// 也必须显式写（缺字段 API 报必填错）。
		fw.writeField("anonymous", "false")
	}
	if req.IMDbLink != "" {
		fw.writeField("imdb", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		fw.writeField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}

	if v, ok := req.FormFields["category"]; ok {
		fw.writeField("category", v)
	}
	if v, ok := req.FormFields["source"]; ok {
		fw.writeField("source", v)
	}
	if v, ok := req.FormFields["medium"]; ok {
		fw.writeField("medium", v)
	}
	if v, ok := req.FormFields["standard"]; ok {
		fw.writeField("standard", v)
	}
	if v, ok := req.FormFields["videoCodec"]; ok {
		fw.writeField("videoCodec", v)
	}
	if v, ok := req.FormFields["audioCodec"]; ok {
		fw.writeField("audioCodec", v)
	}
	if v, ok := req.FormFields["team"]; ok {
		fw.writeField("team", v)
	}
	if v, ok := req.FormFields["processing"]; ok {
		fw.writeField("processing", v)
	}

	for k, v := range req.ExtraFields {
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
	a.setAPIHeaders(httpReq, config.APIKey)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "upload (API)")
	}

	var apiResp struct {
		Data struct {
			ID interface{} `json:"id"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err == nil {
		if apiResp.Data.ID != nil {
			torrentID := fmt.Sprintf("%v", apiResp.Data.ID)
			return &model.PublishResponse{
				Success:    true,
				TorrentID:  torrentID,
				DetailURL:  baseURL + "/detail/" + torrentID,
				TargetSite: config.Domain,
			}, nil
		}
		if apiResp.Message != "" {
			return &model.PublishResponse{Success: false, ErrorMessage: apiResp.Message}, nil
		}
	}

	html := string(body)
	if idMatch := reMTeamDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)}
}

func (a *MTeamAdapter) uploadViaWeb(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadPath := "/upload.php"
	if config.Paths.Upload != "" {
		uploadPath = config.Paths.Upload
	}
	uploadURL := baseURL + uploadPath

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
		fw.writeField("uplver", "1")
	}
	if req.DoubanLink != "" {
		fw.writeField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}
	for k, v := range req.FormFields {
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

	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "upload (web)")
	}

	if idMatch := reMTeamDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reMTeamErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *MTeamAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未找到 info_hash")
	}
	return detail.InfoHash, nil
}

func resolveBaseURL(config *model.SiteConfig) string {
	// §59.460: BaseURL 优先（可乐三面分离站 domain=www.colapt.com Web
	// 主机 / base_url=api.colapt.com API 兼容面——纯 Domain 构造把 API
	// 请求发到 www 主机：profile 打中 reelcore Web API 超时不响应）。
	// 馒头等 API 即主域站 base_url==domain 零变化；全库 base_url≠domain
	// 仅可乐一例。
	// §59.578 附·08 刀2 注释修正：本函数【补协议+去尾斜杠】——
	// HasPrefix("http") 补 https:// + TrimRight("/")。上方「纯 Domain
	// 构造」是修复前的问题叙述（历史语境）·非当前语义——语义与
	// siteBaseURL 标准三态一致（附录G §2.1/§2.3：判 helper 语义以
	// 代码本体+git log -L 为准·不据本段历史叙述）。§59.559 创建起
	// 即带补协议·此后零修改（git log -L 276,289 实证）。
	// baseURLOf 委托此函数（08 刀2·消费点端到端等价）。
	u := config.BaseURL
	if u == "" {
		u = config.Domain
	}
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return strings.TrimRight(u, "/")
}

func (a *MTeamAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
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

func (a *MTeamAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	if config.APIKey == "" {
		return a.fetchUserStatsHTML(ctx, config)
	}
	return a.fetchUserStatsAPI(ctx, config)
}

func (a *MTeamAdapter) fetchUserStatsAPI(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	// §59.460 附五（刀1）: profile 段走 mtorrentCore（json.Number 双兼容
	// 解析单点）；Role→等级名映射是馒头特有资产留宿主。
	prof, err := a.fetchProfile(ctx, config)
	if err != nil {
		return nil, err
	}
	stats := &model.UserStatsResult{
		Username:      prof.Username,
		UploadBytes:   prof.UploadBytes,
		DownloadBytes: prof.DownloadBytes,
		BonusPoints:   prof.Bonus,
		Ratio:         prof.Ratio,
	}
	if className := a.resolveRoleName(ctx, config, prof.Role); className != "" {
		stats.UserClass = className
	}

	peerURL := resolveBaseURL(config) + "/api/tracker/myPeerStatistics"
	peerReq, err := http.NewRequestWithContext(ctx, "POST", peerURL, nil)
	if err != nil {
		a.logger.Warn("failed to construct myPeerStatistics request", zap.Error(err))
		return stats, nil
	}
	a.setAPIHeaders(peerReq, config.APIKey)

	peerResp, err := a.doer.Client.Do(peerReq)
	if err != nil {
		a.logger.Warn("myPeerStatistics request failed", zap.Error(err))
		return stats, nil
	}
	defer func() { drainBody(peerResp) }()

	peerBody, err := readBody(peerResp)
	if err != nil {
		a.logger.Warn("failed to read myPeerStatistics response", zap.Error(err))
		return stats, nil
	}

	var peerResult struct {
		Data struct {
			SeederCount  flexInt `json:"seederCount"`
			SeederSize   flexInt `json:"seederSize"`
			LeecherCount flexInt `json:"leecherCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(peerBody, &peerResult); err != nil {
		a.logger.Warn("failed to parse myPeerStatistics response", zap.Error(err))
		return stats, nil
	}

	stats.SeedingCount = int(peerResult.Data.SeederCount)
	stats.SeedingSize = int64(peerResult.Data.SeederSize)
	return stats, nil
}

var mteamRoleMap = map[int]string{
	1: "小卒", 2: "捕頭", 3: "知縣", 4: "通判", 5: "知州",
	6: "府丞", 7: "府尹", 8: "總督", 9: "大臣", 10: "VIP",
	11: "職人", 12: "巡查", 13: "總版", 14: "總管",
	15: "維護開發員", 16: "站長", 17: "候選管理", 18: "波菜管理", 19: "小組長",
}

func (a *MTeamAdapter) resolveRoleName(ctx context.Context, config *model.SiteConfig, role int) string {
	if role <= 0 {
		return ""
	}
	if name, ok := mteamRoleMap[role]; ok {
		return name
	}
	roleURL := resolveBaseURL(config) + "/api/member/sysRoleList"
	roleReq, err := http.NewRequestWithContext(ctx, "POST", roleURL, nil)
	if err != nil {
		return ""
	}
	a.setAPIHeaders(roleReq, config.APIKey)
	roleResp, err := a.doer.Client.Do(roleReq)
	if err != nil {
		return ""
	}
	defer func() { drainBody(roleResp) }()
	roleBody, err := readBody(roleResp)
	if err != nil {
		return ""
	}
	var roleResult struct {
		Data []struct {
			ID      flexInt `json:"id"`
			NameChs string  `json:"nameChs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(roleBody, &roleResult); err != nil {
		return ""
	}
	for _, r := range roleResult.Data {
		if int(r.ID) == role {
			return r.NameChs
		}
	}
	return ""
}

func (a *MTeamAdapter) fetchUserStatsHTML(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	pageURL := config.Domain + "/index.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(htmlBytes)

	result := &model.UserStatsResult{}
	if m := reNexusUsername.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	} else if m := reNexusUsernameAlt.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	}
	if m := reNexusFontUploaded.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	}
	if m := reNexusFontDownloaded.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelDownload.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	}
	return result, nil
}

// buildIMDbURL §59.297: API 字段双形态——纯 ID 拼接 / 完整 URL 直用（防双拼）。
func buildIMDbURL(v string) string {
	if strings.HasPrefix(v, "http") {
		return v
	}
	return "https://www.imdb.com/title/" + v + "/"
}

// buildDoubanURL §59.297: 同族（Under Current 案 douban 双拼实证——API 返回完整 URL）。
func buildDoubanURL(v string) string {
	if strings.HasPrefix(v, "http") {
		return v
	}
	return "https://movie.douban.com/subject/" + v + "/"
}
