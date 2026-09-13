// searchprobe 诊断工具：生产同款链路（ExtractSearchKeyword→adapter 搜索）。
// 用法: searchprobe -cookie '...' -name '密阳.2007...' 或 -kw '密阳 2007'
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/reseed"
	"go.uber.org/zap"
)

func main() {
	cookie := flag.String("cookie", "", "cookie string")
	kw := flag.String("kw", "", "keyword (direct)")
	name := flag.String("name", "", "orphan name (derive keyword)")
	domain := flag.String("domain", "springsunday.net", "domain")
	flag.Parse()
	keyword := *kw
	if *name != "" {
		keyword = reseed.ExtractSearchKeyword(*name)
		fmt.Fprintf(os.Stderr, "derived kw=%q hex=%x\n", keyword, keyword)
	}
	a := adapter.NewNexusPHPAdapter(adapter.NewHTTPDoer(), zap.NewNop())
	cfg := &model.SiteConfig{Domain: *domain, Cookie: *cookie, Enabled: true}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := a.SearchTorrents(ctx, cfg, keyword, nil)
	fmt.Fprintf(os.Stderr, "err=%v rows=%d\n", err, len(res))
	for i, r := range res {
		if i >= 5 {
			break
		}
		fmt.Printf("[%s] %s (%d)\n", r.TorrentID, r.Title, r.Size)
	}
}
