package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

func (a *GenericAdapter) uploadTTG(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := resolveBaseURL(config)
	uploadURL := baseURL + "/upload.php"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
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

	if req.Title != "" {
		fw.writeField("name", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("subtitle", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		fw.writeField("imdb_c", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		m := reGenericDigits.FindString(req.DoubanLink)
		if m != "" {
			fw.writeField("douban_id", m)
		}
	}
	if req.Anonymous {
		fw.writeField("anonymity", "yes")
	}

	if v, ok := req.FormFields["cat"]; ok {
		fw.writeField("type", v)
	}
	if v, ok := req.FormFields["tags"]; ok {
		if strings.Contains(v, "禁转") {
			fw.writeField("nodistr", "yes")
		}
	}

	fw.writeField("MAX_FILE_SIZE", "4000000")
	fw.writeField("team", "")
	fw.writeField("hr", "no")

	for k, v := range req.FormFields {
		switch k {
		case "cat", "tags":
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

	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "upload")
	}

	if idMatch := reGenericTTGID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/t/" + torrentID + "/",
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "uploaded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reGenericErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *GenericAdapter) uploadStarSpace(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	isMusic := false
	if cat, ok := req.FormFields["cat"]; ok {
		if cat == "music" {
			isMusic = true
		}
	}
	if musicFlag, ok := req.ExtraFields["is_music"]; ok && musicFlag == "true" {
		isMusic = true
	}

	if isMusic {
		return a.uploadStarSpaceMusic(ctx, config, req)
	}
	return a.uploadStarSpaceVideo(ctx, config, req)
}

func (a *GenericAdapter) uploadStarSpaceVideo(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadURL := baseURL + "/p_torrent/video_upload.php"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
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

	fw.writeField("tid", "0")

	if req.Title != "" {
		fw.writeField("name", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("small_desc", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.DoubanLink != "" {
		fw.writeField("douban_url", req.DoubanLink)
	}
	if req.IMDbLink != "" {
		fw.writeField("imdb_url", req.IMDbLink)
	}
	if len(req.Screenshots) > 0 {
		fw.writeField("screen", strings.Join(req.Screenshots, "\n"))
	}

	fireflyFieldMap := map[string]string{
		"cat":            "tr_category",
		"medium_sel":     "tr_source",
		"codec_sel":      "tr_video_codec",
		"audiocodec_sel": "tr_audio_codec",
		"standard_sel":   "tr_resolution",
		"team_sel":       "tr_team",
	}
	for srcType, targetName := range fireflyFieldMap {
		if v, ok := req.FormFields[srcType]; ok {
			fw.writeField(targetName, v)
		}
	}

	if hdr, ok := req.ExtraFields["tr_hdr"]; ok {
		fw.writeField("tr_hdr", hdr)
	}

	if tags, ok := req.FormFields["tags"]; ok {
		tagMap := map[string]string{
			"官方": "tag_gf", "驻站组": "tag_xiaozu", "禁转": "tag_jz",
			"3D": "tag_3d", "中字": "tag_chs_sub", "国语": "tag_chs_lang",
			"粤语": "tag_yueyu", "英字": "tag_eng_sub", "英语": "tag_eng_lang",
			"分集": "tag_ep", "完结": "tag_complete",
		}
		for tagLabel, tagName := range tagMap {
			if strings.Contains(tags, tagLabel) {
				fw.writeField(tagName, "on")
			}
		}
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "audiocodec_sel", "standard_sel", "team_sel", "tags":
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

	return a.doStarSpaceUpload(httpReq, config)
}

func (a *GenericAdapter) uploadStarSpaceMusic(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadURL := baseURL + "/p_music/music_upload.php"

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

	fw.writeField("tid", "0")

	if v, ok := req.ExtraFields["artist"]; ok {
		fw.writeField("artist", v)
	}
	if req.Title != "" {
		fw.writeField("title", req.Title)
	}
	if v, ok := req.ExtraFields["year"]; ok {
		fw.writeField("year", v)
	}
	if v, ok := req.ExtraFields["image"]; ok {
		fw.writeField("image", v)
	}
	if req.Description != "" {
		fw.writeField("release_desc", req.Description)
	}

	if v, ok := req.FormFields["cat"]; ok {
		fw.writeField("release_type", v)
	}
	if v := resolveField(req.FormFields, "category", "cat"); v != "" {
		fw.writeField("release_type", v)
	}
	if v := resolveField(req.FormFields, "codec_sel", "codec"); v != "" {
		fw.writeField("format", v)
	}
	if v := resolveField(req.FormFields, "audiocodec_sel", "audioCodec"); v != "" {
		fw.writeField("bitrate", v)
	}
	if v := resolveField(req.FormFields, "medium_sel", "medium"); v != "" {
		fw.writeField("media", v)
	}

	if v, ok := req.ExtraFields["remaster_year"]; ok {
		fw.writeField("remaster_year", v)
	}
	if v, ok := req.ExtraFields["remaster_title"]; ok {
		fw.writeField("remaster_title", v)
	}
	if v, ok := req.ExtraFields["remaster_record_label"]; ok {
		fw.writeField("remaster_record_label", v)
	}
	if v, ok := req.ExtraFields["remaster_catalogue_number"]; ok {
		fw.writeField("remaster_catalogue_number", v)
	}

	for k, v := range req.ExtraFields {
		switch k {
		case "artist", "year", "image", "remaster_year", "remaster_title",
			"remaster_record_label", "remaster_catalogue_number":
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

	return a.doStarSpaceUpload(httpReq, config)
}

func (a *GenericAdapter) doStarSpaceUpload(httpReq *http.Request, config *model.SiteConfig) (*model.PublishResponse, error) {
	resp, err := a.doer.Client.Do(httpReq) //nolint:gosec // URL from admin-configured site base URL
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
		return nil, classifySiteError(resp, "upload")
	}

	if idMatch := reGenericStarSpaceDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  config.Domain + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reGenericErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *GenericAdapter) uploadYemaPT(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := resolveBaseURL(config)
	uploadURL := baseURL + "/api/torrent/add"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("files", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		fw.writeField("showName", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("shortDesc", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.Anonymous {
		fw.writeField("uploadUserAnonymous", "是")
	}

	yemaFieldMap := map[string]string{
		"cat":            "categoryId",
		"medium_sel":     "medium",
		"codec_sel":      "codec",
		"audiocodec_sel": "audiocodec",
		"standard_sel":   "standard",
		"processing_sel": "region",
		"team_sel":       "team",
	}
	for srcType, targetName := range yemaFieldMap {
		if v, ok := req.FormFields[srcType]; ok {
			fw.writeField(targetName, v)
		}
	}

	if tags, ok := req.FormFields["tags"]; ok {
		tagList := strings.Split(tags, ",")
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				fw.writeField("tagList", tag)
			}
		}
	}

	if imdbID := extractIMDbIDGeneric(req.IMDbLink); imdbID != "" {
		fw.writeField("imdb", imdbID)
	}
	if req.DoubanLink != "" {
		m := reGenericDigits.FindString(req.DoubanLink)
		if m != "" {
			fw.writeField("douban", m)
		}
	}
	if v, ok := req.ExtraFields["picture"]; ok {
		fw.writeField("picture", v)
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "audiocodec_sel", "standard_sel",
			"processing_sel", "team_sel", "tags":
			continue
		}
		fw.writeField(k, v)
	}
	for k, v := range req.ExtraFields {
		if k == "picture" {
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

	var result struct {
		Success  bool        `json:"success"`
		ShowType int         `json:"showType"`
		Data     interface{} `json:"data"`
		Message  string      `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil {
		if result.Success {
			dataMap, ok := result.Data.(map[string]interface{})
			torrentID := ""
			if ok {
				if id, ok := dataMap["id"]; ok {
					torrentID = fmt.Sprintf("%v", id)
				}
			}
			detailURL := ""
			if torrentID != "" {
				detailURL = baseURL + "/torrent/detail/" + torrentID
			}
			return &model.PublishResponse{
				Success:    true,
				TorrentID:  torrentID,
				DetailURL:  detailURL,
				TargetSite: config.Domain,
			}, nil
		}
		errMsg := result.Message
		if errMsg == "" {
			errMsg = fmt.Sprintf("上传失败: showType=%d", result.ShowType)
		}
		return nil, &model.AppError{Code: 15001, Message: errMsg}
	}

	html := string(body)
	if strings.Contains(html, "成功") || strings.Contains(html, "success") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)}
}
