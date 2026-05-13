package api

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func startHub() *Hub {
	hub := NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)
	return hub
}

func TestNewSiteHealthChecker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	logger := zap.NewNop()
	hub := startHub()

	checker := NewSiteHealthChecker(db, logger, hub)
	if checker == nil {
		t.Fatal("expected non-nil checker")
	}
	if checker.db != db {
		t.Error("db mismatch")
	}
}

func TestSiteHealthChecker_check_EmptyDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	hub := startHub()
	checker := NewSiteHealthChecker(db, zap.NewNop(), hub)

	ctx := context.Background()
	checker.check(ctx)
}

func TestSiteHealthChecker_check_WithSites(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	db.Exec("CREATE TABLE sites (id INTEGER PRIMARY KEY, name TEXT, base_url TEXT, enabled BOOLEAN)")

	db.Exec("INSERT INTO sites (name, base_url, enabled) VALUES ('site1', 'https://s1.com', 1)")
	db.Exec("INSERT INTO sites (name, base_url, enabled) VALUES ('site2', 'https://s2.com', 0)")
	db.Exec("INSERT INTO sites (name, base_url, enabled) VALUES ('site3', 'https://s3.com', 1)")

	hub := startHub()
	checker := NewSiteHealthChecker(db, zap.NewNop(), hub)

	ctx := context.Background()
	checker.check(ctx)
}

func TestSiteHealthChecker_Run_Cancellation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	hub := startHub()
	checker := NewSiteHealthChecker(db, zap.NewNop(), hub)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		checker.Run(ctx, 10*time.Second)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after cancellation")
	}
}

func TestSiteHealthChecker_check_QueryError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.Close()

	hub := startHub()
	checker := NewSiteHealthChecker(db, zap.NewNop(), hub)

	ctx := context.Background()
	checker.check(ctx)
}
