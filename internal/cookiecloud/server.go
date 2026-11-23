package cookiecloud

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Server struct {
	syncService *SyncService
	db          *gorm.DB
	logger      *zap.Logger

	store         sync.Map
	debounceMu    sync.Mutex
	debounceTimer *time.Timer
}

func NewServer(syncService *SyncService, db *gorm.DB, logger *zap.Logger) *Server {
	return &Server{
		syncService: syncService,
		db:          db,
		logger:      logger.With(zap.String("component", "cookiecloud-server")),
	}
}

func (s *Server) RestoreFromDB(ctx context.Context) {
	var cfg model.CookieCloudConfig
	if err := s.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return
	}
	if cfg.LastEncrypted != "" {
		s.store.Store(cfg.UUID, cfg.LastEncrypted)
		s.logger.Info("cookiecloud server restored from DB",
			zap.Int("encrypted_len", len(cfg.LastEncrypted)))
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/cookiecloud/update") && r.Method == http.MethodPost:
		s.handleUpdate(w, r)
	case strings.Contains(path, "/cookiecloud/get/") && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		s.handleGet(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	body := r.Body
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
		gzReader, err := gzip.NewReader(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"action":"error","error":"gzip decode failed"}`))
			return
		}
		defer func() { _ = gzReader.Close() }()
		body = gzReader
	}

	data, err := io.ReadAll(io.LimitReader(body, 50*1024*1024))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"action":"error","error":"read body failed"}`))
		return
	}

	var req struct {
		Encrypted  string `json:"encrypted"`
		UUID       string `json:"uuid"`
		CryptoType string `json:"crypto_type"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"action":"error","error":"invalid json"}`))
		return
	}
	if req.Encrypted == "" || req.UUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"action":"error","error":"missing encrypted or uuid"}`))
		return
	}

	// §59.542 追记十（V-01 整改路线 1）: 写入端绑定配置行——与消费端
	// （RestoreFromDB/doSync/SyncAll 全部绑定 cfg.UUID/cfg.Password）对齐。
	// 历史缺陷：store.Store(req.UUID) 信任请求体寻址 + Where("1 = 1") 全表
	// 写 last_encrypted——盲写请求即可污染持久化数据（uuid 不匹配时推送链
	// 暂存活但手动同步立挂·重启恢复后推送链也挂；解密失败发生在写站点之前
	// 故不直接注入错误 cookie·定性=数据污染+同步可用性受损）。
	var cfg model.CookieCloudConfig
	if err := s.db.WithContext(r.Context()).First(&cfg).Error; err != nil {
		// 无配置记录：拒绝写入（不写 store/DB·不触发同步）
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"action":"error","error":"cookiecloud not configured"}`))
		return
	}
	if cfg.Mode != "builtin" {
		// 非 builtin 模式（remote 等）：内置服务端不应接收推送
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"action":"error","error":"cookiecloud mode is not builtin"}`))
		return
	}
	if req.UUID != cfg.UUID {
		// 推送 uuid 与配置不一致：显式拒收并告警（识别异常来源）
		s.logger.Warn("cookiecloud push rejected: uuid mismatch",
			zap.Int("req_uuid_len", len(req.UUID)),
			zap.Int("cfg_uuid_len", len(cfg.UUID)))
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"action":"error","error":"uuid mismatch"}`))
		return
	}

	// 存储与持久化统一以配置行为准（对齐 RestoreFromDB/doSync 消费约定）
	s.store.Store(cfg.UUID, req.Encrypted)
	// 按主键精确更新（替代 Where("1 = 1") 全表写）
	s.db.WithContext(r.Context()).Model(&cfg).
		Update("last_encrypted", req.Encrypted)

	s.logger.Info("cookiecloud push received",
		zap.String("uuid", req.UUID),
		zap.Int("encrypted_len", len(req.Encrypted)),
		zap.String("crypto_type", req.CryptoType))

	s.scheduleDebouncedSync()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"action":"done"}`))
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	uuid := parts[len(parts)-1]
	if uuid == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"missing uuid"}`))
		return
	}

	val, ok := s.store.Load(uuid)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
		return
	}

	encrypted, _ := val.(string)
	resp := map[string]string{
		"encrypted":   encrypted,
		"crypto_type": "legacy",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) scheduleDebouncedSync() {
	s.debounceMu.Lock()
	defer s.debounceMu.Unlock()

	if s.debounceTimer != nil {
		s.debounceTimer.Stop()
	}

	s.debounceTimer = time.AfterFunc(30*time.Second, func() {
		s.doSync()
	})
}

func (s *Server) doSync() {
	var cfg model.CookieCloudConfig
	if err := s.db.First(&cfg).Error; err != nil {
		s.logger.Warn("cookiecloud sync: config not found", zap.Error(err))
		return
	}

	if cfg.Mode != "builtin" {
		return
	}

	val, ok := s.store.Load(cfg.UUID)
	if !ok {
		s.logger.Debug("cookiecloud sync: no data in store")
		return
	}

	encrypted, _ := val.(string)
	if encrypted == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, err := s.syncService.SyncFromEncrypted(ctx, encrypted)
	if err != nil {
		s.logger.Warn("cookiecloud builtin sync failed", zap.Error(err))
	}
}
