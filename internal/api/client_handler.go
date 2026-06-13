package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/client/qbittorrent"
	"github.com/ranfish/pt-forward/internal/client/transmission"
	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/middleware"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ClientHandler struct {
	db        *gorm.DB
	logger    *zap.Logger
	clientMgr ClientManager
}

type ClientManager interface {
	Get(clientID string) (model.DownloaderClient, error)
	Reload(ctx context.Context) error
	ConnectedCount() int
	ListClients() []string
}

func NewClientHandler(db *gorm.DB, logger *zap.Logger, clientMgr ClientManager) *ClientHandler {
	return &ClientHandler{db: db, logger: logger, clientMgr: clientMgr}
}

type createDownloaderRequest struct {
	Name           string               `json:"name"`
	Type           string               `json:"type"`
	URL            string               `json:"url"`
	Username       string               `json:"username"`
	Password       string               `json:"password"`
	Role           string               `json:"role"`
	ReseedTargetID string               `json:"reseedTargetId"`
	Enabled        bool                 `json:"enabled"`
	IsDefault      bool                 `json:"isDefault"`
	PathMappings   []pathMappingRequest `json:"pathMappings"`
}

type pathMappingRequest struct {
	SourcePath string `json:"sourcePath"`
	ReseedPath string `json:"reseedPath"`
}

