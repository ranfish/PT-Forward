package adapter

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// hdcityUploadTwoStep §59.454: 城市（hdcity.city）两步上传——全库唯一
// "先上传种子文件再填表单"站（NexusPHP 深度定制 ProCity 主题）。
//
// 流程（用户授权第一步实测 2026-11-02 解明）：
//   ① GET /upload → 解析第一步表单（vf+日期动态字段/uid/domainname/
//     跨域 action=接收器 URL 含 spm 参数）
//   ② POST 种子文件到跨域接收器（hctres.leniter.org/upload_receiver.php）
//     携带 vf/uid/domainname/citeid=0/name → 302 Location 携带
//     tfu/tn/f/s/n/ih/citeid 参数（协议相对 //hdcity.city/upload?...）
//   ③ POST 第二步表单到 /upload?tfu=...（action=当前 URL 含查询参数——
//     无 file 字段，种子信息经 hidden infohash/listtype/ressize/
//     filecount 传递；vf 字段沿用第一步值——表单页 GET 时同 session）
//
// tfu 实效短（实测 ~10 分钟过期——过期后 GET 返回 ASCII art 彩蛋页
// 无表单），第二步紧跟第一步即可。
func (a *NexusPHPAdapter) hdcityUploadTwoStep(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := "https://" + config.Domain
	if strings.HasPrefix(config.Domain, "http") {
		baseURL = config.Domain
	}

	// ① 第一步页面：解析动态 vf 字段（vf+YYYYMMDD 每日轮换）/uid/action
	page1, err := a.hdcityGet(ctx, config, baseURL+"/upload")
	if err != nil {
		return nil, networkError("城市第一步页面获取失败", err)
	}
	vfM := reHdcityVF.FindSubmatch(page1)
	if vfM == nil {
		return nil, uploadError("城市第一步表单解析失败（vf 动态字段缺失——页面可能改版或登录态失效）", nil)
	}
	vfName, vfVal := string(vfM[1]), string(vfM[2])
	uidM := reHdcityUID.FindSubmatch(page1)
	if uidM == nil {
		return nil, uploadError("城市第一步表单解析失败（uid 字段缺失）", nil)
	}
	actionM := reHdcityAction.FindSubmatch(page1)
	if actionM == nil {
		return nil, uploadError("城市第一步表单解析失败（跨域接收器 action 缺失）", nil)
	}

	// ② POST 种子文件到跨域接收器（不跟随 302——解析 Location）
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, ferr := w.CreateFormFile("file", "upload.torrent")
	if ferr != nil {
		return nil, uploadError("创建表单文件字段失败", ferr)
	}
	if _, err := fw.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}
	w.WriteField("name", req.Title)
	w.WriteField("citeid", "0")
	w.WriteField(vfName, vfVal)
	w.WriteField("domainname", config.Domain)
	w.WriteField("uid", string(uidM[1]))
	w.Close()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", string(actionM[1]), &buf)
	if err != nil {
		return nil, networkError("构造第一步请求失败", err)
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	setCommonHeaders(httpReq, config.Cookie)
	httpReq.Header.Set("Origin", baseURL)
	httpReq.Header.Set("Referer", baseURL+"/upload")
	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("城市第一步上传请求失败", err)
	}
	drainBody(resp)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return nil, classifySiteError(resp, "hdcity step1 (receiver)")
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil, uploadError("城市第一步响应无 Location（302 缺失）", nil)
	}
	if strings.HasPrefix(loc, "//") {
		loc = "https:" + loc
	}
	if !strings.Contains(loc, "tfu=") {
		return nil, uploadError("城市第一步 Location 无 tfu 参数（种子可能被拒）", nil)
	}

	// ③ POST 第二步表单（action=当前 URL 含查询参数）
	var formBuf strings.Builder
	formBuf.WriteString("infohash=" + extractQueryParam(loc, "ih"))
	formBuf.WriteString("&listtype=1")
	formBuf.WriteString("&ressize=" + extractQueryParam(loc, "s"))
	formBuf.WriteString("&filecount=" + extractQueryParam(loc, "f"))
	formBuf.WriteString("&name=" + urlEncode(req.Title))
	formBuf.WriteString("&bigname=" + urlEncode(req.FormFields["bigname"]))
	formBuf.WriteString("&small_descr=" + urlEncode(req.FormFields["small_descr"]))
	formBuf.WriteString("&descr=" + urlEncode(req.Description))
	if req.IMDbLink != "" {
		formBuf.WriteString("&url=" + urlEncode(req.IMDbLink))
	}
	if req.Anonymous {
		formBuf.WriteString("&uplver=yes")
	}
	// 槽位字段（form 里已由 executor 特化填充——直接透传 req.FormFields
	// 的 select 域键）
	for _, key := range []string{"type", "medium_sel", "codec_sel", "standard_sel", "processing_sel", "team_sel", "tag1ing", "tag2ing", "mediainfo", "posterimg", "sub_soft", "sub_hard", "sub_incl", "chnize"} {
		if v, ok := req.FormFields[key]; ok && v != "" {
			formBuf.WriteString("&" + key + "=" + urlEncode(v))
		}
	}

	step2Req, err := http.NewRequestWithContext(ctx, "POST", loc, strings.NewReader(formBuf.String()))
	if err != nil {
		return nil, networkError("构造第二步请求失败", err)
	}
	step2Req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setCommonHeaders(step2Req, config.Cookie)
	step2Req.Header.Set("Origin", baseURL)
	step2Req.Header.Set("Referer", loc)
	resp2, err := a.doer.Client.Do(step2Req)
	if err != nil {
		return nil, uploadError("城市第二步提交失败", err)
	}
	body2, err := readBody(resp2)
	defer func() { drainBody(resp2) }()
	if err != nil {
		return nil, err
	}
	if resp2.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp2, "hdcity step2 (upload)")
	}

	// 成功判定：302 跳详情页（成功页 URL 形态 /t-{id}）或响应含成功标记
	html := string(body2)
	if m := reHdcityDetail.FindStringSubmatch(html); len(m) > 1 {
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  m[1],
			DetailURL:  baseURL + "/t-" + m[1],
			TargetSite: config.Domain,
		}, nil
	}
	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}
	// 错误信息提取
	if m := reHdcityErr.FindStringSubmatch(html); len(m) > 1 {
		return &model.PublishResponse{Success: false, ErrorMessage: m[1]}, nil
	}
	return nil, &model.AppError{Code: 15001, Message: "城市上传失败: HTTP " + fmt.Sprintf("%d", resp2.StatusCode)}
}

// hdcityGet 城市站 GET（带 NP 标准 cookie 头）。
func (a *NexusPHPAdapter) hdcityGet(ctx context.Context, config *model.SiteConfig, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()
	return readBody(resp)
}

func extractQueryParam(rawURL, key string) string {
	i := strings.Index(rawURL, key+"=")
	if i < 0 {
		return ""
	}
	rest := rawURL[i+len(key)+1:]
	if j := strings.IndexAny(rest, "&"); j >= 0 {
		return rest[:j]
	}
	return rest
}

func urlEncode(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "&", "%26"), "=", "%3D")
}

var (
	reHdcityVF     = regexp.MustCompile(`name="(vf\d+)" value="([^"]+)"`)
	reHdcityUID    = regexp.MustCompile(`name="uid" value="([^"]+)"`)
	reHdcityAction = regexp.MustCompile(`action="(https://hctres[^"]+)"`)
	reHdcityDetail = regexp.MustCompile(`(?:/t-|details(?:\.php)?\?id=)(\d+)`)
	reHdcityErr    = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
)
