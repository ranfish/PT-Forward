package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
	"go.uber.org/zap"
)

type NotifyHandler struct {
	repo    *notification.Repository
	service *notification.Service
	logger  *zap.Logger
}

func NewNotifyHandler(repo *notification.Repository, service *notification.Service, logger *zap.Logger) *NotifyHandler {
	return &NotifyHandler{repo: repo, service: service, logger: logger}
}

var validNotifyTypes = map[string]bool{
	"telegram": true, "bark": true, "webhook": true,
	"serverchan": true, "dingtalk": true,
}

type createNotifyRequest struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Config           string `json:"config"`
	Events           string `json:"events,omitempty"`
	MaxErrorsPerHour int    `json:"maxErrorsPerHour,omitempty"`
	TimeoutMs        int    `json:"timeoutMs,omitempty"`
	QuietHoursStart  string `json:"quietHoursStart,omitempty"`
	QuietHoursEnd    string `json:"quietHoursEnd,omitempty"`
	MessageTemplate  string `json:"messageTemplate,omitempty"`
}

type updateNotifyRequest struct {
	Type             *string `json:"type,omitempty"`
	Name             *string `json:"name,omitempty"`
	Enabled          *bool   `json:"enabled,omitempty"`
	Config           *string `json:"config,omitempty"`
	Events           *string `json:"events,omitempty"`
	MaxErrorsPerHour *int    `json:"maxErrorsPerHour,omitempty"`
	TimeoutMs        *int    `json:"timeoutMs,omitempty"`
	QuietHoursStart  *string `json:"quietHoursStart,omitempty"`
	QuietHoursEnd    *string `json:"quietHoursEnd,omitempty"`
	MessageTemplate  *string `json:"messageTemplate,omitempty"`
}

