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
	"sort"
	"path/filepath"
	"strings"
)

type tokenDef struct {
	Canonical   string   `json:"canonical"`
	StandardKey string   `json:"standard_key"`
	Display     string   `json:"display,omitempty"`
	FullName    string   `json:"full_name,omitempty"`
	Variants    []string `json:"variants,omitempty"`

	// tag 域（§59.35 P4）
	Label   string `json:"label,omitempty"`
	Aliases string `json:"aliases,omitempty"`
	Group   string `json:"group,omitempty"`
}

type tagDef struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Aliases string `json:"aliases"`
}

type tagGroupDef struct {
	Name string   `json:"name"`
	Tags []tagDef `json:"tags"`
}

func main() {
	dictDir := "internal/titleparser/dict"
	outPath := "web/src/generated/dict.ts"

	entries, err := os.ReadDir(dictDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dict dir: %v\n", err)
		os.Exit(1)
	}

	// domain → [(key, display)]（保持 tokens 源顺序——select v-for 消费依赖
	// 语义顺序：电影/电视剧/综艺...，字母序会让动漫排首位）
	domains := map[string][][2]string{}
	platforms := map[string]string{}
	// tag 域分组结构（§59.35 P4）：group → [(key, label, aliases)]，保持源顺序
	tagGroups := []tagGroupDef{}
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
			if domain == "tag" {
				label := t.Label
				if label == "" {
					label = t.Canonical
				}
				g := t.Group
				if g == "" {
					g = "其他"
				}
				found := false
				for i := range tagGroups {
					if tagGroups[i].Name == g {
						tagGroups[i].Tags = append(tagGroups[i].Tags, tagDef{Key: t.Canonical, Label: label, Aliases: t.Aliases})
						found = true
						break
					}
				}
				if !found {
					tagGroups = append(tagGroups, tagGroupDef{Name: g, Tags: []tagDef{{Key: t.Canonical, Label: label, Aliases: t.Aliases}}})
				}
				continue
			}
			if t.StandardKey == "" {
				continue
			}
			domains[domain] = append(domains[domain], [2]string{t.StandardKey, display})
		}
	}

	var b strings.Builder
	b.WriteString("// 本文件由 cmd/gen-dict 自动生成（§59.35 P3），禁止手改。\n")
	b.WriteString("// 数据源：internal/titleparser/dict/*.json（唯一真相源）。\n")
	b.WriteString("// 重新生成：go run ./cmd/gen-dict（CI drift 检查强制同步）。\n\n")

	emitMap := func(name, comment string, pairs [][2]string) {
		fmt.Fprintf(&b, "// %s\nexport const %s: Record<string, string> = {\n", comment, name)
		for _, p := range pairs {
			fmt.Fprintf(&b, "  %q: %q,\n", p[0], p[1])
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
	// platform 按字典序输出（无语义顺序需求）
	platPairs := make([][2]string, 0, len(platforms))
	for k, v := range platforms {
		platPairs = append(platPairs, [2]string{k, v})
	}
	sort.Slice(platPairs, func(i, j int) bool { return platPairs[i][0] < platPairs[j][0] })
	emitMap("PLATFORM_FULLNAMES", "platform 域：canonical → 厂商全名（Tab1 分发方 tooltip）", platPairs)

	// tag 域分组（§59.35 P4，TagSelector 数据源）
	fmt.Fprintf(&b, "// tag 域分组结构（TagSelector 数据源，派生自 dict/tag.json group 字段）\n")
	fmt.Fprintf(&b, "export interface TagDef { key: string; label: string; aliases: string }\n")
	fmt.Fprintf(&b, "export interface TagGroup { name: string; tags: TagDef[] }\n")
	fmt.Fprintf(&b, "export const TAG_GROUPS: TagGroup[] = ")
	tagJSON, _ := json.MarshalIndent(tagGroups, "", "  ")
	tagJSON = []byte(strings.ReplaceAll(string(tagJSON), "\n", "\n  "))
	b.Write(tagJSON)
	b.WriteString("\n\n")

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	n := 0
	for _, pairs := range domains {
		n += len(pairs)
	}
	fmt.Printf("%s: %d 词条 + %d platform\n", outPath, n, len(platforms))
}
