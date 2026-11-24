package seeding

import (
	"context"
	"testing"
)

// §59.563 刀C 锚：flush/free_end ctx 派生单源的 nil 安全读取。
// Background() 返回全局唯一实例——指针相等即机械证明。

func TestFlushCtxNilSafety(t *testing.T) {
	// ① ctx 载入透传
	c := context.Background()
	fc := &flushContext{ctx: c}
	if fc.flushCtx() != c {
		t.Fatal("flushCtx 应透传载入的 ctx")
	}
	// ② 字段 nil 兜底 Background
	fc2 := &flushContext{}
	if fc2.flushCtx() != context.Background() {
		t.Fatal("字段 nil 应兜底 Background")
	}
	// ③ nil receiver 兜底（直接构造测试形态防御）
	var fc3 *flushContext
	if fc3.flushCtx() != context.Background() {
		t.Fatal("nil receiver 应兜底 Background")
	}
}

func TestEngineBaseContextNilSafety(t *testing.T) {
	// ① nil receiver 兜底
	var e *Engine
	if e.baseContext() != context.Background() {
		t.Fatal("nil receiver 应兜底 Background")
	}
	// ② rootCtx 未注入（Start 前）兜底
	e2 := &Engine{}
	if e2.baseContext() != context.Background() {
		t.Fatal("rootCtx nil 应兜底 Background")
	}
	// ③ rootCtx 注入后透传（free_end 回调派生单源）
	ctx := context.Background()
	e3 := &Engine{rootCtx: ctx}
	if e3.baseContext() != ctx {
		t.Fatal("应透传 rootCtx（派生单源）")
	}
}
