package main

import (
	"fmt"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDecryptOverride() {
	db, _ := gorm.Open(sqlite.Open("/home/<user>/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	enc, _ := crypto.NewCredentialEncryptor("[REDACTED-enc_key]")
	crypto.RegisterCallbacks(db, enc, zap.NewNop())

	// Test override decrypt
	var o model.SiteConfigOverride
	db.Where("site_name = ? AND field_path = ?", "家园", "download_url_template").First(&o)
	fmt.Printf("Override download_url_template: %q\n", o.FieldValue)

	// Test site rss_key decrypt
	var s model.Site
	db.Where("name = ?", "家园").First(&s)
	fmt.Printf("Site passkey: %q\n", s.Passkey)
	fmt.Printf("Site rss_key: %q\n", s.RSSKey)
}
