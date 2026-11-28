package api

import (
	"os"
	"strings"
	"testing"
)

// §59.597（审计 21 刀3 锚③）: ChangedKeys 契约锁——settings_handler 两处
// Publish 恒发 DB setting key 形态（`ChangedKeys: []string{key}`）。
//
// 审计 21 篇 V-02：设计权威曾规定 YAML 点分路径·发布方从未实现（发 setting
// key）·消费者（图床代理两键）实听 setting key——两端一致即现状契约。
// §59.597 刀1 已将权威修订为实态；本锚锁死发布形态·防将来按点分路径
// 接入新消费者时踩空（「修了订阅端仍不工作」的断链预防）。
//
// 阴性对照（刀3 三阴性 C）：Publish 改发点分路径形态 → 本锚红。
func TestConfigEventPublishKeyContract(t *testing.T) {
	data, err := os.ReadFile("settings_handler.go")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	src := string(data)
	// 两处 Publish 的键形态指纹（[]string{key}——setting key 单元素）
	count := strings.Count(src, "ChangedKeys: []string{key}")
	if count != 2 {
		t.Errorf("settings_handler.go 的 Publish 键形态契约断裂：期望 2 处 `ChangedKeys: []string{key}`（setting key 形态·与消费者实听一致·§59.597 契约锁），实际 %d 处——若改发点分路径等新形态须同步改造消费者并修订契约锚", count)
	}
}
