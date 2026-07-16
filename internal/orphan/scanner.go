package orphan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type Scanner struct {
	provider model.DownloaderProvider
	logger   *zap.Logger
}

func NewScanner(provider model.DownloaderProvider, logger *zap.Logger) *Scanner {
	return &Scanner{provider: provider, logger: logger.With(zap.String("component", "orphan"))}
}

func (s *Scanner) Scan(ctx context.Context) ([]Entry, error) {
	if s.provider == nil {
		return nil, nil
	}

	clientIDs := s.provider.ListClients()

	claimed := make(map[string]map[string]bool)
	clientForPath := make(map[string]string)
	scannedPaths := make(map[string]bool)
	allSavePaths := make(map[string]bool)

	for _, clientID := range clientIDs {
		if ctx.Err() != nil {
			break
		}

		client, err := s.provider.Get(clientID)
		if err != nil {
			continue
		}

		md, err := client.GetMainData(ctx)
		if err != nil || md == nil {
			s.logger.Debug("orphan scan: get maindata failed",
				zap.String("client", clientID), zap.Error(err))
			continue
		}

		for _, t := range md.Torrents {
			sp := t.SavePath
			if sp == "" {
				continue
			}
			sp = filepath.Clean(sp)
			if claimed[sp] == nil {
				claimed[sp] = make(map[string]bool)
				clientForPath[sp] = clientID
			}
			claimed[sp][t.Name] = true
			allSavePaths[sp] = true
		}
	}

	var allOrphans []Entry
	for savePath, claimedNames := range claimed {
		if scannedPaths[savePath] {
			continue
		}
		scannedPaths[savePath] = true
		orphans := s.scanDirectory(savePath, claimedNames, clientForPath[savePath], allSavePaths)
		allOrphans = append(allOrphans, orphans...)
	}

	s.logger.Info("orphan scan completed",
		zap.Int("orphans", len(allOrphans)),
		zap.Int("clients", len(clientIDs)),
		zap.Int("save_paths", len(claimed)))

	return allOrphans, nil
}

var skipSuffixes = []string{".!qB", ".parts", ".tmp"}
var skipNames = map[string]bool{
	".DS_Store":          true,
	"Thumbs.db":          true,
	"desktop.ini":        true,
	".thumbnails":        true,
	"__pycache__":        true,
}

func shouldSkip(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true
	}
	if skipNames[name] {
		return true
	}
	lower := strings.ToLower(name)
	for _, suffix := range skipSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func isClaimedFuzzy(dirName string, claimed map[string]bool) bool {
	stripped := stripChineseBracketPrefix(dirName)
	if stripped != dirName && claimed[stripped] {
		return true
	}
	for cname := range claimed {
		if cname == dirName {
			return true
		}
		if stripChineseBracketPrefix(cname) == stripped && stripped != "" {
			return true
		}
	}
	return false
}

func stripChineseBracketPrefix(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	if len(r) >= 2 && (r[0] == '[' || r[0] == '【') {
		close := ']'
		if r[0] == '【' {
			close = '】'
		}
		for i := 1; i < len(r); i++ {
			if r[i] == close {
				rest := strings.TrimLeft(string(r[i+1:]), ". ")
				return rest
			}
		}
	}
	return s
}

func (s *Scanner) scanDirectory(savePath string, claimed map[string]bool, clientID string, allSavePaths map[string]bool) []Entry {
	entries, err := os.ReadDir(savePath)
	if err != nil {
		s.logger.Debug("orphan scan: cannot read directory",
			zap.String("path", savePath), zap.Error(err))
		return nil
	}

	var orphans []Entry
	now := time.Now()

	for _, entry := range entries {
		name := entry.Name()

		if shouldSkip(name) {
			continue
		}

		if claimed[name] || isClaimedFuzzy(name, claimed) {
			continue
		}

		fullPath := filepath.Join(savePath, name)
		if entry.IsDir() && allSavePaths[fullPath] {
			continue
		}
		size := int64(0)
		if entry.IsDir() {
			size = dirSize(fullPath)
		} else {
			if info, err := entry.Info(); err == nil {
				size = info.Size()
			}
		}

		orphans = append(orphans, Entry{
			Path:       fullPath,
			Name:       name,
			Size:       size,
			IsDir:      entry.IsDir(),
			ClientID:   clientID,
			SavePath:   savePath,
			DetectedAt: now,
		})
	}

	return orphans
}

func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
