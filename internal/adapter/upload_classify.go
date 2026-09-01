// Package adapter 上传响应判定公共单点（§59.159 复盘——双副本教训）。
//
// Generic/NexusPHP 两 adapter 的 UploadTorrent 曾各持一份同款判定，
// 修复改在 Generic 漏了 NexusPHP（幸运走 NexusPHP 路径）——四次假成功排查之源。
// 本文件收敛：existed 重定向捕获 + 已存在文本判定 + 详情 ID 提取（词边界），
// 两 adapter 统一调用，禁止再各写副本。
package adapter

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// reUploadDetailID 详情页 ID（§59.159 词边界——防 userdetails.php?id=N 用户导航
// 链接误命中：实战失败页被误抓判"成功"，tid 恒为用户自身 UID）。
var reUploadDetailID = regexp.MustCompile(`[^a-zA-Z0-9_](?:details|detail|torrent)\.php\?id=(\d+)`)

// uploadRedirectClient 构造捕获 existed=1 的重定向 HTTP 客户端
// （NP 重复上传 302 → details.php?id=N&existed=1 形态；transport 继承站点代理配置）。
// 返回值 existingIDPtr 在 Do 期间被填充。
func uploadRedirectClient(base *http.Client) (client *http.Client, existingIDPtr *string) {
	id := ""
	c := &http.Client{
		Timeout:   base.Timeout,
		Transport: base.Transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Query().Get("existed") == "1" {
				if v := req.URL.Query().Get("id"); v != "" {
					id = v
				}
			}
			return nil
		},
	}
	return c, &id
}

// classifyUploadHTML 上传响应 HTML 判定（公共单点）。
// 判定序（§59.159）：existed 重定向 > 已存在文本（stderr 200 页——幸运实测形态）
// > 详情 ID（词边界）> 成功文案 > 失败（返回 nil，errMsg 由调用方兜底）。
// logger 非空时对详情 ID 命中路径打 body 摘要诊断（假成功排查期）。
func classifyUploadHTML(logger *zap.Logger, site, detailBase, html, existingRedirectID string) *model.PublishResponse {
	// ① 重定向 existed（uploadRedirectClient 捕获）
	if existingRedirectID != "" {
		return &model.PublishResponse{
			Success: true, IsExisting: true, ExistingID: existingRedirectID,
			DetailURL: detailBase + "/details.php?id=" + existingRedirectID,
			TargetSite: site,
		}
	}
	// ② 已存在文本（旧版 NP stderr 200 页——PTNexus 竞品同款关键词族）
	if strings.Contains(html, "已存在") || strings.Contains(strings.ToLower(html), "already exists") {
		exID := ""
		if m := reUploadDetailID.FindStringSubmatch(html); len(m) > 1 {
			exID = m[1]
		}
		resp := &model.PublishResponse{Success: true, IsExisting: true, ExistingID: exID, TargetSite: site}
		if exID != "" {
			resp.DetailURL = detailBase + "/details.php?id=" + exID
		}
		return resp
	}
	// ③ 详情 ID（词边界）——带诊断摘要
	if m := reUploadDetailID.FindStringSubmatch(html); len(m) > 1 {
		if logger != nil {
			plain := strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(html, "<", " <"), ">", "> ")), " ")
			if len(plain) > 700 {
				plain = plain[:700]
			}
			logger.Info("upload success-path body digest",
				zap.String("site", site), zap.String("tid", m[1]), zap.String("body", plain))
		}
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  m[1],
			DetailURL:  detailBase + "/details.php?id=" + m[1],
			TargetSite: site,
		}
	}
	// ④ 成功文案（无详情 ID 的弱成功——诊断期 dump 摘要，防再次假成功）
	if strings.Contains(html, "uploaded") || strings.Contains(html, "成功") ||
		strings.Contains(html, "Upload succeeded") || strings.Contains(html, "succeeded") {
		if logger != nil {
			plain := strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(html, "<", " <"), ">", "> ")), " ")
			if len(plain) > 700 {
				plain = plain[:700]
			}
			logger.Info("upload weak-success body digest",
				zap.String("site", site), zap.String("body", plain))
		}
		return &model.PublishResponse{Success: true, TargetSite: site}
	}
	return nil
}
