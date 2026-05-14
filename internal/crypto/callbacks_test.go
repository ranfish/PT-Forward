package crypto

import (
	"reflect"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testSite struct {
	ID      uint   `gorm:"primaryKey"`
	Name    string `gorm:"size:100"`
	Passkey string `encrypted:"true"`
	Cookie  string `encrypted:"true"`
	APIKey  string `encrypted:"true"`
	Plain   string
}

type testSiteNoEncrypt struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100"`
}

type testEmbedParent struct {
	ID       uint          `gorm:"primaryKey"`
	Name     string        `gorm:"size:100"`
	Embedded testEmbedChild `gorm:"embedded"`
}

type testEmbedChild struct {
	Secret string `encrypted:"true"`
}

func setupTestDB(t *testing.T) (*gorm.DB, *CredentialEncryptor) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&testSite{}, &testSiteNoEncrypt{}, &testEmbedParent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	enc, err := NewCredentialEncryptor("test-encryption-key-32bytes!!!")
	if err != nil {
		t.Fatalf("create encryptor: %v", err)
	}
	RegisterCallbacks(db, enc, zap.NewNop())
	return db, enc
}

func TestRegisterCallbacks_CreateEncryptsFields(t *testing.T) {
	db, enc := setupTestDB(t)

	site := &testSite{
		Name:    "test-site",
		Passkey: "my-secret-passkey",
		Cookie:  "session=abc123",
		APIKey:  "",
		Plain:   "not-encrypted",
	}

	if err := db.Create(site).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var fetched testSite
	if err := db.First(&fetched, site.ID).Error; err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if fetched.Passkey != "my-secret-passkey" {
		t.Errorf("Passkey not decrypted: got %q", fetched.Passkey)
	}
	if fetched.Cookie != "session=abc123" {
		t.Errorf("Cookie not decrypted: got %q", fetched.Cookie)
	}
	if fetched.APIKey != "" {
		t.Errorf("empty APIKey should remain empty: got %q", fetched.APIKey)
	}
	if fetched.Plain != "not-encrypted" {
		t.Errorf("Plain field should not be affected: got %q", fetched.Plain)
	}

	var raw testSite
	db.Raw("SELECT * FROM test_sites WHERE id = ?", site.ID).Scan(&raw)
	if !enc.IsEncrypted(raw.Passkey) {
		t.Error("Passkey should be encrypted in raw DB")
	}
	if !enc.IsEncrypted(raw.Cookie) {
		t.Error("Cookie should be encrypted in raw DB")
	}
}

func TestRegisterCallbacks_UpdateEncryptsFields(t *testing.T) {
	db, _ := setupTestDB(t)

	site := &testSite{Name: "test", Passkey: "original", Cookie: "orig-cookie", Plain: "keep"}
	db.Create(site)

	db.Model(site).Update("passkey", "updated-passkey")

	var fetched testSite
	db.First(&fetched, site.ID)

	if fetched.Passkey != "updated-passkey" {
		t.Errorf("updated Passkey not decrypted: got %q", fetched.Passkey)
	}
	if fetched.Cookie != "orig-cookie" {
		t.Errorf("Cookie should be unchanged: got %q", fetched.Cookie)
	}
}

func TestRegisterCallbacks_QueryDecryptsSlice(t *testing.T) {
	db, _ := setupTestDB(t)

	db.Create(&testSite{Name: "a", Passkey: "pass-a", Cookie: "c-a"})
	db.Create(&testSite{Name: "b", Passkey: "pass-b", Cookie: "c-b"})

	var sites []testSite
	if err := db.Find(&sites).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2, got %d", len(sites))
	}
	for _, s := range sites {
		if len(s.Passkey) < 5 || s.Passkey[:5] != "pass-" {
			t.Errorf("Passkey not decrypted: %q", s.Passkey)
		}
	}
}

func TestRegisterCallbacks_NoEncryptModel_NoPanic(t *testing.T) {
	db, _ := setupTestDB(t)

	ne := &testSiteNoEncrypt{Name: "plain"}
	if err := db.Create(ne).Error; err != nil {
		t.Fatalf("create no-encrypt: %v", err)
	}
	var fetched testSiteNoEncrypt
	if err := db.First(&fetched, ne.ID).Error; err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if fetched.Name != "plain" {
		t.Errorf("Name mismatch: got %q", fetched.Name)
	}
}

func TestDecryptValue_PtrToStruct(t *testing.T) {
	db, _ := setupTestDB(t)

	site := &testSite{Name: "ptr-test", Passkey: "secret123"}
	db.Create(site)

	var fetched testSite
	if err := db.First(&fetched, site.ID).Error; err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if fetched.Passkey != "secret123" {
		t.Errorf("Passkey not decrypted: got %q", fetched.Passkey)
	}
}

