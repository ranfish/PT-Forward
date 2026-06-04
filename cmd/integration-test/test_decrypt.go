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
	db, _ := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	enc, _ := crypto.NewCredentialEncryptor("e1cd1dc1423bb4f4ba4b79bf99a87e1d97ea3eba6459de33bccc4d37673499dc")
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
