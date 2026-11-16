package main

// §59.538 猪猪适配前置验证：tls-client Chrome 指纹双站探针（验证后即删）。
// ① 猪猪 /v1/api/ping——对照 curl_chrome116 的 200（库等效性）
// ② 好大 hashes POST——243 挂起现场复现（§59.1487：Go 标准栈 30s 超时）
// 用法：tlsprobe <piggo_key> [hdarea_key] [pieces_hash]

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

func newClient() (tls_client.HttpClient, error) {
	return tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithClientProfile(profiles.Chrome_131),
		tls_client.WithTimeoutSeconds(20),
	)
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func probeWithProfile(name string, pf profiles.ClientProfile, method, url, key, body string) {
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithClientProfile(pf),
		tls_client.WithTimeoutSeconds(15),
	)
	if err != nil {
		fmt.Printf("[%s] client err: %v\n", name, err)
		return
	}
	var req *http.Request
	var rd io.Reader
	if body != "" {
		rd = bytes.NewBufferString(body)
	}
	req, err = http.NewRequest(method, url, rd)
	if err != nil {
		fmt.Printf("[%s] req err: %v\n", name, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[%-18s] DO err(%v): %v\n", name, time.Since(start).Round(time.Millisecond), err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	ok := "❌"
	if resp.StatusCode == 200 {
		ok = "✅"
	}
	fmt.Printf("[%s %-16s] HTTP %d | %dms | %s\n", ok, name, resp.StatusCode, time.Since(start).Milliseconds(), trunc(string(b), 80))
}

func main() {
	mode := os.Args[1]
	if mode == "profiles" {
		// 遍历全部 Chrome profile 打猪猪 ping——找 CF 放行档
		list := []struct {
			name string
			pf   profiles.ClientProfile
		}{
			{"Chrome_120", profiles.Chrome_120}, {"Chrome_124", profiles.Chrome_124},
			{"Chrome_131", profiles.Chrome_131}, {"Chrome_131_PSK", profiles.Chrome_131_PSK},
			{"Chrome_133", profiles.Chrome_133}, {"Chrome_133_PSK", profiles.Chrome_133_PSK},
			{"Chrome_116_PSK", profiles.Chrome_116_PSK}, {"Chrome_130_PSK", profiles.Chrome_130_PSK},
			{"Chrome_117", profiles.Chrome_117}, {"Chrome_120", profiles.Chrome_120},
		}
		for _, p := range list {
			probeWithProfile(p.name, p.pf, "POST", "https://api.piggo.me/v1/api/ping", "", "{}")
		}
		return
	}
	piggoKey := os.Args[1]
	fmt.Println("== ① 猪猪（tls-client·对照 curl_chrome116==200）==")
	probeWithProfile("Chrome_133", profiles.Chrome_133, "POST", "https://api.piggo.me/v1/api/ping", "", "{}")
	probeWithProfile("Chrome_133", profiles.Chrome_133, "POST", "https://api.piggo.me/v1/api/me", piggoKey, "{}")

	if len(os.Args) >= 4 {
		hdKey, hash := os.Args[2], os.Args[3]
		fmt.Println("== ② 好大 hashes POST（§59.1487 挂起现场·Go 标准栈曾 30s 超时）==")
		body := fmt.Sprintf(`{"hashes":["%s"]}`, hash)
		probeWithProfile("Chrome_133", profiles.Chrome_133, "POST", "https://hdarea.club/api/v1/torrents/hashes", hdKey, body)
		probeWithProfile("Chrome_133", profiles.Chrome_133, "GET", "https://hdarea.club/api/v1/status", hdKey, "")
	}
}
