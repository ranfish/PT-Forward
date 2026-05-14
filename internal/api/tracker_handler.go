package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TrackerHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewTrackerHandler(db *gorm.DB, logger *zap.Logger) *TrackerHandler {
	return &TrackerHandler{db: db, logger: logger}
}

func (h *TrackerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case strings.HasSuffix(trimmed, "/tracker/members") || strings.HasSuffix(trimmed, "/tracker/members/"):
		if r.Method == http.MethodGet {
			h.handleListMembers(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case strings.HasSuffix(trimmed, "/tracker/history"):
		if r.Method == http.MethodGet {
			h.handleListHistory(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if strings.Contains(trimmed, "/tracker/members/") {
		remaining := strings.TrimPrefix(trimmed, "/api/v1/tracker/members/")
		remaining = strings.TrimRight(remaining, "/")
		if remaining != "" {
			h.handleGetMember(w, r, remaining)
			return
		}
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *TrackerHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	q := h.db.Model(&model.PublishGroupMember{})
	if groupID := r.URL.Query().Get("groupId"); groupID != "" {
		q = q.Where("publish_group_id = ?", groupID)
	}

	var total int64
	q.Count(&total)

	var members []model.PublishGroupMember
	if err := q.Order("created_at DESC").Limit(100).Find(&members).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询成员列表失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": members,
		"total": total,
	})
}

func (h *TrackerHandler) handleGetMember(w http.ResponseWriter, r *http.Request, hash string) {
	var members []model.PublishGroupMember
	if err := h.db.Where("info_hash = ?", hash).Find(&members).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询成员失败")
		return
	}
	if len(members) == 0 {
		Error(w, http.StatusNotFound, 40400, "成员不存在")
		return
	}

	var histories []model.PublishGroupStatusHistory
	for _, m := range members {
		var h2 []model.PublishGroupStatusHistory
		if err := h.db.Where("member_hash = ?", m.InfoHash).Order("created_at DESC").Limit(20).Find(&h2).Error; err != nil {
			continue
		}
		histories = append(histories, h2...)
	}

	Success(w, map[string]interface{}{
		"members": members,
		"history": histories,
	})
}

func (h *TrackerHandler) handleListHistory(w http.ResponseWriter, r *http.Request) {
	q := h.db.Model(&model.PublishGroupStatusHistory{})
	if groupID := r.URL.Query().Get("groupId"); groupID != "" {
		q = q.Where("publish_group_id = ?", groupID)
	}

	var total int64
	q.Count(&total)

	var histories []model.PublishGroupStatusHistory
	if err := q.Order("created_at DESC").Limit(100).Find(&histories).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询历史记录失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": histories,
		"total": total,
	})
}

type LifecycleHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewLifecycleHandler(db *gorm.DB, logger *zap.Logger) *LifecycleHandler {
	return &LifecycleHandler{db: db, logger: logger}
}

