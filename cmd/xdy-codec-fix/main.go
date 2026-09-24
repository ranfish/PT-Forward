// xdy-codec-fix §59.269: 修道院存量种子 codec+tags 修复工具。
//
// 背景：§59.268 codec 域 miss（x264/x265/AV1 编码器名 vs 站表标准名）+ §59.269
// TagConfig 缺失标签全丢——修复本体后，存量已发种子经编辑页补齐。
//
// 用法：
//   xdy-codec-fix -url https://xdypt.vip -cookie-file /tmp/xdy.cookie \
//                 -list /tmp/xdy.txt -tags-map /tmp/xdy.tags.json [-apply] [-sleep 1.5s]
//
//   -list      种子链接列表（details.php?id=N 每行一条）
//   -tags-map  tid → 推断标签 JSON（{"2810":["chinese_audio",...]}，来自
//              publish_result_records × torrent_metadata.tags 预生成）
//   -apply     缺省 dry-run（只打印计划不提交）
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
)

// 站方 codec 选项值（form_config 权威——§59.268 修复映射）
var codecTarget = map[string]string{
	"x264": "1", // H.264
	"x265": "6", // H.265
	"AV1":  "5", // Other（用户定案）
}

var reToken = regexp.MustCompile(`(?i)\b(x264|x265|AV1)\b`)
var reTid = regexp.MustCompile(`id=(\d+)`)

// 站方 tags 值映射（tags-map 文件的 standard_key → 站方 value 由 -form-config 提供）
func main() {
	var (
		baseURL    = flag.String("url", "https://xdypt.vip", "站点 base URL")
		cookieFile = flag.String("cookie-file", "", "cookie 文件路径")
		listFile   = flag.String("list", "", "种子链接列表文件")
		tagsMap    = flag.String("tags-map", "", "tid→推断标签 JSON 文件（可选）")
		formConfig = flag.String("form-config", "", "站点 form_config JSON（tags 值映射源，可选）")
		apply      = flag.Bool("apply", false, "真提交（缺省 dry-run）")
		tagOnly    = flag.Bool("tag-only", false, "纯标签补齐模式（无 codec token 的种子也补 tags——后期跨环境元数据补齐批量用）")
		sleep      = flag.Duration("sleep", 1500*time.Millisecond, "请求间隔")
	)
	flag.Parse()
	if *cookieFile == "" || *listFile == "" {
		fmt.Fprintln(os.Stderr, "-cookie-file 与 -list 必填")
		os.Exit(1)
	}
	cookieRaw, err := os.ReadFile(*cookieFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read cookie: %v\n", err)
		os.Exit(1)
	}
	cookie := strings.TrimSpace(string(cookieRaw))

	tagsByTid := map[string][]string{}
	if *tagsMap != "" {
		raw, err := os.ReadFile(*tagsMap)
		if err == nil {
			_ = json.Unmarshal(raw, &tagsByTid)
		}
	}
	// tags standard_key → 站方 value（form_config value_mappings.tags 派生）
	tagValue := map[string]string{}
	if *formConfig != "" {
		raw, err := os.ReadFile(*formConfig)
		if err == nil {
			var cfg model.PublishFormConfig
			if json.Unmarshal(raw, &cfg) == nil {
				for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
					// 禁转/首发不自动勾（§59.269 铁律：禁转站方语义须可见/首发=官组专属）
					if m.Label == "禁转" || m.Label == "首发" {
						continue
					}
					for _, k := range m.StandardKeys {
						tagValue[k] = m.Value
						if i := strings.Index(k, "."); i > 0 {
							tagValue[k[i+1:]] = m.Value
						}
					}
					// §59.272: 存量旧键别名（§59.267 前 metadata.tags 用 vivid_hdr）
					if m.Label == "HDR" {
						tagValue["vivid_hdr"] = m.Value
					}
				}
			}
		}
	}

	nexus := adapter.NewNexusPHPAdapter(adapter.NewHTTPDoer(), nil)
	cfg := &model.SiteConfig{BaseURL: *baseURL, Domain: *baseURL, Cookie: cookie}

	tids, err := readTIDs(*listFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read list: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("种子 %d 个 | 模式=%s | 限速 %v\n\n", len(tids), map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply], *sleep)

	stat := map[string]int{}
	for i, tid := range tids {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		form, err := nexus.GetEditForm(ctx, cfg, tid)
		cancel()
		if err != nil {
			stat["fetch_err"]++
			fmt.Printf("[%3d] tid=%-6s ✗ 获取编辑表单失败: %v\n", i+1, tid, err)
			time.Sleep(*sleep)
			continue
		}

		m := reToken.FindStringSubmatch(form.Title)
		// tags 目标值（现有勾选 ∪ 推断映射——只加不减）
		tagWant := map[string]bool{}
		for _, kv := range form.ArrayFields {
			if kv.Key == "tags[4][]" {
				tagWant[kv.Value] = true
			}
		}
		for _, t := range tagsByTid[tid] {
			if v, ok := tagValue[t]; ok {
				tagWant[v] = true
			}
		}
		curTagCount := 0
		for _, kv := range form.ArrayFields {
			if kv.Key == "tags[4][]" {
				curTagCount++
			}
		}

		if m == nil {
			// 非编码器名 token：纯标签模式下有可补标签才动，否则跳过
			if *tagOnly && len(tagWant) > curTagCount {
				if !*apply {
					stat["plan"]++
					fmt.Printf("[%3d] tid=%-6s %s | tag-only | tags %d→%d | %s\n",
						i+1, tid, "DRY", curTagCount, len(tagWant), trunc(form.Title, 44))
					time.Sleep(*sleep)
					continue
				}
				if err := submitEdit(nexus, cfg, cookie, *baseURL, tid, form, form.Fields["codec_sel[4]"], tagWant); err != nil {
					stat["edit_err"]++
					fmt.Printf("[%3d] tid=%-6s ✗ tag-only | tags %d→%d | %s | err=%v\n",
						i+1, tid, curTagCount, len(tagWant), trunc(form.Title, 40), err)
				} else {
					stat["fixed"]++
					fmt.Printf("[%3d] tid=%-6s %s | tag-only | tags %d→%d | %s\n",
						i+1, tid, "OK ", curTagCount, len(tagWant), trunc(form.Title, 44))
				}
				time.Sleep(*sleep)
				continue
			}
			stat["skip_no_token"]++
			continue
		}
		target := codecTarget[m[1]]

		curCodec := form.Fields["codec_sel[4]"]
		if curCodec == target && len(tagWant) == curTagCount {
			stat["skip_ok"]++
			continue // 已正确（幂等）
		}

		if !*apply {
			stat["plan"]++
			fmt.Printf("[%3d] tid=%-6s %s | %s | codec %q→%s | tags %d→%d | %s\n",
				i+1, tid, "DRY", m[1], curCodec, target, curTagCount, len(tagWant), trunc(form.Title, 44))
			time.Sleep(*sleep)
			continue
		}
		if err := submitEdit(nexus, cfg, cookie, *baseURL, tid, form, target, tagWant); err != nil {
			stat["edit_err"]++
			fmt.Printf("[%3d] tid=%-6s ✗ %s | %s | codec %s→%s tags %d→%d | err=%v\n",
				i+1, tid, m[1], trunc(form.Title, 40), curCodec, target, curTagCount, len(tagWant), err)
		} else {
			stat["fixed"]++
			fmt.Printf("[%3d] tid=%-6s %s | %s | codec %q→%s | tags %d→%d | %s\n",
				i+1, tid, "OK ", m[1], curCodec, target, curTagCount, len(tagWant), trunc(form.Title, 44))
		}
		time.Sleep(*sleep)
	}
	fmt.Printf("\n汇总: %+v\n", stat)
}

