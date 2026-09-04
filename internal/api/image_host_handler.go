package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
)

type ImageHostHandler struct {
	mgr       *imagehost.Manager
	settings  *setting.Repository
	logger    *zap.Logger
}

func NewImageHostHandler(mgr *imagehost.Manager, settings *setting.Repository, logger *zap.Logger) *ImageHostHandler {
	return &ImageHostHandler{mgr: mgr, settings: settings, logger: logger}
}

func (h *ImageHostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/settings/image-host") && r.Method == http.MethodGet:
		h.handleGet(w, r)
	case strings.HasSuffix(path, "/settings/image-host") && r.Method == http.MethodPut:
		h.handlePut(w, r)
	case strings.HasSuffix(path, "/settings/image-host/test") && r.Method == http.MethodPost:
		h.handleTest(w, r)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *ImageHostHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result := map[string]interface{}{
		"hosts":   h.mgr.ListHosts(),
		"default": "",
		"strategy": "auto",
	}

	if v, err := h.settings.Get(ctx, setting.KeyImageHostDefault); err == nil {
		result["default"] = v
	}
	if v, err := h.settings.Get(ctx, setting.KeyImageHostStrategy); err == nil {
		result["strategy"] = v
	}
	if v, err := h.settings.Get(ctx, setting.KeyAGSVPTEmail); err == nil {
		result["agsvpt_email"] = v
	}
	result["agsvpt_configured"] = h.isAGSVPTConfigured(ctx)

	health := h.mgr.HealthCheckAll(r.Context())
	healthMap := make(map[string]string)
	for name, err := range health {
		if err != nil {
			healthMap[name] = err.Error()
		} else {
			healthMap[name] = "ok"
		}
	}
	result["health"] = healthMap

	Success(w, result)
}

func (h *ImageHostHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Default   string `json:"default"`
		Strategy  string `json:"strategy"`
		AGSVPTEmail string `json:"agsvpt_email"`
		AGSVPTPassword string `json:"agsvpt_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "参数解析失败")
		return
	}

	// §59.167 PT31 实战修复：AGSVPT 首配链断裂——原顺序 default 先校验
	// mgr.SetDefault("agsvpt") 必失败（host 仅启动时按已有凭证注册；首配时
	// 凭证还没存→未注册→400 早退，邮箱密码也存不上）。修正：凭证先落库+
	// 运行时注册（不存在的 agsvpt 动态 Register），再设默认。
	if req.AGSVPTEmail != "" && req.AGSVPTPassword != "" {
		if _, err := h.mgr.GetHost("agsvpt"); err != nil {
			h.mgr.Register(imagehost.NewAGSVPTHost(req.AGSVPTEmail, req.AGSVPTPassword, h.logger))
		}
	}
	if req.Default != "" {
		if err := h.settings.Set(ctx, setting.KeyImageHostDefault, req.Default); err != nil {
			Error(w, http.StatusInternalServerError, 50000, "保存默认图床失败")
			return
		}
		if err := h.mgr.SetDefault(req.Default); err != nil {
			Error(w, http.StatusBadRequest, 40001, err.Error())
			return
		}
	}

	if req.Strategy != "" {
		if err := h.settings.Set(ctx, setting.KeyImageHostStrategy, req.Strategy); err != nil {
			Error(w, http.StatusInternalServerError, 50000, "保存截图策略失败")
			return
		}
	}

	if req.AGSVPTEmail != "" {
		if err := h.settings.Set(ctx, setting.KeyAGSVPTEmail, req.AGSVPTEmail); err != nil {
			Error(w, http.StatusInternalServerError, 50000, "保存AGSVPT邮箱失败")
			return
		}
	}
	if req.AGSVPTPassword != "" {
		if err := h.settings.Set(ctx, setting.KeyAGSVPTPassword, req.AGSVPTPassword); err != nil {
			Error(w, http.StatusInternalServerError, 50000, "保存AGSVPT密码失败")
			return
		}
		if host, err := h.mgr.GetHost("agsvpt"); err == nil {
			if agsvpt, ok := host.(*imagehost.AGSVPTHost); ok {
				agsvpt.SetCredentials(req.AGSVPTEmail, req.AGSVPTPassword)
			}
		}
	}

	Success(w, map[string]interface{}{"success": true})
}

func (h *ImageHostHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Host string `json:"host"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	hostName := req.Host
	if hostName == "" {
		if v, err := h.settings.Get(ctx, setting.KeyImageHostDefault); err == nil {
			hostName = v
		}
	}

	host, err := h.mgr.GetHost(hostName)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "图床未配置: "+err.Error())
		return
	}

	testData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	result, err := host.Upload(ctx, testData, "test.png")
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "测试上传失败: "+err.Error())
		return
	}

	Success(w, map[string]interface{}{
		"success": true,
		"url":     result.URL,
		"host":    result.HostName,
	})
}

func (h *ImageHostHandler) isAGSVPTConfigured(ctx context.Context) bool {
	email, err := h.settings.Get(ctx, setting.KeyAGSVPTEmail)
	if err != nil || email == "" {
		return false
	}
	pass, err := h.settings.Get(ctx, setting.KeyAGSVPTPassword)
	return err == nil && pass != ""
}
