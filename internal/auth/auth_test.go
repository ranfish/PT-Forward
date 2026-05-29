package auth

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newAuthManager(t *testing.T) (*AuthManager, *gorm.DB) {
	t.Helper()
	db := setupAuthDB(t)
	repo := NewGormAuthRepository(db)
	mgr, err := NewAuthManager(repo, zap.NewNop())
	if err != nil {
		t.Fatalf("NewAuthManager: %v", err)
	}
	return mgr, db
}

func createAdminUser(t *testing.T, db *gorm.DB, password string) {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	db.Create(&model.User{
		Username:     "admin",
		DisplayName:  "admin",
		PasswordHash: hash,
	})
}

func TestAuthManager_Login_Success(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	pair, err := mgr.Login(context.Background(), "admin", "Admin@123", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if pair.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}
	if pair.ExpiresIn <= 0 {
		t.Error("expires in should be positive")
	}
}

func TestAuthManager_Login_WrongUsername(t *testing.T) {
	mgr, _ := newAuthManager(t)
	_, err := mgr.Login(context.Background(), "wronguser", "pass", "1.2.3.4")
	if err == nil {
		t.Fatal("should fail with wrong username")
	}
	appErr, ok := err.(*model.AppError)
	if !ok || appErr.Code != 40100 {
		t.Errorf("expected AppError 40100, got %v", err)
	}
}

func TestAuthManager_Login_WrongPassword(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	_, err := mgr.Login(context.Background(), "admin", "wrong", "1.2.3.4")
	if err == nil {
		t.Fatal("should fail with wrong password")
	}
}

func TestAuthManager_Login_Lockout(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	for i := 0; i < 5; i++ {
		_, _ = mgr.Login(context.Background(), "admin", "wrong", "10.0.0.1")
	}

	_, err := mgr.Login(context.Background(), "admin", "Admin@123", "10.0.0.1")
	if err == nil {
		t.Fatal("should be locked after 5 failures")
	}
	appErr, ok := err.(*model.AppError)
	if !ok || appErr.Code != 42901 {
		t.Errorf("expected AppError 42901 (lockout), got %v", err)
	}
}

func TestAuthManager_TokenPair(t *testing.T) {
	mgr, _ := newAuthManager(t)
	pair, err := mgr.IssueTokenPair()
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	claims, err := mgr.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.Sub != "admin" {
		t.Errorf("expected sub=admin, got %s", claims.Sub)
	}
	if claims.Type != "access" {
		t.Errorf("expected type=access, got %s", claims.Type)
	}
}

func TestAuthManager_RefreshTokens(t *testing.T) {
	mgr, _ := newAuthManager(t)
	pair, err := mgr.IssueTokenPair()
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	newPair, err := mgr.RefreshTokens(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshTokens: %v", err)
	}
	if newPair.AccessToken == "" {
		t.Error("new access token should not be empty")
	}

	_, err = mgr.RefreshTokens(pair.RefreshToken)
	if err == nil {
		t.Error("reusing old refresh token should fail")
	}
}

func TestAuthManager_RefreshTokens_InvalidToken(t *testing.T) {
	mgr, _ := newAuthManager(t)
	_, err := mgr.RefreshTokens("invalid.token.here")
	if err == nil {
		t.Error("should fail with invalid token")
	}
}

func TestAuthManager_ValidateAccessToken_Invalid(t *testing.T) {
	mgr, _ := newAuthManager(t)
	_, err := mgr.ValidateAccessToken("invalid")
	if err == nil {
		t.Error("should fail with invalid token")
	}
}

func TestAuthManager_ValidateAccessToken_RefreshToken(t *testing.T) {
	mgr, _ := newAuthManager(t)
	pair, _ := mgr.IssueTokenPair()
	_, err := mgr.ValidateAccessToken(pair.RefreshToken)
	if err == nil {
		t.Error("refresh token should not validate as access token")
	}
}

func TestAuthManager_RevokeAllRefreshTokens(t *testing.T) {
	mgr, _ := newAuthManager(t)
	pair, _ := mgr.IssueTokenPair()

	mgr.RevokeAllRefreshTokens()

	_, err := mgr.RefreshTokens(pair.RefreshToken)
	if err == nil {
		t.Error("should fail after revocation")
	}
}

func TestAuthManager_VerifyPassword(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	err := mgr.VerifyPassword(context.Background(), "Admin@123")
	if err != nil {
		t.Errorf("correct password should verify: %v", err)
	}

	err = mgr.VerifyPassword(context.Background(), "wrong")
	if err == nil {
		t.Error("wrong password should fail")
	}
}

func TestAuthManager_SetPassword(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	err := mgr.SetPassword(context.Background(), "NewP@ss1!")
	if err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	err = mgr.VerifyPassword(context.Background(), "NewP@ss1!")
	if err != nil {
		t.Errorf("new password should verify: %v", err)
	}
}

