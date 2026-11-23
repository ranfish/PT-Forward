package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (a *MTeamAdapter) detailViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

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
		Code    json.Number `json:"code"`
		Message string      `json:"message"`
		Data    struct {
			Name        string   `json:"name"`
			SmallDescr  string   `json:"smallDescr"`
			Description string   `json:"description"`
			Size        flexInt  `json:"size"`
			InfoHash    string   `json:"infoHash"`
			Category    string   `json:"category"`
			Source      string   `json:"source"`
			Standard    string   `json:"standard"`
			VideoCodec  string   `json:"videoCodec"`
			AudioCodec  string   `json:"audioCodec"`
			Team        string   `json:"team"`
			Imdb        string   `json:"imdb"`
			Douban      string   `json:"douban"`
			MediaInfo   string   `json:"mediaInfo"`
			Tags        []string `json:"tags"`
			Status      struct {
				Discount        string  `json:"discount"`
				DiscountEndTime string  `json:"discountEndTime"`
				Seeders         flexInt `json:"seeders"`
				Leechers        flexInt `json:"leechers"`
				HR              bool    `json:"hr"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析 API 响应失败", err)
	}
	// §59.60 B1: code≠0（无效 tid/限流/域名熔断）必须报错——原实现直接解空 data
	// 构造全空 detail 伪成功（243 实测空行落库且降级链不触发）。照抄 search :577 模式。
	if result.Code.String() != "0" {
		return nil, parseError(fmt.Sprintf("API 详情失败: %s", result.Message), nil)
	}

	d := result.Data
	// §59.317: detail 响应可观测——raw category 原始值落日志（对齐 §59.223 search
	// req/resp Debug 模式；243 全库 cartoon 案因 detail 无日志成盲区，原始值不可考）。
	if a.logger != nil {
		a.logger.Debug("mteam detail resp",
			zap.String("tid", torrentID),
			zap.String("raw_category", d.Category),
			zap.Int("body_bytes", len(body)))
	}
	detail := &model.TorrentDetail{
		Title:        d.Name,
		Subtitle:     d.SmallDescr,
		Description:  d.Description,
		Size:         int64(d.Size),
		InfoHash:     strings.ToLower(d.InfoHash),
		Category:     d.Category,
		Source:       d.Source,
		Resolution:   d.Standard,
		Codec:        d.VideoCodec,
		AudioCodec:   d.AudioCodec,
		ReleaseGroup: d.Team,
		MediaInfo:    d.MediaInfo,
		IMDbID:       d.Imdb,
		DoubanID:     d.Douban,
		Tags:         d.Tags,
	}
	if d.Imdb != "" {
		detail.IMDbURL = buildIMDbURL(d.Imdb) // §59.297 双形态
	}
	if d.Douban != "" {
		detail.DoubanURL = buildDoubanURL(d.Douban) // §59.297 双形态
	}
	if d.Description != "" {
		detail.Screenshots = extractScreenshotsFromBBCode(d.Description)
		if len(detail.Screenshots) > 0 {
			detail.PosterURL = detail.Screenshots[0]
			detail.Screenshots = detail.Screenshots[1:]
		}
	}
	// §59.317 P0+P3: 分类归一——动态缓存（categoryList）→ 静态表两级解析。
	// 成人区命中：flags 标记 adult（§56.40 合规增强）+ category.adult。
	if nc := a.resolveCategory(ctx, config, detail.Category); nc != "" {
		detail.Category = nc
		if nc == "category.adult" {
			detail.Flags = []string{"adult"}
			return detail, nil
		}
	} else {
		// map miss：站方新增分类早知道（Warn 可观测——本案若有此告警 09-30
		// 当天即暴露，无需等用户肉眼发现错标）
		if a.logger != nil && detail.Category != "" {
			a.logger.Warn("mteam category id unmapped",
				zap.String("tid", torrentID),
				zap.String("raw_category", detail.Category))
		}
		detail.Category = NormalizeCategory(detail.Category)
	}
	// 从结构化字段提取禁转标记（§56.37 合规修复）
	detail.Flags = extractFlagsFromStructured(detail.Title, detail.Subtitle, detail.Tags)
	return detail, nil
}

func (a *MTeamAdapter) detailViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
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
	if m := reMTeamTitle.FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}
	if m := reMTeamInfoHash.FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}
	if m := reMTeamSize.FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}
	if m := reMTeamCategory.FindStringSubmatch(html); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}
	detail.Tags = extractTags(html)

	return detail, nil
}
