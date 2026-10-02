package orphan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeProvider struct{}

func (f *fakeProvider) Get(clientUID uint) (model.DownloaderClient, error) {
	return nil, os.ErrNotExist // 模拟不可达
}
func (f *fakeProvider) ListClients() []uint { return nil }

// §59.314 客户端不可达 → SkippedPaths 逐路径透出（28 案 14 路径静默 0 的回归）
func TestScanWithSkip_UnreachableClient(t *testing.T) {
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "t.db")), &gorm.Config{})
	if err != nil {
		t.Skip("sqlite unavailable:", err)
	}
	_ = db.AutoMigrate(&model.OrphanScanConfig{})
	db.Create(&model.OrphanScanConfig{ClientUID: 9, ScanPath: filepath.Join(dir, "a"), Enabled: true})
	db.Create(&model.OrphanScanConfig{ClientUID: 9, ScanPath: filepath.Join(dir, "b"), Enabled: true})
	s := NewScanner(&fakeProvider{}, db, zap.NewNop())
	res, err := s.ScanWithSkip(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Orphans) != 0 {
		t.Fatalf("orphans=%d want 0", len(res.Orphans))
	}
	if len(res.SkippedPaths) != 2 {
		t.Fatalf("SkippedPaths=%v want 2 条", res.SkippedPaths)
	}
}
