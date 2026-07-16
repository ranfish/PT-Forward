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

	store       sync.Map
	debounceMu  sync.Mutex
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
		w.Write([]byte(`{"error":"not found"}`))
	}
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	body := r.Body
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
		gzReader, err := gzip.NewReader(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"action":"error","error":"gzip decode failed"}`))
			return
		}
		defer gzReader.Close()
		body = gzReader
	}

	data, err := io.ReadAll(io.LimitReader(body, 50*1024*1024))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"action":"error","error":"read body failed"}`))
		return
	}

	var req struct {
		Encrypted  string `json:"encrypted"`
		UUID       string `json:"uuid"`
		CryptoType string `json:"crypto_type"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"action":"error","error":"invalid json"}`))
		return
	}
	if req.Encrypted == "" || req.UUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"action":"error","error":"missing encrypted or uuid"}`))
		return
	}

	s.store.Store(req.UUID, req.Encrypted)

	s.db.Model(&model.CookieCloudConfig{}).Where("1 = 1").
		Update("last_encrypted", req.Encrypted)

	s.logger.Info("cookiecloud push received",
		zap.String("uuid", req.UUID),
		zap.Int("encrypted_len", len(req.Encrypted)),
		zap.String("crypto_type", req.CryptoType))

	s.scheduleDebouncedSync()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"action":"done"}`))
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	uuid := parts[len(parts)-1]
	if uuid == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"missing uuid"}`))
		return
	}

	val, ok := s.store.Load(uuid)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
		return
	}

	encrypted, _ := val.(string)
	resp := map[string]string{
		"encrypted":   encrypted,
		"crypto_type": "legacy",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
