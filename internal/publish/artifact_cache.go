package publish

import (
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

type ArtifactCache struct {
	dir    string
	logger *zap.Logger
	mu     sync.RWMutex
}

func NewArtifactCache(dir string, logger *zap.Logger) *ArtifactCache {
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Warn("artifact cache dir create failed", zap.String("dir", dir), zap.Error(err))
	}
	return &ArtifactCache{dir: dir, logger: logger}
}

func (ac *ArtifactCache) Get(key string) ([]byte, bool) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	p := ac.path(key)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (ac *ArtifactCache) Set(key string, data []byte) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return os.WriteFile(ac.path(key), data, 0644)
}

func (ac *ArtifactCache) Delete(key string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return os.Remove(ac.path(key))
}

func (ac *ArtifactCache) path(key string) string {
	return filepath.Join(ac.dir, key)
}