type notifyResponse struct {
	ID               uint      `json:"id"`
	Type             string    `json:"type"`
	Name             string    `json:"name"`
	Enabled          bool      `json:"enabled"`
	Healthy          bool      `json:"healthy"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Events           string    `json:"events,omitempty"`
	MaxErrorsPerHour int       `json:"maxErrorsPerHour"`
	TimeoutMs        int       `json:"timeoutMs"`
	QuietHoursStart  string    `json:"quietHoursStart,omitempty"`
	QuietHoursEnd    string    `json:"quietHoursEnd,omitempty"`
	MessageTemplate  string    `json:"messageTemplate,omitempty"`
	HasConfig        bool      `json:"hasConfig"`
}

func (h *NotifyHandler) toResponse(ch *model.NotificationChannel) notifyResponse {
	return notifyResponse{
		ID:               ch.ID,
		Type:             ch.Type,
		Name:             ch.Name,
		Enabled:          ch.Enabled,
		Healthy:          ch.Healthy,
		CreatedAt:        ch.CreatedAt,
		UpdatedAt:        ch.UpdatedAt,
		Events:           ch.Events,
		MaxErrorsPerHour: ch.MaxErrorsPerHour,
		TimeoutMs:        ch.TimeoutMs,
		QuietHoursStart:  ch.QuietHoursStart,
		QuietHoursEnd:    ch.QuietHoursEnd,
		MessageTemplate:  ch.MessageTemplate,
		HasConfig:        ch.Config != "",
	}
}

func (h *NotifyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleRouteByPath(w, r)
}

func (h *NotifyHandler) handleRouteByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if trimmed == "/api/v1/notifications/channels" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
			return
		}
		if r.Method == http.MethodPost {
			h.handleCreate(w, r)
			return
		}
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/notifications/channels/")
	if remaining == "" || remaining == "/" {
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	remaining = strings.TrimPrefix(path, "/api/v1/notifications/channels/")
	remaining = strings.TrimRight(remaining, "/")
	parts := strings.SplitN(remaining, "/", 2)

	switch r.Method {
	case http.MethodGet:
		if len(parts) == 2 && parts[1] == "history" {
			h.handleHistory(w, r, parts[0])
		} else {
			h.handleGet(w, r, parts[0])
		}
	case http.MethodPut:
		h.handleUpdate(w, r, parts[0])
	case http.MethodDelete:
		h.handleDelete(w, r, parts[0])
	case http.MethodPost:
		if len(parts) == 2 && parts[1] == "test" {
			h.handleTest(w, r, parts[0])
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
	}
}

func (h *NotifyHandler) handleList(w http.ResponseWriter, r *http.Request) {
	channels, err := h.repo.List(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询通道失败")
		return
	}

	items := make([]notifyResponse, 0, len(channels))
	for i := range channels {
		items = append(items, h.toResponse(&channels[i]))
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": len(items),
	})
}

func (h *NotifyHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createNotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name == "" || req.Type == "" {
		Error(w, http.StatusBadRequest, 40001, "name, type 为必填项")
		return
	}
	if !validNotifyTypes[req.Type] {
		Error(w, http.StatusBadRequest, 40001, "type 必须为 telegram/bark/webhook/serverchan/dingtalk")
		return
	}

	exists, _ := h.repo.ExistsByName(r.Context(), req.Name, 0)
	if exists {
		Error(w, http.StatusConflict, 40900, "通道名称已存在")
		return
	}

	maxErrors := req.MaxErrorsPerHour
	if maxErrors == 0 {
		maxErrors = 100
	}
	timeout := req.TimeoutMs
	if timeout == 0 {
		timeout = 10000
	}

	ch := &model.NotificationChannel{
		Type:             req.Type,
		Name:             req.Name,
		Enabled:          req.Enabled,
		Config:           req.Config,
		Events:           req.Events,
		MaxErrorsPerHour: maxErrors,
		TimeoutMs:        timeout,
		QuietHoursStart:  req.QuietHoursStart,
		QuietHoursEnd:    req.QuietHoursEnd,
		MessageTemplate:  req.MessageTemplate,
		Healthy:          true,
	}

	if err := h.repo.Create(r.Context(), ch); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建通道失败")
		return
	}

	h.logger.Info("notification channel created", zap.String("name", ch.Name), zap.String("type", ch.Type))
	Success(w, h.toResponse(ch))
}

func (h *NotifyHandler) handleGet(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseNotifyUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的通道 ID")
		return
	}

	ch, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 15001, "通道不存在")
		return
	}

	Success(w, h.toResponse(ch))
}

func (h *NotifyHandler) handleUpdate(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseNotifyUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的通道 ID")
		return
	}

	ch, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 15001, "通道不存在")
		return
	}

	var req updateNotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name != nil {
		exists, _ := h.repo.ExistsByName(r.Context(), *req.Name, id)
		if exists {
			Error(w, http.StatusConflict, 40900, "通道名称已存在")
			return
		}
		ch.Name = *req.Name
	}
	if req.Type != nil {
		if !validNotifyTypes[*req.Type] {
			Error(w, http.StatusBadRequest, 40001, "不支持的 type")
			return
		}
		ch.Type = *req.Type
	}
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	if req.Config != nil {
		ch.Config = *req.Config
	}
	if req.Events != nil {
		ch.Events = *req.Events
	}
	if req.MaxErrorsPerHour != nil {
		ch.MaxErrorsPerHour = *req.MaxErrorsPerHour
	}
	if req.TimeoutMs != nil {
		ch.TimeoutMs = *req.TimeoutMs
	}
	if req.QuietHoursStart != nil {
		ch.QuietHoursStart = *req.QuietHoursStart
	}
	if req.QuietHoursEnd != nil {
		ch.QuietHoursEnd = *req.QuietHoursEnd
	}
	if req.MessageTemplate != nil {
		ch.MessageTemplate = *req.MessageTemplate
	}

	if err := h.repo.Update(r.Context(), ch); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新通道失败")
		return
	}

	h.logger.Info("notification channel updated", zap.String("name", ch.Name))
	Success(w, h.toResponse(ch))
}

func (h *NotifyHandler) handleDelete(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseNotifyUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的通道 ID")
		return
	}

	_, err = h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 15001, "通道不存在")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除通道失败")
		return
	}

	h.logger.Info("notification channel deleted", zap.Uint("id", id))
	Success(w, nil)
}

func (h *NotifyHandler) handleTest(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseNotifyUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的通道 ID")
		return
	}

	ch, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 15001, "通道不存在")
		return
	}

	msg := model.FormattedMessage{
		Title:   "PT-Forward 测试通知",
		Message: fmt.Sprintf("通道 %s 测试消息（%s）", ch.Name, time.Now().Format("2006-01-02 15:04:05")),
		Level:   "info",
	}

	testService := notification.NewTestService(ch, h.logger)
	ok, message := testService.SendTest(r.Context(), msg)
	Success(w, map[string]interface{}{
		"ok":      ok,
		"message": message,
	})
}

func (h *NotifyHandler) handleHistory(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseNotifyUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的通道 ID")
		return
	}

	history, err := h.repo.ListHistory(r.Context(), id, 50)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询历史失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": history,
		"total": len(history),
	})
}

func parseNotifyUint(s string) (uint, error) {
	var n uint
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
