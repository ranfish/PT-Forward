package httpclient

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 09 刀4 §3 静态锚：HTTP 边界资源四类禁止形态源码扫描——新增即拦·
// 存量不要求清零（白名单例外登记）。指纹设计对齐 TestNoBarePublishStatusList
// 先例（§59.546 源码扫描锚——模式=正则指纹+walk+排除 testdata/隐藏目录/
// _test.go）。四类形态：
//
//	① 裸 io.ReadAll(resp.Body)——无 LimitReader 上限（远端可驱动内存放大）
//	② 裸 http.NewRequest（无 WithContext——请求取消不传播·01 篇 ctx 纪律）
//	③ UA 硬编码（Mozilla/5.0 字面量——指纹单点漂移·09 刀3 收编后残留 0）
//	④ http.DefaultTransport 写入（全局污染——09 刀1 根修后残留 0·
//	   imagehost 的 transportSwitcher 读 default 是合法回落）
//
// 白名单例外文件（合理例外·附录H 登记）：
//   - 本包 drain.go/browser.go：单点实现本体（ReadAll 的 LimitReader 版）
//   - httpclient/chrome_transport.go：CF 指纹栈（azuretls——基础设施例外）
//   - 各 _test.go：测试（walk 已排除）
//
// 阴性对照（09 §4 要求·实施时已验证）：临时新增一处裸 client/裸 ReadAll/
// 裸 NewRequest/裸 ==200 → CI 红；删除后恢复。锚的防回归声明逐条
// 可触发（08 回归审核方法论——虚拟安全感零容忍）。
func TestNoBareHTTPBoundaryPatterns(t *testing.T) {
	// ① 裸 io.ReadAll(resp.Body)——不含 LimitReader 包裹的
	reBareReadAll := regexp.MustCompile(`io\.ReadAll\(resp\.Body\)`)
	// ② 裸 http.NewRequest（非 WithContext——后跟非大写 W 字母区分）
	reBareNewRequest := regexp.MustCompile(`http\.NewRequest\(`)
	// ③ UA 硬编码字面量
	reHardcodedUA := regexp.MustCompile(`"Mozilla/5\.0`)
	// ④ http.DefaultTransport 赋值（写·非读）
	reDefaultTransport := regexp.MustCompile(`http\.DefaultTransport\s*=`)

	// 白名单（相对 internal/ 路径前缀匹配）
	whitelist := []string{
		"httpclient/drain.go",            // ReadBody 单点本体（LimitReader 版 ReadAll）
		"httpclient/browser.go",          // BrowserUserAgent 单点本体
		"httpclient/chrome_transport.go", // CF 指纹栈基础设施
	}

	type pattern struct {
		name string
		re   *regexp.Regexp
	}
	patterns := []pattern{
		{"裸 io.ReadAll(resp.Body)", reBareReadAll},
		{"裸 http.NewRequest（无 WithContext）", reBareNewRequest},
		{"UA 硬编码 Mozilla/5.0 字面量", reHardcodedUA},
		{"http.DefaultTransport 赋值", reDefaultTransport},
	}

	root := ".." // internal/（从 httpclient 包目录上溯一级）
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// root 自身（path==root）不应用隐藏目录检查——".." 的 Name()
			// 以 "." 开头会被自己 SkipDir（本坑）
			if path == root {
				return nil
			}
			if info.Name() == "testdata" || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+"/"))
		if rel == "" {
			return nil
		}
		// 白名单豁免
		for _, w := range whitelist {
			if strings.HasSuffix(rel, w) {
				return nil
			}
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, p := range patterns {
			for _, loc := range p.re.FindAllStringIndex(string(raw), -1) {
				// ② 排除 NewRequestWithContext（含 NewRequest( 子串——
				// 精确判定：匹配位置后紧跟 'W' 大写=WithContext）
				if p.name == "裸 http.NewRequest（无 WithContext）" {
					after := string(raw[loc[1]:])
					if strings.HasPrefix(after, "W") { // NewRequestWithContext
						continue
					}
				}
				line := strings.Count(string(raw)[:loc[0]], "\n") + 1
				t.Errorf("%s:%d 出现 %s——须走公共单点（httpclient.ReadBody/NewRequestWithContext/BrowserUserAgent·附录H 三条铁律）", rel, line, p.name)
			}
		}
		return nil
	})
	_ = io.Discard
}
