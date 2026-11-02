package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ranfish/pt-forward/internal/model"
)

func main() {
	raw, _ := os.ReadFile("/tmp/opencode/hx_cfg.json")
	cfg := model.ParseFormConfig(string(raw))
	fmt.Println("ValueMappings 键集:", func() []string {
		keys := make([]string, 0, len(cfg.ValueMappings))
		for k := range cfg.ValueMappings {
			keys = append(keys, k)
		}
		return keys
	}())
	for _, dom := range []string{"processing", "source", "audiocodec", "codec"} {
		fmt.Printf("%s: %d 项\n", dom, len(cfg.ValueMappings[dom]))
	}
	// 验证 FormValueMapping 反序列化
	b, _ := json.Marshal(cfg.ValueMappings["processing"])
	fmt.Println("processing 首字节:", string(b)[:80])
}
