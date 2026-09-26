// aka-title-fix §59.290: 已发布种子 AKA 双名标题批修工具（独立小工具，
// 与主程序零耦合——xdy-codec-fix 同模式）。
//
// 用法：
//   aka-title-fix -url https://xdypt.vip -cookie-file /tmp/xdy.cookie \
//                 -list /tmp/aka_fix.jsonl [-apply] [-sleep 1.5s]
//
//   -list  JSONL 每行 {"tid":"3620","title":"旧标题","site":"修道院"}
//          （从 publish_result_records 导出——见 docs §59.290）
package main

import (
	"bufio"
	"regexp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

type fixRec struct {
	TID   string `json:"tid"`
	Title string `json:"title"`
	Site  string `json:"site"`
}

func main() {
	var (
		baseURL    = flag.String("url", "", "站点 base URL")
		cookieFile = flag.String("cookie-file", "", "cookie 文件")
		listFile   = flag.String("list", "", "JSONL 修复清单")
	dump       = flag.String("dump", "", "输出新旧映射 JSONL（不请求站点——DB 记录同步用）")
		apply      = flag.Bool("apply", false, "真提交（缺省 dry-run）")
		sleep      = flag.Duration("sleep", 1500*time.Millisecond, "请求间隔")
	)
	flag.Parse()
	if *baseURL == "" || *cookieFile == "" || *listFile == "" {
		fmt.Fprintln(os.Stderr, "-url -cookie-file -list 必填")
		os.Exit(1)
	}
	ckRaw, err := os.ReadFile(*cookieFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cookie: %v\n", err)
		os.Exit(1)
	}
	cookie := strings.TrimSpace(string(ckRaw))

	// -dump 模式：纯本地重算输出（不请求站点）
	if *dump != "" {
		df, derr := os.Create(*dump)
		if derr != nil {
			fmt.Fprintf(os.Stderr, "dump: %v", derr)
			os.Exit(1)
		}
		defer df.Close()
		lf, lerr := os.Open(*listFile)
		if lerr != nil {
			fmt.Fprintf(os.Stderr, "list: %v", lerr)
			os.Exit(1)
		}
		defer lf.Close()
		dsc := bufio.NewScanner(lf)
		for dsc.Scan() {
			var r fixRec
			if json.Unmarshal(dsc.Bytes(), &r) != nil {
				continue
			}
			nt := RewriteTitle(r.Title)
			if nt != r.Title {
				out, _ := json.Marshal(map[string]string{"tid": r.TID, "new_title": nt})
				fmt.Fprintln(df, string(out))
			}
		}
		fmt.Println("dump 完成:", *dump)
		return
	}

	f, err := os.Open(*listFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	nexus := adapter.NewNexusPHPAdapter(adapter.NewHTTPDoer(), nil)
	cfg := &model.SiteConfig{BaseURL: *baseURL, Domain: *baseURL, Cookie: cookie}

	stat := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4<<20), 4<<20)
	n := 0
	for sc.Scan() {
		var r fixRec
		if json.Unmarshal(sc.Bytes(), &r) != nil || r.TID == "" || r.Title == "" {
			continue
		}
		n++
		newTitle := RewriteTitle(r.Title)
		if newTitle == r.Title {
			stat["skip_nochange"]++
			continue
		}
		if !*apply {
			stat["plan"]++
			fmt.Printf("[%2d] tid=%-6s DRY\n     旧: %s\n     新: %s\n", n, r.TID, r.Title, newTitle)
			continue
		}
		form, err := nexus.GetEditForm(context.Background(), cfg, r.TID)
		if err != nil {
			stat["form_err"]++
			fmt.Printf("[%2d] tid=%-6s ✗ 表单: %v\n", n, r.TID, err)
			time.Sleep(*sleep)
			continue
		}
		// §59.271: descr 回填 + name 替换（其余字段原样回放）
		req := &model.EditRequest{
			TorrentID:   r.TID,
			FormFields:  form.Fields,
			Cookie:      cookie,
			BaseURL:     *baseURL,
			Referer:     *baseURL + "/edit.php?id=" + r.TID,
			ArrayFields: form.ArrayFields,
		}
		req.FormFields["descr"] = form.Description
		req.FormFields["name"] = newTitle
		if err := nexus.SubmitEdit(context.Background(), req); err != nil {
			stat["edit_err"]++
			fmt.Printf("[%2d] tid=%-6s ✗ 提交: %v\n", n, r.TID, err)
		} else {
			stat["fixed"]++
			fmt.Printf("[%2d] tid=%-6s OK  %s\n", n, r.TID, newTitle)
		}
		time.Sleep(*sleep)
	}
	fmt.Printf("\n汇总(%s): %+v\n", map[bool]string{true: "APPLY", false: "DRY"}[*apply], stat)
}

