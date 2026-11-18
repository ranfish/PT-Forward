package main

import (
	"fmt"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
)

func testDecryptOverride() {
	dbPath := os.Getenv("PTF_DB")
	if dbPath == "" {
		dbPath = "data/pt-forward.db"
	}
	db, _ := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	enc, _ := crypto.NewCredentialEncryptor(os.Getenv("ENCRYPTION_KEY"))
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