func (h *LifecycleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case strings.HasSuffix(trimmed, "/lifecycle/config"):
		switch r.Method {
		case http.MethodGet:
			h.handleGetConfig(w, r)
		case http.MethodPut:
			h.handleUpdateConfig(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case strings.HasSuffix(trimmed, "/lifecycle/backpressure"):
		switch r.Method {
		case http.MethodGet:
			h.handleBackpressure(w, r)
		case http.MethodPut:
			h.handleUpdateBackpressure(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *LifecycleHandler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"pauseSeeders":        true,
		"deleteSeeders":       false,
		"deleteSeedHours":     720,
		"checkInterval":       "5m",
		"maxConcurrentChecks": 10,
	}

	if val, err := h.getSetting(r, "lifecycle.pause_seeders"); err == nil {
		config["pauseSeeders"] = val == "true"
	}
	if val, err := h.getSetting(r, "lifecycle.delete_seeders"); err == nil {
		config["deleteSeeders"] = val == "true"
	}
	if val, err := h.getSetting(r, "lifecycle.delete_seed_hours"); err == nil {
		config["deleteSeedHours"] = val
	}

	Success(w, config)
}

func (h *LifecycleHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if v, ok := req["pauseSeeders"]; ok {
		h.saveSetting(r, "lifecycle.pause_seeders", interfaceToStr(v))
	}
	if v, ok := req["deleteSeeders"]; ok {
		h.saveSetting(r, "lifecycle.delete_seeders", interfaceToStr(v))
	}
	if v, ok := req["deleteSeedHours"]; ok {
		h.saveSetting(r, "lifecycle.delete_seed_hours", interfaceToStr(v))
	}
	if v, ok := req["checkInterval"]; ok {
		h.saveSetting(r, "lifecycle.check_interval", interfaceToStr(v))
	}
	if v, ok := req["maxConcurrentChecks"]; ok {
		h.saveSetting(r, "lifecycle.max_concurrent_checks", interfaceToStr(v))
	}

	Success(w, req)
}

func (h *LifecycleHandler) handleBackpressure(w http.ResponseWriter, _ *http.Request) {
	var activePublishes int64
	h.db.Model(&model.PublishCandidate{}).Where("publish_status = ?", "publishing").Count(&activePublishes)

	maxConcurrent := int64(5)
	if val, err := h.getSettingFromDB("lifecycle.max_concurrent_publishes"); err == nil && val != "" {
		if n, err := parseInt64(val); err == nil {
			maxConcurrent = n
		}
	}

	pauseOnPressure := true
	if val, err := h.getSettingFromDB("lifecycle_backpressure"); err == nil && val != "" {
		var cfg map[string]interface{}
		if json.Unmarshal([]byte(val), &cfg) == nil {
			if v, ok := cfg["max_concurrent"]; ok {
				if n, err := parseInt64(fmt.Sprintf("%.0f", v)); err == nil && n > 0 {
					maxConcurrent = n
				}
			}
			if v, ok := cfg["pause_on_pressure"]; ok {
				if b, ok := v.(bool); ok {
					pauseOnPressure = b
				}
			}
		}
	}

	Success(w, map[string]interface{}{
		"queueDepth":             0,
		"maxQueueDepth":          256,
		"activePublishes":        activePublishes,
		"maxConcurrentPublishes": maxConcurrent,
		"bandwidthLimitKB":       0,
		"isThrottled":            activePublishes >= maxConcurrent,
		"pauseOnPressure":        pauseOnPressure,
	})
}

func (h *LifecycleHandler) handleUpdateBackpressure(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaxConcurrent   *int  `json:"max_concurrent"`
		PauseOnPressure *bool `json:"pause_on_pressure"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	existing := map[string]interface{}{}
	if val, err := h.getSettingFromDB("lifecycle_backpressure"); err == nil && val != "" {
		_ = json.Unmarshal([]byte(val), &existing)
	}

	if req.MaxConcurrent != nil {
		existing["max_concurrent"] = *req.MaxConcurrent
	}
	if req.PauseOnPressure != nil {
		existing["pause_on_pressure"] = *req.PauseOnPressure
	}

	data, _ := json.Marshal(existing)
	h.saveSetting(r, "lifecycle_backpressure", string(data))

	Success(w, existing)
}

func (h *LifecycleHandler) getSetting(r *http.Request, key string) (string, error) {
	var s setting.Setting
	err := h.db.Where("key = ?", key).First(&s).Error
	return s.Value, err
}

func (h *LifecycleHandler) getSettingFromDB(key string) (string, error) {
	var s struct {
		Value string `gorm:"column:value"`
	}
	err := h.db.Table("system_settings").Where("key = ?", key).First(&s).Error
	return s.Value, err
}

func (h *LifecycleHandler) saveSetting(r *http.Request, key, value string) {
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	var s setting.Setting
	result := h.db.WithContext(ctx).Where("`key` = ?", key).First(&s)
	if result.Error != nil {
		if err := h.db.WithContext(ctx).Create(&setting.Setting{Key: key, Value: value}).Error; err != nil {
			h.logger.Error("saveSetting: create failed", zap.String("key", key), zap.Error(err))
		}
	} else {
		if err := h.db.WithContext(ctx).Model(&s).Update("value", value).Error; err != nil {
			h.logger.Error("saveSetting: update failed", zap.String("key", key), zap.Error(err))
		}
	}
}

func interfaceToStr(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		seconds := int64(val)
		if seconds > 0 && seconds < 1e9 {
			return (time.Duration(seconds) * time.Second).String()
		}
		return fmt.Sprintf("%.0f", val)
	default:
		return ""
	}
}
