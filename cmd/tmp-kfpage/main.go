package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/httpclient"
)

func main() {
	keyB, _ := os.ReadFile("/tmp/opencode/kf_enckey.txt")
	enc, _ := crypto.NewCredentialEncryptor(strings.TrimSpace(string(keyB)))
	raw, _ := os.ReadFile("/tmp/opencode/multi_cookies.tsv")
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	type job struct{ name, domain, tid string }
	jobs := []job{
		{"修道院", "xdypt.vip", "3240"},
		{"幸运", "pt.luckpt.de", "57799"},
		{"憨憨", "hhanclub.net", "152467"},
		{"织梦", "zmpt.cc", "485659"},
	}
	cookieMap := map[string]string{}
	for _, ln := range lines {
		parts := strings.SplitN(ln, "|", 2)
		if len(parts) == 2 {
			if ck, err := enc.Decrypt(strings.TrimSpace(parts[1])); err == nil {
				cookieMap[parts[0]] = ck
			} else {
				fmt.Println("解密失败:", parts[0], err)
			}
		}
	}
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{})
	for _, j := range jobs {
		ck, ok := cookieMap[j.name]
		if !ok {
			fmt.Println(j.name, "无 cookie")
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET",
			"https://"+j.domain+"/details.php?id="+j.tid, nil)
		req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
		req.Header.Set("Cookie", ck)
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(j.name, "ERR:", err)
			cancel()
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		cancel()
		os.WriteFile("/tmp/opencode/site_"+j.name+".html", body, 0644)
		fmt.Printf("%s(%s): status=%d bytes=%d\n", j.name, j.domain, resp.StatusCode, len(body))
		time.Sleep(3 * time.Second)
	}
}