type downloaderResponse struct {
	ID             uint                 `json:"id"`
	Name           string               `json:"name"`
	Type           string               `json:"type"`
	URL            string               `json:"url"`
	Username       string               `json:"username"`
	Role           string               `json:"role"`
	ReseedTargetID string               `json:"reseedTargetId,omitempty"`
	Enabled        bool                 `json:"enabled"`
	IsDefault      bool                 `json:"isDefault"`
	PathMappings   []pathMappingRequest `json:"pathMappings"`
	DownloadSpeed  int64                `json:"downloadSpeed"`
	UploadSpeed    int64                `json:"uploadSpeed"`
	FreeSpace      int64                `json:"freeSpace"`
	TotalDiskSpace int64                `json:"totalDiskSpace"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

func (h *ClientHandler) toResponse(c *model.ClientConfig, mappings []model.ClientPathMapping) downloaderResponse {
	resp := downloaderResponse{
		ID:             c.ID,
		Name:           c.Name,
		Type:           c.Type,
		URL:            c.URL,
		Username:       c.Username,
		Role:           c.Role,
		ReseedTargetID: c.ReseedTargetID,
		Enabled:        c.Enabled,
		IsDefault:      c.IsDefault,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
	for _, m := range mappings {
		resp.PathMappings = append(resp.PathMappings, pathMappingRequest{
			SourcePath: m.SourcePath,
			ReseedPath: m.ReseedPath,
		})
	}
	if resp.PathMappings == nil {
		resp.PathMappings = []pathMappingRequest{}
	}
	return resp
}

func (h *ClientHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	page, size := parsePagination(r)
	light := r.URL.Query().Get("light") == "true"

	var total int64
	h.db.Model(&model.ClientConfig{}).Count(&total)

	var clients []model.ClientConfig
	h.db.
		Offset(offset(page, size)).Limit(size).
		Find(&clients)

	items := make([]downloaderResponse, 0, len(clients))
	for i := range clients {
		var mappings []model.ClientPathMapping
		if err := h.db.Where("source_client_id = ? OR reseed_client_id = ?", clients[i].ID, clients[i].ID).Find(&mappings).Error; err != nil {
			h.logger.Error("query path mappings failed", zap.Uint("clientID", clients[i].ID), zap.Error(err))
			continue
		}
		resp := h.toResponse(&clients[i], mappings)
		items = append(items, resp)
	}

	if !light {
		var wg sync.WaitGroup
		for i := range items {
			if !clients[i].Enabled || h.clientMgr == nil {
				continue
			}
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				name := clients[idx].Name
				dlClient, err := h.clientMgr.Get(name)
				if err != nil {
					return
				}
				md, mdErr := dlClient.GetMainData(r.Context())
				if mdErr != nil || md == nil {
					return
				}
				items[idx].DownloadSpeed = md.ServerState.DownloadSpeed
				items[idx].UploadSpeed = md.ServerState.UploadSpeed
				items[idx].FreeSpace = md.FreeSpace
				items[idx].TotalDiskSpace = md.TotalDiskSpace
			}(i)
		}
		wg.Wait()
	}

	Success(w, PaginatedResult{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func (h *ClientHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	id, err := extractIDFromPath(r.URL.Path, "/api/v1/downloaders/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	var client model.ClientConfig
	if err := h.db.First(&client, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(w, http.StatusNotFound, 11001, "下载器不存在")
			return
		}
		Error(w, http.StatusInternalServerError, 50000, "查询下载器失败")
		return
	}

	var mappings []model.ClientPathMapping
	if err := h.db.Where("source_client_id = ? OR reseed_client_id = ?", client.ID, client.ID).Find(&mappings).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询路径映射失败")
		return
	}

	Success(w, h.toResponse(&client, mappings))
}

func (h *ClientHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	var req createDownloaderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name == "" || req.Type == "" || req.URL == "" || req.Role == "" {
		Error(w, http.StatusBadRequest, 40001, "name, type, url, role 为必填项")
		return
	}
	if err := middleware.ValidateSafeURL(req.URL); err != nil {
		Error(w, http.StatusBadRequest, 40001, "url 不合法: "+err.Error())
		return
	}
	if req.Type != "qbittorrent" && req.Type != "transmission" {
		Error(w, http.StatusBadRequest, 40001, "type 必须为 qbittorrent 或 transmission")
		return
	}

	validRoles := map[string]bool{"seeding": true, "download": true, "source": true, "reseed": true}
	if !validRoles[req.Role] {
		Error(w, http.StatusBadRequest, 40001, "role 必须为 seeding/download/source/reseed")
		return
	}

	var existing model.ClientConfig
	if h.db.Where("name = ?", req.Name).First(&existing).Error == nil {
		Error(w, http.StatusConflict, 40900, "下载器名称已存在")
		return
	}

	client := model.ClientConfig{
		Name:           req.Name,
		Type:           req.Type,
		URL:            req.URL,
		Username:       req.Username,
		Password:       req.Password,
		Enabled:        req.Enabled,
		IsDefault:      req.IsDefault,
		Role:           req.Role,
		ReseedTargetID: req.ReseedTargetID,
	}
	if err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := dbimpl.ForceCreateTx(tx, &client); err != nil {
			return err
		}
		for _, pm := range req.PathMappings {
			mapping := model.ClientPathMapping{
				SourceClientID: client.ID,
				ReseedClientID: client.ID,
				SourcePath:     pm.SourcePath,
				ReseedPath:     pm.ReseedPath,
			}
			if err := tx.Create(&mapping).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建下载器失败")
		return
	}

	h.logger.Info("downloader created", zap.String("name", client.Name), zap.String("type", client.Type))

	auditLog(r, "client", "create", "client", fmt.Sprintf("%d", client.ID), client.Name, "success")

	var mappings []model.ClientPathMapping
	if err := h.db.Where("source_client_id = ?", client.ID).Find(&mappings).Error; err != nil {
		h.logger.Error("query path mappings failed", zap.Uint("clientID", client.ID), zap.Error(err))
	}
	Success(w, h.toResponse(&client, mappings))
}

func (h *ClientHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	id, err := extractIDFromPath(r.URL.Path, "/api/v1/downloaders/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	var client model.ClientConfig
	if err := h.db.First(&client, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(w, http.StatusNotFound, 11001, "下载器不存在")
			return
		}
		Error(w, http.StatusInternalServerError, 50000, "查询下载器失败")
		return
	}

	var req createDownloaderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name != "" {
		client.Name = req.Name
	}
	if req.Type != "" {
		if req.Type != "qbittorrent" && req.Type != "transmission" {
			Error(w, http.StatusBadRequest, 40001, "type 必须为 qbittorrent 或 transmission")
			return
		}
		client.Type = req.Type
	}
	if req.URL != "" {
		if err := middleware.ValidateSafeURL(req.URL); err != nil {
			Error(w, http.StatusBadRequest, 40001, "url 不合法: "+err.Error())
			return
		}
		client.URL = req.URL
	}
	if req.Username != "" {
		client.Username = req.Username
	}
	if req.Password != "" {
		client.Password = req.Password
	}
	if req.Role != "" {
		validRoles := map[string]bool{"seeding": true, "download": true, "source": true, "reseed": true}
		if !validRoles[req.Role] {
			Error(w, http.StatusBadRequest, 40001, "role 必须为 seeding, download, source 或 reseed")
			return
		}
		client.Role = req.Role
	}
	client.ReseedTargetID = req.ReseedTargetID
	client.Enabled = req.Enabled
	client.IsDefault = req.IsDefault

	if err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&client).Updates(map[string]interface{}{
			"name":             client.Name,
			"type":             client.Type,
			"url":              client.URL,
			"username":         client.Username,
			"password":         client.Password,
			"role":             client.Role,
			"reseed_target_id": client.ReseedTargetID,
			"enabled":          client.Enabled,
			"is_default":       client.IsDefault,
		}).Error; err != nil {
			return err
		}
		if req.PathMappings != nil {
			if err := tx.Where("source_client_id = ?", client.ID).Delete(&model.ClientPathMapping{}).Error; err != nil {
				return err
			}
			for _, pm := range req.PathMappings {
				mapping := model.ClientPathMapping{
					SourceClientID: client.ID,
					ReseedClientID: client.ID,
					SourcePath:     pm.SourcePath,
					ReseedPath:     pm.ReseedPath,
				}
				if err := tx.Create(&mapping).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新下载器失败")
		return
	}

	h.logger.Info("downloader updated", zap.String("name", client.Name))

	auditLog(r, "client", "update", "client", fmt.Sprintf("%d", id), client.Name, "success")

	var mappings []model.ClientPathMapping
	if err := h.db.Where("source_client_id = ?", client.ID).Find(&mappings).Error; err != nil {
		h.logger.Error("query path mappings failed", zap.Uint("clientID", client.ID), zap.Error(err))
	}
	Success(w, h.toResponse(&client, mappings))
}

func (h *ClientHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	id, err := extractIDFromPath(r.URL.Path, "/api/v1/downloaders/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	var client model.ClientConfig
	if err := h.db.First(&client, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(w, http.StatusNotFound, 11001, "下载器不存在")
			return
		}
		Error(w, http.StatusInternalServerError, 50000, "查询下载器失败")
		return
	}

	if err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_client_id = ? OR reseed_client_id = ?", client.ID, client.ID).Delete(&model.ClientPathMapping{}).Error; err != nil {
			return err
		}
		return tx.Delete(&client).Error
	}); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除下载器失败")
		return
	}

	h.logger.Info("downloader deleted", zap.String("name", client.Name))
	auditLog(r, "client", "delete", "client", fmt.Sprintf("%d", id), client.Name, "success")
	Success(w, nil)
}

func (h *ClientHandler) HandleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	id, err := extractIDFromPath(r.URL.Path, "/api/v1/downloaders/", "/test")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	var client model.ClientConfig
	if err := h.db.First(&client, id).Error; err != nil {
		Error(w, http.StatusNotFound, 11001, "下载器不存在")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ok, message, version := h.testConnection(ctx, &client)
	if !ok {
		Error(w, http.StatusBadGateway, 11003, message)
		return
	}
	Success(w, map[string]interface{}{
		"ok":      ok,
		"message": message,
		"version": version,
	})
}

func (h *ClientHandler) HandleTorrents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	id, err := extractIDFromPath(r.URL.Path, "/api/v1/downloaders/", "/torrents")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	dlClient, err := h.buildClient(r.Context(), id)
	if err != nil {
		writeAppError(w, err)
		return
	}

	torrents, err := dlClient.GetSeedingTorrents(r.Context())
	if err != nil {
		writeAppError(w, err)
		return
	}

	Success(w, map[string]interface{}{
		"items": torrents,
		"total": len(torrents),
	})
}

func (h *ClientHandler) HandleFreeSpace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	id, err := extractIDFromPath(r.URL.Path, "/api/v1/downloaders/", "/free-space")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	dlClient, err := h.buildClient(r.Context(), id)
	if err != nil {
		writeAppError(w, err)
		return
	}

	freeSpace, err := dlClient.GetFreeSpace(r.Context())
	if err != nil {
		writeAppError(w, err)
		return
	}

	totalDiskSpace := int64(0)
	if md, mdErr := dlClient.GetMainData(r.Context()); mdErr == nil && md != nil {
		totalDiskSpace = md.TotalDiskSpace
	}

	Success(w, map[string]interface{}{
		"freeSpace":      freeSpace,
		"totalDiskSpace": totalDiskSpace,
	})
}

func (h *ClientHandler) testConnection(ctx context.Context, cfg *model.ClientConfig) (bool, string, string) {
	switch cfg.Type {
	case "qbittorrent":
		c, err := qbittorrent.NewQBClient(cfg, nil, h.logger)
		if err != nil {
			return false, err.Error(), ""
		}
		if err := c.Connect(ctx); err != nil {
			return false, err.Error(), ""
		}
		return true, "连接成功", c.Version()
	case "transmission":
		c, err := transmission.NewTRClient(cfg, nil, h.logger)
		if err != nil {
			return false, err.Error(), ""
		}
		if err := c.Connect(ctx); err != nil {
			return false, err.Error(), ""
		}
		return true, "连接成功", ""
	default:
		return false, "不支持的下载器类型: " + cfg.Type, ""
	}
}

func (h *ClientHandler) buildClient(ctx context.Context, id uint) (model.DownloaderClient, error) {
	var cfg model.ClientConfig
	if err := h.db.First(&cfg, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &model.AppError{Code: 11001, Message: "下载器不存在"}
		}
		return nil, &model.AppError{Code: 11002, Message: "查询下载器失败"}
	}

	if h.clientMgr != nil {
		if pooled, err := h.clientMgr.Get(cfg.Name); err == nil {
			return pooled, nil
		}
	}

	var mappings []model.ClientPathMapping
	if err := h.db.Where("source_client_id = ?", cfg.ID).Find(&mappings).Error; err != nil {
		return nil, &model.AppError{Code: 11002, Message: "查询路径映射失败"}
	}

	sharedPaths := make([]model.SharedPathMapping, 0, len(mappings))
	for _, m := range mappings {
		sharedPaths = append(sharedPaths, model.SharedPathMapping{
			SourcePath: m.SourcePath,
			ReseedPath: m.ReseedPath,
		})
	}

	switch cfg.Type {
	case "qbittorrent":
		c, err := qbittorrent.NewQBClient(&cfg, sharedPaths, h.logger)
		if err != nil {
			return nil, &model.AppError{Code: 11002, Message: "创建 qBittorrent 客户端失败: " + err.Error()}
		}
		if err := c.Connect(ctx); err != nil {
			return nil, err
		}
		return c, nil
	case "transmission":
		c, err := transmission.NewTRClient(&cfg, sharedPaths, h.logger)
		if err != nil {
			return nil, &model.AppError{Code: 11002, Message: "创建 Transmission 客户端失败: " + err.Error()}
		}
		if err := c.Connect(ctx); err != nil {
			return nil, err
		}
		return c, nil
	default:
		return nil, &model.AppError{Code: 11002, Message: "不支持的下载器类型: " + cfg.Type}
	}
}

func extractIDFromPath(path string, prefix string, suffixes ...string) (uint, error) {
	p := strings.TrimPrefix(path, prefix)
	for _, s := range suffixes {
		p = strings.TrimSuffix(p, s)
	}
	p = strings.TrimSuffix(p, "/")
	n, err := strconv.ParseUint(p, 10, 32)
	if err != nil {
		return 0, apiError(ErrAPIInvalidID, "invalid id", nil)
	}
	return uint(n), nil
}

func (h *ClientHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.HandleRouteByPath(w, r)
}

func (h *ClientHandler) HandleRouteByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if trimmed == "/api/v1/downloaders" {
		if r.Method == http.MethodGet {
			h.HandleList(w, r)
			return
		}
		if r.Method == http.MethodPost {
			h.HandleCreate(w, r)
			return
		}
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许 dl")
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/downloaders/")
	if remaining == "" || remaining == "/" {
		switch r.Method {
		case http.MethodGet:
			h.HandleList(w, r)
		case http.MethodPost:
			h.HandleCreate(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	remaining = strings.TrimPrefix(path, "/api/v1/downloaders/")
	remaining = strings.TrimRight(remaining, "/")
	parts := strings.SplitN(remaining, "/", 3)

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.HandleGet(w, r)
		case http.MethodPut:
			h.HandleUpdate(w, r)
		case http.MethodDelete:
			h.HandleDelete(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	idStr := parts[0]
	subResource := parts[1]

	switch subResource {
	case "test":
		r.URL.Path = "/api/v1/downloaders/" + idStr + "/test"
		h.HandleTest(w, r)
	case "torrents":
		if len(parts) == 3 && parts[2] != "" {
			subParts := strings.SplitN(parts[2], "/", 2)
			switch {
			case len(subParts) == 2 && subParts[1] == "pause":
				h.handleTorrentAction(w, r, idStr, "pause")
			case len(subParts) == 2 && subParts[1] == "resume":
				h.handleTorrentAction(w, r, idStr, "resume")
			default:
				h.handleTorrentInfo(w, r, idStr, subParts[0])
			}
		} else {
			r.URL.Path = "/api/v1/downloaders/" + idStr + "/torrents"
			h.HandleTorrents(w, r)
		}
	case "free-space":
		r.URL.Path = "/api/v1/downloaders/" + idStr + "/free-space"
		h.HandleFreeSpace(w, r)
	case "maindata":
		h.handleMaindata(w, r, idStr)
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

func (h *ClientHandler) handleTorrentAction(w http.ResponseWriter, r *http.Request, idStr string, action string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/v1/downloaders/"+idStr+"/torrents/"), "/"+action, 2)
	infoHash := parts[0]

	dlClient, err := h.buildClient(r.Context(), uint(id))
	if err != nil {
		writeAppError(w, err)
		return
	}

	switch action {
	case "pause":
		err = dlClient.PauseTorrent(r.Context(), infoHash)
	case "resume":
		err = dlClient.ResumeTorrent(r.Context(), infoHash)
	}
	if err != nil {
		writeAppError(w, err)
		return
	}
	Success(w, nil)
}

func (h *ClientHandler) handleTorrentInfo(w http.ResponseWriter, r *http.Request, idStr string, infoHash string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}

	if r.Method == http.MethodDelete {
		dlClient, err := h.buildClient(r.Context(), uint(id))
		if err != nil {
			writeAppError(w, err)
			return
		}
		deleteFiles := r.URL.Query().Get("deleteFiles") == "true"
		if err := dlClient.DeleteTorrent(r.Context(), infoHash, deleteFiles); err != nil {
			writeAppError(w, err)
			return
		}
		Success(w, nil)
		return
	}

	dlClient, err := h.buildClient(r.Context(), uint(id))
	if err != nil {
		writeAppError(w, err)
		return
	}
	info, err := dlClient.GetTorrentByHash(r.Context(), infoHash)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if info == nil {
		Error(w, http.StatusNotFound, 11005, "种子不存在")
		return
	}
	Success(w, info)
}

func (h *ClientHandler) handleMaindata(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的下载器 ID")
		return
	}
	dlClient, err := h.buildClient(r.Context(), uint(id))
	if err != nil {
		writeAppError(w, err)
		return
	}
	md, err := dlClient.GetMainData(r.Context())
	if err != nil {
		writeAppError(w, err)
		return
	}
	Success(w, md)
}

func (h *ClientHandler) handlePublishTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var targets []model.ClientPublishTarget
		if err := h.db.WithContext(r.Context()).Find(&targets).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "failed to list publish targets")
			return
		}
		Success(w, targets)

	case http.MethodPost:
		var req struct {
			ClientID        uint   `json:"client_id"`
			SiteName        string `json:"site_name"`
			CategoryMapping string `json:"category_mapping"`
			SourceMapping   string `json:"source_mapping"`
			CodecMapping    string `json:"codec_mapping"`
			AutoPublish     bool   `json:"auto_publish"`
			NotifyOnPublish bool   `json:"notify_on_publish"`
			Enabled         bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		if req.ClientID == 0 || req.SiteName == "" {
			Error(w, http.StatusBadRequest, 40001, "client_id and site_name are required")
			return
		}
		target := model.ClientPublishTarget{
			ClientID:        req.ClientID,
			SiteName:        req.SiteName,
			CategoryMapping: req.CategoryMapping,
			SourceMapping:   req.SourceMapping,
			CodecMapping:    req.CodecMapping,
			AutoPublish:     req.AutoPublish,
			NotifyOnPublish: req.NotifyOnPublish,
			Enabled:         req.Enabled,
		}
		if err := dbimpl.ForceCreate(h.db.WithContext(r.Context()), &target); err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
				Error(w, http.StatusConflict, 40900, "publish target already exists")
				return
			}
			Error(w, http.StatusInternalServerError, 50000, "failed to create publish target")
			return
		}
		Success(w, target)

	case http.MethodPut:
		var req struct {
			ID              uint   `json:"id"`
			CategoryMapping string `json:"category_mapping"`
			SourceMapping   string `json:"source_mapping"`
			CodecMapping    string `json:"codec_mapping"`
			AutoPublish     *bool  `json:"auto_publish"`
			NotifyOnPublish *bool  `json:"notify_on_publish"`
			Enabled         *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		var target model.ClientPublishTarget
		if err := h.db.WithContext(r.Context()).First(&target, req.ID).Error; err != nil {
			Error(w, http.StatusNotFound, 40400, "publish target not found")
			return
		}
		updates := map[string]interface{}{}
		if req.CategoryMapping != "" {
			updates["category_mapping"] = req.CategoryMapping
		}
		if req.SourceMapping != "" {
			updates["source_mapping"] = req.SourceMapping
		}
		if req.CodecMapping != "" {
			updates["codec_mapping"] = req.CodecMapping
		}
		if req.AutoPublish != nil {
			updates["auto_publish"] = *req.AutoPublish
		}
		if req.NotifyOnPublish != nil {
			updates["notify_on_publish"] = *req.NotifyOnPublish
		}
		if req.Enabled != nil {
			updates["enabled"] = *req.Enabled
		}
		if len(updates) > 0 {
			if err := h.db.WithContext(r.Context()).Model(&target).Updates(updates).Error; err != nil {
				Error(w, http.StatusInternalServerError, 50000, "更新下载器设置失败")
				return
			}
		}
		if err := h.db.WithContext(r.Context()).First(&target, req.ID).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "查询下载器设置失败")
			return
		}
		Success(w, target)

	case http.MethodDelete:
		var req struct {
			ID uint `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		result := h.db.WithContext(r.Context()).Delete(&model.ClientPublishTarget{}, req.ID)
		if result.RowsAffected == 0 {
			Error(w, http.StatusNotFound, 40400, "publish target not found")
			return
		}
		Success(w, map[string]string{"status": "deleted"})

	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "method not allowed")
	}
}