// RewriteTitle 标题内 AKA 双名段替换——选段复用 titleparser.SplitAKATitle，
// 其余部分（年份/技术区/组名）与分隔风格原样保留。
func RewriteTitle(t string) string {
	loc := akaSpan(t)
	if loc == nil {
		return t
	}
	head := strings.Trim(t[:loc[0]], ". ")
	tailAll := t[loc[1]:]
	// AKA 右侧第二个名截至下一个锚（4 位年份/分辨率前缀）——其后是技术区
	end := len(tailAll)
	for _, anchor := range []string{" 19", " 20", " 1080", " 2160", " 720", ".19", ".20", ".1080", ".2160", ".720"} {
		if j := strings.Index(tailAll, anchor); j > 0 && j < end {
			end = j
		}
	}
	tailName := strings.Trim(tailAll[:end], ". ")
	rest := strings.Trim(tailAll[end:], ". ")
	// 年份括注保全："(YYYY)" 附着在被丢弃的第二名上时移回 rest（该标题
	// 唯一年份标记——Andhra King Taluka aka RaPo 22 (2025) 案）
	if ym := reYearParen.FindStringSubmatch(tailName); ym != nil {
		tailName = strings.TrimSpace(strings.ReplaceAll(tailName, ym[0], ""))
		if !strings.Contains(rest, ym[0]) {
			dotStylePre := strings.Contains(t, ".") && !strings.Contains(t[:loc[1]], " ")
			if dotStylePre {
				rest = ym[0] + "." + rest
			} else {
				rest = ym[0] + " " + rest
			}
		}
	}
	chosen := titleparser.SplitAKATitle(head + " AKA " + tailName)
	dotStyle := strings.Contains(t[:loc[1]], ".") && !strings.Contains(t[:loc[1]], " ")
	sep := " "
	if dotStyle {
		sep = "."
		chosen = strings.ReplaceAll(chosen, " ", ".")
		rest = strings.ReplaceAll(strings.ReplaceAll(rest, " ", "."), "..", ".")
	}
	out := chosen + sep + rest
	if dotStyle {
		return strings.Trim(out, ".")
	}
	return strings.Join(strings.Fields(out), " ")
}

var reYearParen = regexp.MustCompile(`\((19|20)\d{2}\)`)

// akaSpan 找词边界 AKA/aka 的外延分隔（含两侧空格/点）
func akaSpan(t string) []int {
	lt := strings.ToLower(t)
	for i := 0; i+3 <= len(lt); i++ {
		if lt[i:i+3] != "aka" {
			continue
		}
		before, after := byte(' '), byte(' ')
		if i > 0 {
			before = t[i-1]
		}
		if i+3 < len(t) {
			after = t[i+3]
		}
		if (before == ' ' || before == '.') && (after == ' ' || after == '.') {
			s, e := i, i+3
			for s > 0 && (t[s-1] == ' ' || t[s-1] == '.') {
				s--
			}
			for e < len(t) && (t[e] == ' ' || t[e] == '.') {
				e++
			}
			return []int{s, e}
		}
	}
	return nil
}
