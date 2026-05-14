package publish

import (
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

type TorrentCache struct {
	dir    string
	logger *zap.Logger
	mu     sync.RWMutex
}

func NewTorrentCache(dir string, logger *zap.Logger) *TorrentCache {
	if err := os.MkdirAll(dir, 0750); err != nil {
		logger.Warn("torrent cache dir create failed", zap.String("dir", dir), zap.Error(err))
	}
	return &TorrentCache{dir: dir, logger: logger}
}

func (tc *TorrentCache) Get(key string) ([]byte, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	p := tc.path(key)
	data, err := os.ReadFile(p) //nolint:gosec // p derived from info_hash key, not user input
	if err != nil {
		return nil, false
	}
	return data, true
}

func (tc *TorrentCache) Set(key string, data []byte) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return os.WriteFile(tc.path(key), data, 0600)
}

func (tc *TorrentCache) Delete(key string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return os.Remove(tc.path(key))
}

func (tc *TorrentCache) path(key string) string {
	return filepath.Join(tc.dir, key)
}