func TestHasEncryptedFields_WithEncryptedTag(t *testing.T) {
	typ := reflect.TypeOf(testSite{})
	if !hasEncryptedFields(typ) {
		t.Error("testSite should have encrypted fields")
	}
}

func TestHasEncryptedFields_WithoutEncryptedTag(t *testing.T) {
	typ := reflect.TypeOf(testSiteNoEncrypt{})
	if hasEncryptedFields(typ) {
		t.Error("testSiteNoEncrypt should not have encrypted fields")
	}
}

func TestHasEncryptedFields_GormEmbeddedTag(t *testing.T) {
	typ := reflect.TypeOf(testEmbedParent{})
	if hasEncryptedFields(typ) {
		t.Error("gorm:embedded tag does NOT make a field anonymous in reflect; this should return false")
	}
}

func TestHasEncryptedFields_AnonymousEmbed(t *testing.T) {
	type inner struct {
		Secret string `encrypted:"true"`
	}
	type outer struct {
		Name string
		inner
	}
	typ := reflect.TypeOf(outer{})
	if !hasEncryptedFields(typ) {
		t.Error("struct with anonymous embedded struct with encrypted tag should return true")
	}
}

func TestEncryptStruct_EmbeddedField(t *testing.T) {
	db, _ := setupTestDB(t)

	parent := &testEmbedParent{
		Name:     "embed-test",
		Embedded: testEmbedChild{Secret: "embedded-secret"},
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create embedded: %v", err)
	}

	var fetched testEmbedParent
	if err := db.First(&fetched, parent.ID).Error; err != nil {
		t.Fatalf("fetch embedded: %v", err)
	}
	if fetched.Embedded.Secret != "embedded-secret" {
		t.Errorf("embedded Secret not decrypted: got %q", fetched.Embedded.Secret)
	}
}

func TestEncryptStruct_SkipsAlreadyEncrypted(t *testing.T) {
	enc, _ := NewCredentialEncryptor("test-encryption-key-32bytes!!!")
	already, _ := enc.Encrypt("already-encrypted")

	db, _ := setupTestDB(t)
	site := &testSite{Name: "skip", Passkey: already}
	db.Create(site)

	var raw testSite
	db.Raw("SELECT * FROM test_sites WHERE id = ?", site.ID).Scan(&raw)
	if raw.Passkey != already {
		t.Error("already encrypted value should not be double-encrypted")
	}
}

func TestGetEncryptedModels_ContainsExpectedTables(t *testing.T) {
	models := getEncryptedModels()
	tables := make(map[string][]string)
	for _, m := range models {
		tables[m.TableName] = m.Columns
	}
	if cols, ok := tables["sites"]; !ok {
		t.Error("missing sites table")
	} else {
		expectedCols := []string{"passkey", "cookie", "api_key"}
		for _, ec := range expectedCols {
			found := false
			for _, c := range cols {
				if c == ec {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("sites table missing column %q", ec)
			}
		}
	}
	if _, ok := tables["clients"]; !ok {
		t.Error("missing clients table")
	}
}

func TestMigratePlaintext_NoPlaintext(t *testing.T) {
	db, enc := setupTestDB(t)
	err := MigratePlaintext(db, enc, zap.NewNop())
	if err != nil {
		t.Fatalf("MigratePlaintext returned error: %v", err)
	}
}

func TestMigratePlaintext_EmptyTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	db.Exec("CREATE TABLE IF NOT EXISTS sites (id INTEGER PRIMARY KEY, name TEXT, passkey TEXT, cookie TEXT, api_key TEXT, bearer_token TEXT, auth_key TEXT, auth_hash TEXT, rss_key TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS clients (id INTEGER PRIMARY KEY, name TEXT, password TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS cookie_cloud_configs (id INTEGER PRIMARY KEY, password TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS iyuu_configs (id INTEGER PRIMARY KEY, token TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS notification_channels (id INTEGER PRIMARY KEY, config TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS site_config_overrides (id INTEGER PRIMARY KEY, field_value TEXT)")
	defer func() {
		db.Exec("DROP TABLE IF EXISTS sites")
		db.Exec("DROP TABLE IF EXISTS clients")
		db.Exec("DROP TABLE IF EXISTS cookie_cloud_configs")
		db.Exec("DROP TABLE IF EXISTS iyuu_configs")
		db.Exec("DROP TABLE IF EXISTS notification_channels")
		db.Exec("DROP TABLE IF EXISTS site_config_overrides")
	}()

	enc, encErr := NewCredentialEncryptor("test-encryption-key-32bytes!!!")
	if encErr != nil {
		t.Fatalf("create encryptor: %v", encErr)
	}

	err = MigratePlaintext(db, enc, zap.NewNop())
	if err != nil {
		t.Fatalf("MigratePlaintext on empty tables returned error: %v", err)
	}
}

