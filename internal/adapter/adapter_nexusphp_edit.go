// Package adapter NexusPHP 编辑接口实现（§56.23 决策 2/4）。
//
// GetEditForm: GET edit.php?id=XXX → 解析表单字段
// SubmitEdit: POST takeedit.php → 提交编辑
//
// 覆盖 90+ NexusPHP 站点（通用 takeedit.php 机制）。
package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ranfish/pt-forward/internal/model"
)

// GetEditForm §56.23: 获取编辑表单（NexusPHP edit.php）。
func (a *NexusPHPAdapter) GetEditForm(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.EditForm, error) {
	if config == nil || torrentID == "" {
		return nil, fmt.Errorf("config and torrentID required")
	}
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://" + config.Domain
	}
	editURL := strings.TrimRight(baseURL, "/") + "/edit.php?id=" + torrentID

	req, err := http.NewRequestWithContext(ctx, "GET", editURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build edit request: %w", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch edit page: %w", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edit page returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse edit page: %w", err)
	}

	form := &model.EditForm{
		TorrentID: torrentID,
		Fields:    make(map[string]string),
	}

	// 提取 descr textarea（简介）
	if s := doc.Find("textarea[name='descr']").First(); s.Length() > 0 {
		form.Description = s.Text()
		form.ExistingDesc = s.Text()
	}

	// 提取 title
	if s := doc.Find("input[name='name']").First(); s.Length() > 0 {
		form.Title, _ = s.Attr("value")
	}

	// 提取所有 input/select 字段
	doc.Find("input[type='text'], input[type='hidden'], select").Each(func(_ int, s *goquery.Selection) {
		name, ok := s.Attr("name")
		if !ok || name == "" {
			return
		}
		// select 取选中项
		if s.Is("select") {
			s.Find("option[selected]").Each(func(_ int, opt *goquery.Selection) {
				if v, ok := opt.Attr("value"); ok {
					form.Fields[name] = v
				}
			})
			return
		}
		// input 取 value
		if v, ok := s.Attr("value"); ok && v != "" {
			// 跳过空名/敏感字段
			if name == "auth" || strings.HasPrefix(name, "_") {
				form.Fields[name] = v // 保留 auth 等隐藏字段
			} else {
				form.Fields[name] = v
			}
		}
	})

	// small_descr（副标题）
	if s := doc.Find("input[name='small_descr']").First(); s.Length() > 0 {
		if v, ok := s.Attr("value"); ok {
			form.Fields["small_descr"] = v
		}
	}

	return form, nil
}

// SubmitEdit §56.23: 提交编辑（NexusPHP takeedit.php）。
func (a *NexusPHPAdapter) SubmitEdit(ctx context.Context, req *model.EditRequest) error {
	if req == nil || req.TorrentID == "" {
		return fmt.Errorf("edit request requires torrentID")
	}
	baseURL := req.BaseURL
	if baseURL == "" {
		return fmt.Errorf("baseURL required for submit edit")
	}
	takeEditURL := strings.TrimRight(baseURL, "/") + "/takeedit.php"

	// 构建 form data
	form := url.Values{}
	form.Set("id", req.TorrentID)
	for k, v := range req.FormFields {
		form.Set(k, v)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", takeEditURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build submit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Referer", req.Referer)
	setCommonHeaders(httpReq, req.Cookie)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("submit edit: %w", err)
	}
	defer func() { drainBody(resp) }()

	// 302/301 重定向 = 成功
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		return nil
	}

	// 200 = 可能有错误信息
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		// 检查常见错误
		if strings.Contains(bodyStr, "失败") || strings.Contains(bodyStr, "error") || strings.Contains(bodyStr, "权限") {
			return fmt.Errorf("edit failed: %s", extractErrorMessage(bodyStr))
		}
		// 有些站点 200 也是成功（无重定向）
		return nil
	}

	return fmt.Errorf("unexpected status %d from takeedit", resp.StatusCode)
}

// extractErrorMessage 从 HTML 中提取错误信息。
var errorMsgRe = regexp.MustCompile(`(?is)(?:<p[^>]*>|<div[^>]*class="[^"]*error[^"]*"[^>]*>)([^<]+)`)

func extractErrorMessage(html string) string {
	m := errorMsgRe.FindStringSubmatch(html)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return "未知错误"
}
