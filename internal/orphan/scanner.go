package orphan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Scanner struct {
	provider      model.DownloaderProvider
	db            *gorm.DB
	logger        *zap.Logger
	ignoredPaths  map[string]bool
	scanConfigs   []model.OrphanScanConfig
}

func NewScanner(provider model.DownloaderProvider, db *gorm.DB, logger *zap.Logger) *Scanner {
	return &Scanner{provider: provider, db: db, logger: logger.With(zap.String("component", "orphan"))}
}

func (s *Scanner) SetIgnoredPaths(paths []string) {
	s.ignoredPaths = make(map[string]bool, len(paths))
	for _, p := range paths {
		s.ignoredPaths[filepath.Clean(p)] = true
	}
}

func (s *Scanner) loadScanConfigs() {
	s.scanConfigs = nil
	if s.db == nil {
		return
	}
	s.db.Where("enabled = ?", true).Find(&s.scanConfigs)
}

func (s *Scanner) Scan(ctx context.Context) ([]Entry, error) {
	if s.provider == nil {
		return nil, nil
	}

	s.loadScanConfigs()

	// 配置的扫描路径 → 所属 client（用户明确指定"这个路径在这个下载器的磁盘上"）
	configuredClient := make(map[string]string)
	for _, cfg := range s.scanConfigs {
		configuredClient[filepath.Clean(cfg.ScanPath)] = cfg.ClientID
	}

	claimed := make(map[string]map[string]bool)
	scannedPaths := make(map[string]bool)
	allSavePaths := make(map[string]bool)
	pathToClients := make(map[string][]string)
	queriedClients := make(map[string]bool) // 成功查询 GetMainData 的 client

	for _, clientID := range s.provider.ListClients() {
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
		queriedClients[clientID] = true

		for _, t := range md.Torrents {
			sp := t.SavePath
			if sp == "" {
				continue
			}
			sp = filepath.Clean(sp)
			// 配置路径只认配置 client 的种子（跨机器同路径不混入）
			if cfgClient, ok := configuredClient[sp]; ok && cfgClient != clientID {
				continue
			}
			if claimed[sp] == nil {
				claimed[sp] = make(map[string]bool)
			}
			claimed[sp][t.Name] = true
			allSavePaths[sp] = true
			// 非配置路径：从遍历结果构建 pathToClients
			if _, ok := configuredClient[sp]; !ok {
				pathToClients[sp] = append(pathToClients[sp], clientID)
			}
		}
	}

	// 配置路径：检查配置 client 是否可达，不可达则跳过（防止假孤儿）
	for sp, cfgClient := range configuredClient {
		if !queriedClients[cfgClient] {
			s.logger.Warn("orphan scan: configured client unreachable, skipping path",
				zap.String("path", sp),
				zap.String("client", cfgClient))
			delete(claimed, sp)
			continue
		}
		if claimed[sp] == nil {
			claimed[sp] = make(map[string]bool)
		}
		pathToClients[sp] = []string{cfgClient}
		s.logger.Debug("orphan scan: configured path",
			zap.String("path", sp),
			zap.String("client", cfgClient))
	}

	for sp := range claimed {
		if len(pathToClients[sp]) == 0 {
			pathToClients[sp] = s.provider.ListClients()
		}
	}

	var allOrphans []Entry
	for savePath, claimedNames := range claimed {
		if scannedPaths[savePath] {
			continue
		}
		scannedPaths[savePath] = true
		// 去重 clientIDs（多个客户端名字可能指向同一实例）
		rawClients := pathToClients[savePath]
		seen := make(map[string]bool, len(rawClients))
		clients := make([]string, 0, len(rawClients))
		for _, c := range rawClients {
			if !seen[c] {
				seen[c] = true
				clients = append(clients, c)
			}
		}
		orphans := s.scanDirectory(savePath, claimedNames, clients, allSavePaths)
		allOrphans = append(allOrphans, orphans...)
	}

	s.logger.Info("orphan scan completed",
		zap.Int("orphans", len(allOrphans)),
		zap.Int("scan_paths", len(claimed)),
		zap.Int("configured_paths", len(s.scanConfigs)))

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

func (s *Scanner) scanDirectory(savePath string, claimed map[string]bool, clientIDs []string, allSavePaths map[string]bool) []Entry {
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
		if s.ignoredPaths != nil && s.ignoredPaths[fullPath] {
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
			ClientIDs:  clientIDs,
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
