package audit

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditDB(t *testing.T) *gorm.DB {
	t.Helper()
	// §59.567 刀B 补修：`:memory:` 在 gorm 池开第二连接时分裂为空库
	// （WaitFor 轮询与 flush INSERT 并发触发·原固定 sleep 串行化掩住）——
	// unique+cache=shared 内存库（seeding 包 uniqueSQLiteDSN 同款惯例）
	dsn := fmt.Sprintf("file:audittest_%d?mode=memory&cache=shared&_busy_timeout=5000&_journal_mode=WAL", auditDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.OperationAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

var auditDBCounter atomic.Uint64

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

func TestLogger_StartAndFlushOnCancel(t *testing.T) {
	db := setupAuditDB(t)
	l := NewLogger(db, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	l.Start(ctx)

	l.Log("admin", "site", "create", "site", "1", "detail", "success")
	l.Log("admin", "site", "delete", "site", "2", "", "failure")

	// §59.577 附（1625 build 门二拦实证·同测试第三修）：轮询可见性竞态
	// 终结——cancel→Stop()=flush 完成的精确同步点（wg.Wait 在 flush 返回
	// 后才归零·INSERT 已提交）·直断言零轮询。前两修（§59.567 刀B
	// cache=shared+WAL / §59.568 刀4 窗口 3→10s）都在竞态窗口上加码·
	// 本修消灭窗口本身。
	cancel()
	l.Stop()
	var n int64
	db.Model(&model.OperationAuditLog{}).Count(&n)
	if n != 2 {
		t.Errorf("expected 2 logs after cancel, got %d", n)
	}
}

func TestLogger_FlushLoopBatchSize(t *testing.T) {
	db := setupAuditDB(t)
	l := NewLogger(db, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)

	for i := 0; i < 55; i++ {
		l.Log("admin", "site", "action", "type", string(rune(i)), "", "ok")
	}

	// §59.577 附：同上——cancel→Stop 精确同步·直断言（55 条全落库）
	cancel()
	l.Stop()
	count := func() int64 {
		var n int64
		db.Model(&model.OperationAuditLog{}).Count(&n)
		return n
	}
	if n := count(); n < 50 {
		t.Errorf("expected at least 50 flushed, got %d", count())
	}
}

func TestLogger_NewLogger(t *testing.T) {
	db := setupAuditDB(t)
	l := NewLogger(db, zap.NewNop())
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	if cap(l.ch) != 1000 {
		t.Errorf("expected channel capacity 1000, got %d", cap(l.ch))
	}
}