func TestAuthManager_SetPassword_Weak(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	err := mgr.SetPassword(context.Background(), "weak")
	if err == nil {
		t.Error("weak password should be rejected")
	}
}

func TestAuthManager_IsInitialized(t *testing.T) {
	mgr, db := newAuthManager(t)

	if mgr.IsInitialized(context.Background()) {
		t.Error("should not be initialized without admin user")
	}

	createAdminUser(t, db, "Admin@123")
	if !mgr.IsInitialized(context.Background()) {
		t.Error("should be initialized after creating admin user")
	}
}

func TestAuthManager_GetUserInfo(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	user, err := mgr.GetUserInfo(context.Background())
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("expected admin, got %s", user.Username)
	}
}

func TestAuthManager_UpdateProfile(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	err := mgr.UpdateProfile(context.Background(), "New Display Name")
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	user, _ := mgr.GetUserInfo(context.Background())
	if user.DisplayName != "New Display Name" {
		t.Errorf("expected New Display Name, got %s", user.DisplayName)
	}
}

func TestAuthManager_ResetPassword(t *testing.T) {
	mgr, db := newAuthManager(t)
	createAdminUser(t, db, "Admin@123")

	err := mgr.ResetPassword(context.Background(), "ResetP@ss1")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	err = mgr.VerifyPassword(context.Background(), "ResetP@ss1")
	if err != nil {
		t.Errorf("reset password should verify: %v", err)
	}
}

func TestEnsureAdminUser_CreateNew(t *testing.T) {
	db := setupAuthDB(t)
	repo := NewGormAuthRepository(db)
	logger := zap.NewNop()

	err := EnsureAdminUser(context.Background(), repo, logger)
	if err != nil {
		t.Fatalf("EnsureAdminUser: %v", err)
	}

	user, err := repo.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("admin user should exist: %v", err)
	}
	if user.PasswordHash == "" {
		t.Error("password hash should not be empty")
	}
}

func TestEnsureAdminUser_AlreadyExists(t *testing.T) {
	db := setupAuthDB(t)
	repo := NewGormAuthRepository(db)
	logger := zap.NewNop()

	hash, _ := HashPassword("Existing@123")
	db.Create(&model.User{Username: "admin", DisplayName: "admin", PasswordHash: hash})

	err := EnsureAdminUser(context.Background(), repo, logger)
	if err != nil {
		t.Fatalf("should not error when admin exists: %v", err)
	}

	user, _ := repo.GetByUsername(context.Background(), "admin")
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("Existing@123")); err != nil {
		t.Error("existing password should be unchanged")
	}
}

func TestLoginLimiter(t *testing.T) {
	limiter := &loginLimiter{enabled: true, maxRetries: 5, lockoutMin: 5, attempts: make(map[string]*loginAttempt)}

	if err := limiter.CheckLocked("1.1.1.1"); err != nil {
		t.Error("should not be locked initially")
	}

	for i := 0; i < 5; i++ {
		limiter.RecordFailure("1.1.1.1")
	}

	if err := limiter.CheckLocked("1.1.1.1"); err == nil {
		t.Error("should be locked after 5 failures")
	}

	limiter.RecordSuccess("1.1.1.1")
	if err := limiter.CheckLocked("1.1.1.1"); err != nil {
		t.Error("should be unlocked after success")
	}
}

func TestLoginLimiter_Disabled(t *testing.T) {
	limiter := &loginLimiter{enabled: false, maxRetries: 5, lockoutMin: 5, attempts: make(map[string]*loginAttempt)}

	for i := 0; i < 10; i++ {
		limiter.RecordFailure("1.1.1.1")
	}
	if err := limiter.CheckLocked("1.1.1.1"); err != nil {
		t.Error("should never be locked when disabled")
	}
}

func TestAccessTokenClaims_MapClaims(t *testing.T) {
	claims := AccessTokenClaims{
		Sub:  "admin",
		Type: "access",
		Iat:  1000,
		Exp:  2000,
		Jti:  "test-jti",
	}
	mc := claims.MapClaims()
	if mc["sub"] != "admin" {
		t.Error("sub mismatch")
	}
	if mc["jti"] != "test-jti" {
		t.Error("jti mismatch")
	}
}

func TestEvictOldestIfNeeded(t *testing.T) {
	mgr, _ := newAuthManager(t)
	for i := 0; i < 12; i++ {
		_, _ = mgr.IssueTokenPair()
	}
	mgr.mu.RLock()
	count := len(mgr.refreshTokens)
	mgr.mu.RUnlock()
	if count > 10 {
		t.Errorf("should not exceed maxRefreshTokens=10, got %d", count)
	}
}

type mockSettingStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMockSettingStore() *mockSettingStore {
	return &mockSettingStore{data: make(map[string]string)}
}

func (m *mockSettingStore) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return v, nil
}

func (m *mockSettingStore) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockSettingStore) waitFor(key string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.mu.RLock()
		_, ok := m.data[key]
		m.mu.RUnlock()
		if ok {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func newAuthManagerWithSettings(t *testing.T) (*AuthManager, *gorm.DB, *mockSettingStore) {
	t.Helper()
	db := setupAuthDB(t)
	repo := NewGormAuthRepository(db)
	store := newMockSettingStore()
	mgr, err := NewAuthManagerWithSettings(repo, store, zap.NewNop())
	if err != nil {
		t.Fatalf("NewAuthManagerWithSettings: %v", err)
	}
	return mgr, db, store
}

func TestAuthManager_PersistentSigningKey(t *testing.T) {
	mgr, _, store := newAuthManagerWithSettings(t)

	_, ok := store.data["jwt_signing_key"]
	if !ok {
		t.Fatal("signing key should be persisted")
	}

	store2 := newMockSettingStore()
	store2.data["jwt_signing_key"] = store.data["jwt_signing_key"]
	repo2 := NewGormAuthRepository(setupAuthDB(t))
	mgr2, err := NewAuthManagerWithSettings(repo2, store2, zap.NewNop())
	if err != nil {
		t.Fatalf("NewAuthManagerWithSettings reload: %v", err)
	}

	pair, err := mgr.IssueTokenPair()
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	claims, err := mgr2.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("token from mgr should validate under mgr2 with same key: %v", err)
	}
	if claims.Sub != "admin" {
		t.Errorf("expected sub=admin, got %s", claims.Sub)
	}
}

func TestAuthManager_PersistentRefreshTokens(t *testing.T) {
	mgr, _, store := newAuthManagerWithSettings(t)

	pair, err := mgr.IssueTokenPair()
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	if !store.waitFor("refresh_tokens", 2*time.Second) {
		t.Fatal("timed out waiting for refresh_tokens persist")
	}

	store.mu.RLock()
	jwtKey := store.data["jwt_signing_key"]
	rtData := store.data["refresh_tokens"]
	store.mu.RUnlock()

	store2 := newMockSettingStore()
	store2.mu.Lock()
	store2.data["jwt_signing_key"] = jwtKey
	store2.data["refresh_tokens"] = rtData
	store2.mu.Unlock()
	repo2 := NewGormAuthRepository(setupAuthDB(t))
	mgr2, err := NewAuthManagerWithSettings(repo2, store2, zap.NewNop())
	if err != nil {
		t.Fatalf("NewAuthManagerWithSettings reload: %v", err)
	}

	newPair, err := mgr2.RefreshTokens(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshTokens with restored state: %v", err)
	}
	if newPair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

func TestAuthManager_SigningKeyRotationBreaksOldTokens(t *testing.T) {
	mgr, _, _ := newAuthManagerWithSettings(t)
	pair, _ := mgr.IssueTokenPair()

	freshStore := newMockSettingStore()
	repo2 := NewGormAuthRepository(setupAuthDB(t))
	mgr2, _ := NewAuthManagerWithSettings(repo2, freshStore, zap.NewNop())

	_, err := mgr2.ValidateAccessToken(pair.AccessToken)
	if err == nil {
		t.Error("token signed with old key should not validate under new key")
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	password := "TestP@ss1"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	if hash == password {
		t.Fatal("hash should not equal plaintext")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "MyStr0ng!Pass"
	hash, _ := HashPassword(password)

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Fatalf("password should match: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrong")); err == nil {
		t.Fatal("wrong password should not match")
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	pwd, err := GenerateRandomPassword(12)
	if err != nil {
		t.Fatalf("GenerateRandomPassword failed: %v", err)
	}
	if len(pwd) != 12 {
		t.Fatalf("expected length 12, got %d", len(pwd))
	}
}

func TestGenerateRandomPassword_Uniqueness(t *testing.T) {
	pwd1, _ := GenerateRandomPassword(16)
	pwd2, _ := GenerateRandomPassword(16)
	if pwd1 == pwd2 {
		t.Fatal("two generated passwords should differ")
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name    string
		pwd     string
		wantErr bool
	}{
		{"valid", "Abc123!@", false},
		{"too short", "A1!", true},
		{"no upper", "abc123!@", true},
		{"no lower", "ABC123!@", true},
		{"no digit", "Abcdef!@", true},
		{"no special", "Abc12345", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.pwd)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePasswordStrength(%q) = %v, wantErr %v", tt.pwd, err, tt.wantErr)
			}
		})
	}
}