func TestMigratePlaintext_SkipsAlreadyEncrypted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	db.Exec("CREATE TABLE IF NOT EXISTS sites (id INTEGER PRIMARY KEY, name TEXT, passkey TEXT, cookie TEXT, api_key TEXT, bearer_token TEXT, auth_key TEXT, auth_hash TEXT, rss_key TEXT)")
	defer db.Exec("DROP TABLE IF EXISTS sites")

	enc, encErr := NewCredentialEncryptor("test-encryption-key-32bytes!!!")
	if encErr != nil {
		t.Fatalf("create encryptor: %v", encErr)
	}

	encValue, _ := enc.Encrypt("already-encrypted")
	db.Exec("INSERT INTO sites (name, passkey) VALUES ('s1', ?)", encValue)

	err = MigratePlaintext(db, enc, zap.NewNop())
	if err != nil {
		t.Fatalf("MigratePlaintext returned error: %v", err)
	}

	type siteRow struct {
		Passkey string
	}
	var site siteRow
	db.Table("sites").Where("name = ?", "s1").Select("passkey").Scan(&site)
	if site.Passkey != encValue {
		t.Error("already encrypted value was modified")
	}
}

func TestMigratePlaintext_MigratesPlaintext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	db.Exec("CREATE TABLE IF NOT EXISTS sites (id INTEGER PRIMARY KEY, name TEXT, passkey TEXT, cookie TEXT, api_key TEXT, bearer_token TEXT, auth_key TEXT, auth_hash TEXT, rss_key TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS clients (id INTEGER PRIMARY KEY, name TEXT, password TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS cookie_cloud_configs (id INTEGER PRIMARY KEY, password TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS iyuu_configs (id INTEGER PRIMARY KEY, token TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS notification_channels (id INTEGER PRIMARY KEY, config TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS site_config_overrides (id INTEGER PRIMARY KEY, field_value TEXT)")
	defer func() {
		db.Exec("DROP TABLE IF EXISTS sites")
		db.Exec("DROP TABLE IF EXISTS clients")
		db.Exec("DROP TABLE IF EXISTS cookie_cloud_configs")
		db.Exec("DROP TABLE IF EXISTS iyuu_configs")
		db.Exec("DROP TABLE IF EXISTS notification_channels")
		db.Exec("DROP TABLE IF EXISTS site_config_overrides")
	}()

	enc, encErr := NewCredentialEncryptor("test-encryption-key-32bytes!!!")
	if encErr != nil {
		t.Fatalf("create encryptor: %v", encErr)
	}

	db.Exec("INSERT INTO sites (name, passkey, cookie) VALUES ('plain-site', 'plaintext-pass', 'plaintext-cookie')")
	db.Exec("INSERT INTO sites (name, passkey, cookie) VALUES ('mixed-site', 'another-pass', '')")
	db.Exec("INSERT INTO clients (name, password) VALUES ('test-client', 'client-secret')")

	err = MigratePlaintext(db, enc, zap.NewNop())
	if err != nil {
		t.Fatalf("MigratePlaintext returned error: %v", err)
	}

	type siteRow struct {
		Passkey string
		Cookie  string
	}
	var s1 siteRow
	db.Table("sites").Where("name = ?", "plain-site").Select("passkey, cookie").Scan(&s1)
	if !enc.IsEncrypted(s1.Passkey) {
		t.Error("plain-site passkey should be encrypted after migration")
	}
	if !enc.IsEncrypted(s1.Cookie) {
		t.Error("plain-site cookie should be encrypted after migration")
	}
	decrypted, _ := enc.Decrypt(s1.Passkey)
	if decrypted != "plaintext-pass" {
		t.Errorf("decrypted passkey mismatch: got %q", decrypted)
	}

	var s2 siteRow
	db.Table("sites").Where("name = ?", "mixed-site").Select("passkey, cookie").Scan(&s2)
	if !enc.IsEncrypted(s2.Passkey) {
		t.Error("mixed-site passkey should be encrypted")
	}

	type clientRow struct {
		Password string
	}
	var c1 clientRow
	db.Table("clients").Where("name = ?", "test-client").Select("password").Scan(&c1)
	if !enc.IsEncrypted(c1.Password) {
		t.Error("client password should be encrypted after migration")
	}
	decryptedClient, _ := enc.Decrypt(c1.Password)
	if decryptedClient != "client-secret" {
		t.Errorf("decrypted client password mismatch: got %q", decryptedClient)
	}
}

func TestMigratePlaintext_SkipsNonexistentTable(t *testing.T) {
	db, enc := setupTestDB(t)

	err := MigratePlaintext(db, enc, zap.NewNop())
	if err != nil {
		t.Fatalf("MigratePlaintext on test DB should not fail: %v", err)
	}
}
