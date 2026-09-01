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

// reUploadNewID NP 上传成功页新种链接（details.php?id=N&uploaded=1——权威形态；
// 实战：普通形态首个命中是推荐位种子 53342 而非新种 53505）
var reUploadNewID = regexp.MustCompile(`[^a-zA-Z0-9_]details\.php\?id=(\d+)&uploaded=1`)

// uploadRedirectClient 构造捕获 existed=1 的重定向 HTTP 客户端
// （NP 重复上传 302 → details.php?id=N&existed=1 形态；transport 继承站点代理配置）。
// 返回值 existingIDPtr 在 Do 期间被填充。
func uploadRedirectClient(base *http.Client) (client *http.Client, existingIDPtr, finalIDPtr *string) {
	existingID := ""
	finalID := ""
	c := &http.Client{
		Timeout:   base.Timeout,
		Transport: base.Transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			id := req.URL.Query().Get("id")
			if req.URL.Query().Get("existed") == "1" && id != "" {
				existingID = id
			}
			// §59.159: 记录最后一跳的 details id——NP 上传成功 302 → 新种
			// 详情页，最终 URL 的 id 才是权威新种 ID（body 链接是推荐位，实战
			// 抓错两次：53342 HDH/52394 UBits 无关种）
			if id != "" && strings.Contains(req.URL.Path, "details.php") {
				finalID = id
			}
			return nil
		},
	}
	return c, &existingID, &finalID
}

// classifyUploadHTML 上传响应 HTML 判定（公共单点）。
// 判定序（§59.159）：existed 重定向 > 已存在文本（stderr 200 页——幸运实测形态）
// > 详情 ID（词边界）> 成功文案 > 失败（返回 nil，errMsg 由调用方兜底）。
// logger 非空时对详情 ID 命中路径打 body 摘要诊断（假成功排查期）。
func classifyUploadHTML(logger *zap.Logger, site, detailBase, html, existingRedirectID, finalRedirectID string) *model.PublishResponse {
	// ① 重定向 existed（uploadRedirectClient 捕获）
	if existingRedirectID != "" {
		return &model.PublishResponse{
			Success: true, IsExisting: true, ExistingID: existingRedirectID,
			DetailURL: detailBase + "/details.php?id=" + existingRedirectID,
			TargetSite: site,
		}
	}
	// ①' 重定向最终 URL 的新种 id（NP 成功 302 → details.php?id=NEW——权威，
	// body 链接不可信[推荐位]；仅当 body 也佐证成功形态时采用）
	if finalRedirectID != "" && (strings.Contains(html, "成功") || strings.Contains(html, "uploaded") ||
		strings.Contains(html, "已存在") || reUploadDetailID != nil) {
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  finalRedirectID,
			DetailURL:  detailBase + "/details.php?id=" + finalRedirectID,
			TargetSite: site,
			IDSource:   "redirect", // 成功页 302 权威——加种免检
		}
	}
	// §59.159 四轮定案（用户）：发布二元化——**仅接受发布成功页的种子**：
	//   成功页形态① 302 最终 URL（finalRedirectID，IDSource=redirect）
	//   成功页形态② 200 成功页内 uploaded=1 权威链接（NP 成功页标志参数——
	//              推荐位链接不带此参数，天然区分）
	// 其余（通用 body 链接/成功文案）一律失败——其它页面的种子是辅种业务
	// 的工作，发布不抓取（53342/52394 推荐位误推根治）。
	if m := reUploadNewID.FindStringSubmatch(html); len(m) > 1 {
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  m[1],
			DetailURL:  detailBase + "/details.php?id=" + m[1],
			TargetSite: site,
			IDSource:   "success_page", // 成功页 uploaded=1 权威链接
		}
	}
	if strings.Contains(html, "已存在") || strings.Contains(strings.ToLower(html), "already exists") {
		// 站上已有同种=发布业务终点（PT 站不允许重复发——辅种业务范畴）
		return &model.PublishResponse{
			Success:      false,
			TargetSite:   site,
			ErrorMessage: "站点提示种子已存在（发布不重复发——辅种业务范畴）",
		}
	}
	if logger != nil {
		plain := strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(html, "<", " <"), ">", "> ")), " ")
		if len(plain) > 700 {
			plain = plain[:700]
		}
		logger.Warn("upload rejected: no success-page seed link",
			zap.String("site", site), zap.String("body", plain))
	}
	return nil
}