// submitEdit 组装并提交编辑（codec 路径与 tag-only 路径共用）。
// §59.271: descr 必须回填（GetEditForm 存 form.Description 不入 Fields——
// 缺省提交被"有项目没有填写"拒）；tags 只加不减（现有勾选 ∪ 目标）。
func submitEdit(nexus *adapter.NexusPHPAdapter, cfg *model.SiteConfig, cookie, baseURL, tid string,
	form *model.EditForm, codecTargetValue string, tagWant map[string]bool) error {
	req := &model.EditRequest{
		TorrentID:   tid,
		FormFields:  form.Fields,
		Cookie:      cookie,
		BaseURL:     baseURL,
		Referer:     baseURL + "/edit.php?id=" + tid,
		ArrayFields: form.ArrayFields,
	}
	req.FormFields["descr"] = form.Description
	if codecTargetValue != "" {
		req.FormFields["codec_sel[4]"] = codecTargetValue
	}
	seen := map[string]bool{}
	arr := []model.TagKV{}
	for _, kv := range form.ArrayFields {
		if kv.Key == "tags[4][]" && seen[kv.Value] {
			continue
		}
		seen[kv.Value] = true
		arr = append(arr, kv)
	}
	for v := range tagWant {
		if !seen[v] {
			arr = append(arr, model.TagKV{Key: "tags[4][]", Value: v})
		}
	}
	req.ArrayFields = arr
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return nexus.SubmitEdit(ctx, req)
}

func readTIDs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := reTid.FindStringSubmatch(sc.Text()); len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out, sc.Err()
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
