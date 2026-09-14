// searchprobe 诊断工具：生产同款 adapter 链路单发搜索。
// 用法: searchprobe -cookie '...' -kw '关键词' [-domain springsunday.net]
//       searchprobe -apikey '...' -kw '关键词' -mteam
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
	apikey := flag.String("apikey", "", "api key (mteam)")
	kw := flag.String("kw", "", "keyword")
	name := flag.String("name", "", "orphan name (derive keyword)")
	domain := flag.String("domain", "springsunday.net", "domain (nexusphp)")
	mteam := flag.Bool("mteam", false, "use mteam adapter")
	flag.Parse()

	keyword := *kw
	if *name != "" {
		keyword = reseed.ExtractSearchKeyword(*name)
		fmt.Fprintf(os.Stderr, "derived kw=%q hex=%x\n", keyword, keyword)
	}
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "keyword is empty")
		os.Exit(1)
	}

	if *mteam {
		cfg := &model.SiteConfig{Domain: "api.m-team.cc", BaseURL: "https://api.m-team.cc", APIKey: *apikey, Enabled: true}
		a := adapter.NewMTeamAdapter(adapter.NewHTTPDoer(), zap.NewNop())
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := a.SearchTorrents(ctx, cfg, keyword, nil)
		fmt.Fprintf(os.Stderr, "err=%v rows=%d\n", err, len(res))
		for i, r := range res {
			if i >= 8 {
				break
			}
			adult := ""
			if r.Adult {
				adult = " [ADULT]"
			}
			fmt.Printf("[%s] %s (%d)%s\n", r.TorrentID, r.Title, r.Size, adult)
		}
		return
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
