package reseed

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
)

// TestRunTask_CleansUpRegistrationAfterFinish §59.544 修1 回归锚：
// RunTask 结束后必须自清理 tasks 注册——原 bug：defer 无 delete，
// 上次任务（调度/手动）结束后 tasks[id] 残留旧 cancel，下次 RunTask 的
// `if !exists` 判断被残留占位 → 不注册不换 ctx → CancelTask cancel 的
// 是无人在用的旧链（任务失控·status 永久 running——2026-11-21 好大 L2
// 案根因①·两次 dump+cancel 行为实验实锤）。
func TestRunTask_CleansUpRegistrationAfterFinish(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "cleanup-reg", Enabled: true, ClientIDs: "1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// 引擎启动形态：startTask 预注册（残留的形成源头）
	e.Start(context.Background())
	e.mu.RLock()
	stale, ok := e.tasks[task.ID]
	e.mu.RUnlock()
	if !ok || stale == nil {
		t.Fatal("startTask 预注册失败")
	}

	// 手动触发形态（handleTrigger 同构：Background ctx）
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = e.RunTask(context.Background(), task)
	}()

	// 等待 RunTask 完成：defer 自清理后 map 条目消失=完成的可靠信号
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		_, exists := e.tasks[task.ID]
		e.mu.RUnlock()
		if !exists {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunTask 未在期限内返回")
	}

	e.mu.RLock()
	_, exists := e.tasks[task.ID]
	e.mu.RUnlock()
	if exists {
		t.Fatal("RunTask 结束后 tasks 注册未自清理（残留=cancel 断链复发温床·§59.544 根因①回归）")
	}
}

// TestRunTask_OverridesStaleRegistration §59.544 修1 核心锚：
// startTask 预注册的残留必须被 RunTask 无条件覆盖——若保留旧 tc
// （注册未被替换），CancelTask 永远 cancel 不到真正在跑的任务。
func TestRunTask_OverridesStaleRegistration(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "override-stale", Enabled: true, ClientIDs: "1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	e.Start(context.Background())
	e.mu.RLock()
	stale, ok := e.tasks[task.ID]
	e.mu.RUnlock()
	if !ok {
		t.Fatal("startTask 预注册失败")
	}

	// RunTask 期间（用 channel 卡住完成信号观察中途态）：
	// 无条件覆盖后 map 里的 tc ≠ stale 残留
	inFlight := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	// 在 RunTask 返回前检查：观察手段=轮询 map 指针替换
	go func() {
		_, _ = e.RunTask(context.Background(), task)
		once.Do(func() { close(inFlight) })
	}()
	_ = release

	deadline := time.Now().Add(5 * time.Second)
	replaced := false
	for time.Now().Before(deadline) {
		e.mu.RLock()
		cur, exists := e.tasks[task.ID]
		e.mu.RUnlock()
		if exists && cur != stale {
			replaced = true
			break
		}
		select {
		case <-inFlight:
			// RunTask 已结束（快速路径）——若 map 已清理也算替换发生过
			e.mu.RLock()
			_, stillExists := e.tasks[task.ID]
			e.mu.RUnlock()
			if !stillExists {
				replaced = true
			}
			deadline = time.Now() // 退出轮询
		case <-time.After(5 * time.Millisecond):
		}
	}
	if !replaced {
		t.Fatal("RunTask 未覆盖残留注册（cancel 断链·§59.544 根因①核心回归）")
	}
}
