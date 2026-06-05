package api

import (
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
)

// SupportedSitesHandler 暴露系统白名单站点（go:embed seed）给前端。
//
// 路由：
//   GET /api/v1/supported-sites            列表（可选过滤 framework/status/search）
//   GET /api/v1/supported-sites/{domain}   单个站点详情
type SupportedSitesHandler struct {
	logger *zap.Logger
}

func NewSupportedSitesHandler(logger *zap.Logger) *SupportedSitesHandler {
	return &SupportedSitesHandler{logger: logger}
}

const supportedSitesBase = "/api/v1/supported-sites"

func (h *SupportedSitesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	if err := site.SupportedSitesErr(); err != nil {
		h.logger.Error("supported_sites.json 解析失败", zap.Error(err))
		ErrorWithDetail(w, http.StatusInternalServerError, 50001, "白名单数据加载失败", err.Error())
		return
	}

	path := strings.TrimRight(r.URL.Path, "/")

	if path == supportedSitesBase {
		h.handleList(w, r)
		return
	}

	remaining := strings.TrimPrefix(path, supportedSitesBase+"/")
	if remaining == "" || strings.Contains(remaining, "/") {
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}
	h.handleGet(w, remaining)
}

func (h *SupportedSitesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	framework := strings.TrimSpace(q.Get("framework"))
	status := strings.TrimSpace(q.Get("status"))
	search := strings.ToLower(strings.TrimSpace(q.Get("search")))

	all := site.ListSupportedSites()
	out := make([]*site.SupportedSite, 0, len(all))
	for _, s := range all {
		if framework != "" && s.Framework != framework {
			continue
		}
		if status != "" && s.VerificationStatus != status {
			continue
		}
		if search != "" {
			if !strings.Contains(strings.ToLower(s.Domain), search) &&
				!strings.Contains(strings.ToLower(s.NameCN), search) {
				continue
			}
		}
		out = append(out, s)
	}

	Success(w, map[string]interface{}{
		"items": out,
		"total": len(out),
	})
}

func (h *SupportedSitesHandler) handleGet(w http.ResponseWriter, domain string) {
	s, ok := site.GetSupportedSite(domain)
	if !ok {
		Error(w, http.StatusNotFound, 40400, "站点不在支持列表中")
		return
	}
	Success(w, s)
}
