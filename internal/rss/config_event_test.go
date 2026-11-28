package rss

import (
	"testing"
	"time"
)

// §59.597（审计 21 刀3）: ConfigEventBus 健康锚——图床代理真链的总线侧保真。
//
// 用户第三案裁决：总线保留（唯一秒级消费者=图床代理·main.go 订阅者）·
// rss Engine 侧死件已拆（刀2）。本锚锁总线核心语义：
//
//	① Publish→Subscribe 实收（fanout 不丢）
//	② 200ms debounce（合并窗口·两次 Publish 只收一次）
//	③ 版本去重（旧版本事件被拒）
//
// 阴性对照（刀3 三阴性 A）：删 Publish 的 fanout（select+default 置空）→ ①红。
func TestConfigEventBusAlive(t *testing.T) {
	bus := NewConfigEventBus()
	ch := bus.Subscribe()

	// ① 实收
	bus.Publish(ConfigChangedEvent{
		ChangedKeys: []string{"image_host_use_proxy", "httpProxy"},
		Version:     time.Now().UnixNano(),
	})
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("通道意外关闭")
		}
		if len(ev.ChangedKeys) != 2 || ev.ChangedKeys[0] != "image_host_use_proxy" || ev.ChangedKeys[1] != "httpProxy" {
			t.Fatalf("键集合不符: %v", ev.ChangedKeys)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("① 实收失败：debounce 200ms 后 1s 内未收到事件——总线 fanout 断裂（刀2 误伤或后续改动破坏）")
	}

	// ② debounce 合并（同窗口事件收敛一次发送）+ ③ 版本去重（同/旧版本拒——首个胜出）
	v2 := time.Now().UnixNano()
	bus.Publish(ConfigChangedEvent{ChangedKeys: []string{"b"}, Version: v2})
	bus.Publish(ConfigChangedEvent{ChangedKeys: []string{"c"}, Version: v2})     // 同版本拒发（首个 b 胜出）
	bus.Publish(ConfigChangedEvent{ChangedKeys: []string{"a"}, Version: v2 - 1}) // 旧版本拒发
	select {
	case ev := <-ch:
		if len(ev.ChangedKeys) != 1 || ev.ChangedKeys[0] != "b" {
			t.Fatalf("③ 版本去重语义不符（期望同版本首个 b·实收 %v）", ev.ChangedKeys)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("② 第二事件未到达——debounce Reset 语义破坏")
	}
	// 旧版本事件不应再有
	select {
	case ev := <-ch:
		t.Fatalf("③ 旧版本事件漏发（应被版本去重拒绝·实收 %v）", ev.ChangedKeys)
	case <-time.After(300 * time.Millisecond):
	}
}
