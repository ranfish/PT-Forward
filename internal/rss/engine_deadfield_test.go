package rss

import (
	"os"
	"strings"
	"testing"
)

// §59.597（审计 21 刀3 锚②）: 死件拆除静态锚——rss Engine 不得复活
// configBus 字段/SetConfigEventBus（审计 21 篇核验③：原实现只声明+赋值零读取·
// 用户第三案刀2 拆除）。
//
// 保留面（不受本锚约束）：ConfigEventBus 本体（config_event.go）+ api 侧
// settings_handler 的活链（发布端）——本锚只锁 Engine 侧死件不复活。
//
// 阴性对照（刀3 三阴性 B）：engine.go 加回 configBus 字段 → 本锚红。
func TestEngineConfigBusDeadFieldRemoved(t *testing.T) {
	data, err := os.ReadFile("engine.go")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	src := string(data)
	for _, banned := range []string{"configBus", "SetConfigEventBus"} {
		if strings.Contains(src, banned) {
			t.Errorf("engine.go 含死件符号 %q（§59.597 刀2 拆除后不得复活——若需事件总线消费请走 api 侧活链或命令通道单独立案）", banned)
		}
	}
}
