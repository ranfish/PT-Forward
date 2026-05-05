package audit

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.OperationAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestLogger_Flush(t *testing.T) {
	db := setupAuditDB(t)
	l := NewLogger(db, zap.NewNop())

	entries := []*model.OperationAuditLog{
		{Actor: "admin", Module: "site", Action: "create", TargetType: "site", TargetID: "1", Result: "success"},
		{Actor: "admin", Module: "site", Action: "delete", TargetType: "site", TargetID: "2", Result: "failure"},
	}
	l.flush(entries)

	var count int64
	db.Model(&model.OperationAuditLog{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 logs, got %d", count)
	}
}

func TestLogger_FlushEmpty(t *testing.T) {
	db := setupAuditDB(t)
	l := NewLogger(db, zap.NewNop())
	l.flush(nil)

	var count int64
	db.Model(&model.OperationAuditLog{}).Count(&count)
	if count != 0 {
		t.Error("expected 0 logs after empty flush")
	}
}

func TestLogger_ChannelFull(t *testing.T) {
	db := setupAuditDB(t)
	l := NewLogger(db, zap.NewNop())

	for i := 0; i < 1100; i++ {
		l.Log("admin", "test", "action", "type", "id", "", "ok")
	}

	if len(l.ch) != 1000 {
		t.Errorf("expected channel full at 1000, got %d entries", len(l.ch))
	}
}

func TestLogger_LogEnqueue(t *testing.T) {
	db := setupAuditDB(t)
	l := NewLogger(db, zap.NewNop())

	l.Log("admin", "site", "create", "site", "1", "detail", "success")

	if len(l.ch) != 1 {
		t.Fatalf("expected 1 entry in channel, got %d", len(l.ch))
	}

	entry := <-l.ch
	if entry.Actor != "admin" {
		t.Errorf("expected actor=admin, got %s", entry.Actor)
	}
	if entry.Module != "site" {
		t.Errorf("expected module=site, got %s", entry.Module)
	}
	if entry.Result != "success" {
		t.Errorf("expected result=success, got %s", entry.Result)
	}
}
