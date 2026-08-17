// gen-dict 前端字典 codegen（§59.35 P3）。
//
// 从 dict/*.json 生成 web/src/generated/dict.ts——前端唯一词表来源，
// 生成物进 git，CI drift 检查（make dict && git diff --exit-code）。
//
// 生成内容（Layer 1 通用词表；站点 tag_config 是 DB 运行时数据，不进 codegen）：
//   - CATEGORY_LABELS：type 域 standard_key → 显示名（categoryLabel 数据源）
//   - MEDIUM_LABELS：medium 域 standard_key → 显示名
//   - HDR_LABELS：hdr 域 standard_key → 显示名
//   - CODEC_LABELS：video/audio 域 standard_key → 显示名
//   - RESOLUTION_LABELS：resolution 域 standard_key → 显示名
//   - PLATFORM_FULLNAMES：platform 域 canonical → 厂商全名（展示用）
//
// 用法：go run ./cmd/gen-dict
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type tokenDef struct {
	Canonical   string   `json:"canonical"`
	StandardKey string   `json:"standard_key"`
	Display     string   `json:"display,omitempty"`
	FullName    string   `json:"full_name,omitempty"`
	Variants    []string `json:"variants,omitempty"`
}

func main() {
	dictDir := "internal/titleparser/dict"
	outPath := "web/src/generated/dict.ts"

	entries, err := os.ReadDir(dictDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dict dir: %v\n", err)
		os.Exit(1)
	}

	// domain → key → display
	domains := map[string]map[string]string{}
	platforms := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		domain := strings.TrimSuffix(e.Name(), ".json")
		raw, err := os.ReadFile(filepath.Join(dictDir, e.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", e.Name(), err)
			os.Exit(1)
		}
		var body struct {
			Tokens []tokenDef `json:"tokens"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", e.Name(), err)
			os.Exit(1)
		}
		for _, t := range body.Tokens {
			display := t.Display
			if display == "" {
				display = t.Canonical
			}
			if domain == "platform" {
				if t.FullName != "" && platforms[t.Canonical] == "" {
					platforms[t.Canonical] = t.FullName
				}
				continue
			}
			if t.StandardKey == "" {
				continue
			}
			if domains[domain] == nil {
				domains[domain] = map[string]string{}
			}
			domains[domain][t.StandardKey] = display
		}
	}

	var b strings.Builder
	b.WriteString("// 本文件由 cmd/gen-dict 自动生成（§59.35 P3），禁止手改。\n")
	b.WriteString("// 数据源：internal/titleparser/dict/*.json（唯一真相源）。\n")
	b.WriteString("// 重新生成：go run ./cmd/gen-dict（CI drift 检查强制同步）。\n\n")

	emitMap := func(name, comment string, m map[string]string) {
		fmt.Fprintf(&b, "// %s\nexport const %s: Record<string, string> = {\n", comment, name)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %q: %q,\n", k, m[k])
		}
		b.WriteString("}\n\n")
	}

	emitMap("CATEGORY_LABELS", "type 域：standard_key → 显示名（categoryLabel 数据源）", domains["type"])
	emitMap("MEDIUM_LABELS", "medium 域：standard_key → 显示名", domains["medium"])
	emitMap("HDR_LABELS", "hdr 域：standard_key → 显示名", domains["hdr"])
	emitMap("VIDEO_CODEC_LABELS", "video_codec 域：standard_key → 显示名", domains["video_codec"])
	emitMap("AUDIO_CODEC_LABELS", "audio_codec 域：standard_key → 显示名", domains["audio_codec"])
	emitMap("RESOLUTION_LABELS", "resolution 域：standard_key → 显示名", domains["resolution"])
	emitMap("SOURCE_LABELS", "source 域：standard_key → 显示名", domains["source"])
	emitMap("PLATFORM_FULLNAMES", "platform 域：canonical → 厂商全名（Tab1 分发方 tooltip）", platforms)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	n := 0
	for _, m := range domains {
		n += len(m)
	}
	fmt.Printf("%s: %d 词条 + %d platform\n", outPath, n, len(platforms))
}
