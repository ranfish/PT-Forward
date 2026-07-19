// Package adapter MTeam 编辑接口实现（§56.23 决策 4）。
//
// MTeam 用 /api/torrent/createOredit 端点（创建+编辑共用）。
// 编辑模式：multipart/form-data + id 参数 + x-api-key 认证。
// 来源：docs/25-M-Team-API完整指南.md
package adapter

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// GetEditForm §56.23: MTeam 获取编辑表单（复用 GetTorrentDetail）。
func (a *MTeamAdapter) GetEditForm(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.EditForm, error) {
	if config == nil || torrentID == "" {
		return nil, fmt.Errorf("config and torrentID required")
	}

	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return nil, fmt.Errorf("get torrent detail for edit: %w", err)
	}
	if detail == nil {
		return nil, fmt.Errorf("torrent %s not found", torrentID)
	}

	form := &model.EditForm{
		TorrentID:    torrentID,
		Title:        detail.Title,
		Description:  detail.Description,
		ExistingDesc: detail.Description,
		Fields:       make(map[string]string),
	}

	// MTeam API 字段映射
	form.Fields["smallDescr"] = detail.Subtitle
	form.Fields["imdb"] = detail.IMDbURL
	form.Fields["douban"] = detail.DoubanURL
	form.Fields["mediainfo"] = detail.MediaInfo
	form.Fields["category"] = detail.Category
	form.Fields["source"] = detail.Source
	form.Fields["standard"] = detail.Resolution
	form.Fields["videoCodec"] = detail.Codec
	form.Fields["audioCodec"] = detail.AudioCodec
	form.Fields["team"] = detail.ReleaseGroup

	return form, nil
}

// SubmitEdit §56.23: MTeam 提交编辑（POST /api/torrent/createOredit + id）。
func (a *MTeamAdapter) SubmitEdit(ctx context.Context, req *model.EditRequest) error {
	if req == nil || req.TorrentID == "" {
		return fmt.Errorf("edit request requires torrentID")
	}
	if req.APIKey == "" {
		return fmt.Errorf("MTeam edit requires APIKey")
	}

	editURL := strings.TrimRight(req.BaseURL, "/") + "/api/torrent/createOredit"

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	// id（编辑模式的关键参数）
	fw.writeField("id", req.TorrentID)

	// 表单字段
	for k, v := range req.FormFields {
		if v != "" {
			fw.writeField(k, v)
		}
	}

	if err := fw.hasError(); err != nil {
		return fmt.Errorf("write form fields: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", editURL, &buf)
	if err != nil {
		return fmt.Errorf("build edit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("x-api-key", req.APIKey)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("submit edit: %w", err)
	}
	defer func() { drainBody(resp) }()

	body, _ := readBody(resp)

	// MTeam API 返回 JSON {code, message, data}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("edit failed: HTTP %d, body: %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}

	// 检查 MTeam 统一响应格式
	bodyStr := string(body)
	if strings.Contains(bodyStr, `"code"`) {
		// JSON 响应，检查 code 字段
		if strings.Contains(bodyStr, `"code":0`) || strings.Contains(bodyStr, `"code": 0`) {
			return nil // code=0 表示成功
		}
		return fmt.Errorf("edit API error: %s", bodyStr[:min(300, len(bodyStr))])
	}

	return nil
}
