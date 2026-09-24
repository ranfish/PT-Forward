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
			// auth 等隐藏字段与普通字段同样保留（历史分支同体已合并）
			form.Fields[name] = v
		}
	})

	// §59.269: checkbox:checked / radio:checked 采集 → ArrayFields（同名多值——
	// tags[4][] 等 checkbox 数组缺失提交=清空勾选，必须原样回放）
	doc.Find("input[type='checkbox']:checked, input[type='radio']:checked").Each(func(_ int, s *goquery.Selection) {
		name, ok := s.Attr("name")
		if !ok || name == "" {
			return
		}
		v, _ := s.Attr("value")
		if v == "" {
			v = "on"
		}
		form.ArrayFields = append(form.ArrayFields, model.TagKV{Key: name, Value: v})
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
	// §59.269: 同名多值字段回放（Add 非 Set——checkbox 数组重复键语义）
	for _, kv := range req.ArrayFields {
		form.Add(kv.Key, kv.Value)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", takeEditURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build submit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Referer", req.Referer)
	setCommonHeaders(httpReq, req.Cookie)

	// §59.271: 禁止自动跟随重定向——成功判定必须看首响应（302=成功）。
	// 此前跟随到 details 页（200）后在正文 grep "失败/error/权限"，而站点
	// slogan/简介声明常含这些词 → 编辑已生效却误报失败（修道院 1898 案：
	// 报错但 codec 实际已落库）。
	noRedirect := *a.doer.Client
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := noRedirect.Do(httpReq)
	if err != nil {
		return fmt.Errorf("submit edit: %w", err)
	}
	defer func() { drainBody(resp) }()

	// 302/301 重定向 = 成功（NexusPHP takeedit 标准行为）
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		return nil
	}

	// 200 = 真·错误页（站方 stderr 模板——含真实原因如"有项目没有填写"）
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("edit failed: %s", extractErrorMessage(string(body)))
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
