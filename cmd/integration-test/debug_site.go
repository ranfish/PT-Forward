//go:build debug

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func debugSite() {
	siteName := "家园"
	if len(os.Args) > 1 {
		siteName = os.Args[1]
	}

	db, _ := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	enc, _ := crypto.NewCredentialEncryptor("e1cd1dc1423bb4f4ba4b79bf99a87e1d97ea3eba6459de33bccc4d37673499dc")
	crypto.RegisterCallbacks(db, enc, zap.NewNop())

	logger, _ := zap.NewProduction()
	provider := site.NewProvider(db, adapter.NewFactory(logger), logger)

	var s model.Site
	db.Where("name = ?", siteName).First(&s)
	fmt.Printf("Site: %s | Domain: %s\n", s.Name, s.Domain)
	fmt.Printf("  Passkey: %s\n", s.Passkey)
	fmt.Printf("  RSSKey: %s\n", s.RSSKey)
	fmt.Printf("  DownloadURLTemplate (sites): %s\n", s.DownloadURLTemplate)
	fmt.Printf("  DownloadMode: %s\n", s.DownloadMode)

	ctx := context.Background()
	config, err := provider.GetSiteConfig(ctx, s.Domain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetSiteConfig: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n--- SiteConfig (after overrides) ---\n")
	fmt.Printf("  Domain: %s\n", config.Domain)
	fmt.Printf("  Passkey: %s\n", config.Passkey)
	fmt.Printf("  RSSKey: %s\n", config.RSSKey)
	fmt.Printf("  DownloadURLTemplate: %s\n", config.DownloadURLTemplate)
	fmt.Printf("  DownloadMode: %s\n", config.DownloadMode)
	fmt.Printf("  Cookie (len): %d\n", len(config.Cookie))

	doer := adapter.NewHTTPDoerWithSite(config.ProxyURL, config.SkipSSLVerify)
	a := adapter.NewFactory(logger).Create(s.Framework, doer)

	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	data, err := a.DownloadTorrent(dlCtx, config, "294469")
	if err != nil {
		fmt.Printf("\nDownload ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nDownload OK: %d bytes\n", len(data))
}
