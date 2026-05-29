package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenDuration  = 30 * time.Minute
	refreshTokenDuration = 7 * 24 * time.Hour
	maxRefreshTokens     = 10
	bcryptCost           = 12
	randomPasswordChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
)

type ctxKey string

const CtxKeyUserID ctxKey = "user_id"

type AccessTokenClaims struct {
	Sub  string `json:"sub"`
	Type string `json:"type"`
	Iat  int64  `json:"iat"`
	Exp  int64  `json:"exp"`
	Jti  string `json:"jti"`
}

func (c AccessTokenClaims) MapClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"sub":  c.Sub,
		"type": c.Type,
		"iat":  c.Iat,
		"exp":  c.Exp,
		"jti":  c.Jti,
	}
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

type loginAttempt struct {
	count       int
	lockedUntil time.Time
}

type loginLimiter struct {
	enabled    bool
	maxRetries int
	lockoutMin int
	attempts   map[string]*loginAttempt
	mu         sync.RWMutex
	stopCh     chan struct{}
}

func (l *loginLimiter) CheckLocked(ip string) error {
	if !l.enabled {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	a, ok := l.attempts[ip]
	if !ok {
		return nil
	}
	if a.count >= l.maxRetries && time.Now().Before(a.lockedUntil) {
		return authError(ErrAuthLogin, fmt.Sprintf("登录已锁定，请 %s 后重试", time.Until(a.lockedUntil).Round(time.Minute)), nil)
	}
	return nil
}

func (l *loginLimiter) RecordFailure(ip string) {
	if !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.attempts[ip]
	if a == nil {
		a = &loginAttempt{}
		l.attempts[ip] = a
	}
	if a.count >= l.maxRetries && time.Now().After(a.lockedUntil) {
		a.count = 0
	}
	a.count++
	if a.count >= l.maxRetries {
		a.lockedUntil = time.Now().Add(time.Duration(l.lockoutMin) * time.Minute)
	}
}

func (l *loginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

func (l *loginLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for ip, a := range l.attempts {
				if a.count < l.maxRetries || now.After(a.lockedUntil) {
					delete(l.attempts, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

func (l *loginLimiter) stop() {
	select {
	case <-l.stopCh:
	default:
		close(l.stopCh)
	}
}

type AuthManager struct {
	signingKey    []byte
	refreshTokens map[string]time.Time
	loginLimiter  *loginLimiter
	repo          model.AuthRepository
	settingRepo   SettingStore
	logger        *zap.Logger
	mu            sync.RWMutex
	setupOnce     sync.Once

	persistCh  chan struct{}
	persistWg  sync.WaitGroup
	stopOnce   sync.Once
	stopCh     chan struct{}
}

type SettingStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

func NewAuthManager(repo model.AuthRepository, logger *zap.Logger) (*AuthManager, error) {
	return NewAuthManagerWithSettings(repo, nil, logger)
}

func NewAuthManagerWithSettings(repo model.AuthRepository, settingRepo SettingStore, logger *zap.Logger) (*AuthManager, error) {
	var key []byte

	if settingRepo != nil {
		ctx := context.Background()
		stored, err := settingRepo.Get(ctx, "jwt_signing_key")
		if err == nil && stored != "" {
			decoded, decErr := base64.StdEncoding.DecodeString(stored)
			if decErr == nil && len(decoded) == 32 {
				key = decoded
				logger.Info("JWT signing key loaded from persistent storage")
			}
		}
	}

	if key == nil {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, authError(ErrAuthJWT, "JWT signing key generation failed", err)
		}
		if settingRepo != nil {
			ctx := context.Background()
			encoded := base64.StdEncoding.EncodeToString(key)
			if err := settingRepo.Set(ctx, "jwt_signing_key", encoded); err != nil {
				logger.Warn("failed to persist JWT signing key", zap.Error(err))
			}
		}
	}

	mgr := &AuthManager{
		signingKey:    key,
		refreshTokens: make(map[string]time.Time),
		loginLimiter: &loginLimiter{
			enabled:    true,
			maxRetries: 5,
			lockoutMin: 5,
			attempts:   make(map[string]*loginAttempt),
			stopCh:     make(chan struct{}),
		},
		repo:        repo,
		settingRepo: settingRepo,
		logger:      logger,
		persistCh:   make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
	}

	go mgr.loginLimiter.cleanupLoop()
	go mgr.persistLoop()

	if settingRepo != nil {
		mgr.loadRefreshTokens(context.Background())
	}

	return mgr, nil
}

func (m *AuthManager) ConfigureLoginLockout(enabled bool, maxRetries, lockoutMin int) {
	m.loginLimiter.mu.Lock()
	m.loginLimiter.enabled = enabled
	if maxRetries > 0 {
		m.loginLimiter.maxRetries = maxRetries
	}
	if lockoutMin > 0 {
		m.loginLimiter.lockoutMin = lockoutMin
	}
	m.loginLimiter.mu.Unlock()
}

func (m *AuthManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.loginLimiter.stop()
	m.persistWg.Wait()
}

func (m *AuthManager) persistLoop() {
	for {
		select {
		case <-m.stopCh:
			m.persistRefreshTokens()
			return
		case <-m.persistCh:
			select {
			case <-m.stopCh:
				m.persistRefreshTokens()
				return
			case <-time.After(500 * time.Millisecond):
			}
			m.persistRefreshTokens()
		}
	}
}

func (m *AuthManager) triggerPersist() {
	select {
	case m.persistCh <- struct{}{}:
	default:
	}
}

func (m *AuthManager) loadRefreshTokens(ctx context.Context) {
	if m.settingRepo == nil {
		return
	}
	stored, err := m.settingRepo.Get(ctx, "refresh_tokens")
	if err != nil || stored == "" {
		return
	}
	var tokens map[string]int64
	if err := json.Unmarshal([]byte(stored), &tokens); err != nil {
		return
	}
	now := time.Now()
	for jti, expUnix := range tokens {
		exp := time.Unix(expUnix, 0)
		if exp.After(now) {
			m.refreshTokens[jti] = exp
		}
	}
	m.logger.Info("refresh tokens loaded from persistent storage", zap.Int("count", len(m.refreshTokens)))
}

func (m *AuthManager) persistRefreshTokens() {
	if m.settingRepo == nil {
		return
	}
	m.mu.RLock()
	tokens := make(map[string]int64, len(m.refreshTokens))
	for jti, exp := range m.refreshTokens {
		tokens[jti] = exp.Unix()
	}
	m.mu.RUnlock()

	data, err := json.Marshal(tokens)
	if err != nil {
		m.logger.Warn("failed to marshal refresh tokens", zap.Error(err))
		return
	}
	ctx := context.Background()
	if err := m.settingRepo.Set(ctx, "refresh_tokens", string(data)); err != nil {
		m.logger.Warn("failed to persist refresh tokens", zap.Error(err))
	}
}

func (m *AuthManager) Login(ctx context.Context, username, password, clientIP string) (*TokenPair, error) {
	if err := m.loginLimiter.CheckLocked(clientIP); err != nil {
		m.logger.Warn("login locked",
			zap.String("ip", clientIP),
		)
		return nil, &model.AppError{Code: 42901, Message: err.Error()}
	}

	if username != "admin" {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummyhashfortimingpadding000000000000000000000000000000000"), []byte(password))
		m.loginLimiter.RecordFailure(clientIP)
		return nil, &model.AppError{Code: 40100, Message: "用户名或密码错误"}
	}

	user, err := m.repo.GetByUsername(ctx, username)
	if err != nil {
		m.loginLimiter.RecordFailure(clientIP)
		return nil, &model.AppError{Code: 40100, Message: "用户名或密码错误"}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		m.loginLimiter.RecordFailure(clientIP)
		m.logger.Warn("login failed: wrong password", zap.String("ip", clientIP))
		return nil, &model.AppError{Code: 40100, Message: "用户名或密码错误"}
	}

	m.loginLimiter.RecordSuccess(clientIP)

	pair, err := m.IssueTokenPair()
	if err != nil {
		return nil, err
	}

	m.logger.Info("login success", zap.String("ip", clientIP))
	return pair, nil
}

func (m *AuthManager) IssueTokenPair() (*TokenPair, error) {
	now := time.Now()
	accessJTI := uuid.New().String()
	refreshJTI := uuid.New().String()

	accessClaims := AccessTokenClaims{
		Sub:  "admin",
		Type: "access",
		Iat:  now.Unix(),
		Exp:  now.Add(accessTokenDuration).Unix(),
		Jti:  accessJTI,
	}
	accessToken, err := m.signToken(accessClaims.MapClaims())
	if err != nil {
		return nil, authError(ErrAuthJWT, "sign access token", err)
	}

	refreshClaims := jwt.MapClaims{
		"sub":  "admin",
		"type": "refresh",
		"iat":  now.Unix(),
		"exp":  now.Add(refreshTokenDuration).Unix(),
		"jti":  refreshJTI,
	}
	refreshToken, err := m.signToken(refreshClaims)
	if err != nil {
		return nil, authError(ErrAuthJWT, "sign refresh token", err)
	}

	m.mu.Lock()
	m.refreshTokens[refreshJTI] = now.Add(refreshTokenDuration)
	m.evictOldestIfNeeded()
	m.mu.Unlock()

	m.triggerPersist()

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTokenDuration.Seconds()),
	}, nil
}

func (m *AuthManager) RefreshTokens(oldRefreshToken string) (*TokenPair, error) {
	claims, err := m.parseToken(oldRefreshToken)
	if err != nil {
		return nil, &model.AppError{Code: 40101, Message: "Refresh Token 无效或已过期"}
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		return nil, &model.AppError{Code: 40101, Message: "Refresh Token 无效或已过期"}
	}

	jti, _ := claims["jti"].(string)

	m.mu.Lock()
	if _, exists := m.refreshTokens[jti]; !exists {
		m.mu.Unlock()
		return nil, &model.AppError{Code: 40101, Message: "Refresh Token 已被吊销"}
	}
	delete(m.refreshTokens, jti)
	m.mu.Unlock()

	m.triggerPersist()

	pair, err := m.IssueTokenPair()
	if err != nil {
		return nil, err
	}
	return pair, nil
}

func (m *AuthManager) ValidateAccessToken(tokenStr string) (*AccessTokenClaims, error) {
	claims, err := m.parseToken(tokenStr)
	if err != nil {
		return nil, &model.AppError{Code: 40101, Message: "Token 无效或已过期"}
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "access" {
		return nil, &model.AppError{Code: 40101, Message: "Token 无效或已过期"}
	}

	sub, _ := claims["sub"].(string)
	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)
	jti, _ := claims["jti"].(string)

	return &AccessTokenClaims{
		Sub:  sub,
		Type: tokenType,
		Iat:  int64(iat),
		Exp:  int64(exp),
		Jti:  jti,
	}, nil
}

func (m *AuthManager) VerifyPassword(ctx context.Context, password string) error {
	user, err := m.repo.GetByUsername(ctx, "admin")
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return &model.AppError{Code: 40102, Message: "旧密码错误"}
	}
	return nil
}

func (m *AuthManager) SetPassword(ctx context.Context, password string) error {
	if err := ValidatePasswordStrength(password); err != nil {
		return &model.AppError{Code: 40001, Message: err.Error()}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return authError(ErrAuthPassword, "bcrypt hash", err)
	}
	user, err := m.repo.GetByUsername(ctx, "admin")
	if err != nil {
		return err
	}
	return m.repo.UpdatePassword(ctx, user.ID, string(hash))
}

func (m *AuthManager) RevokeAllRefreshTokens() {
	m.mu.Lock()
	m.refreshTokens = make(map[string]time.Time)
	m.mu.Unlock()

	m.triggerPersist()
}

func (m *AuthManager) IsInitialized(ctx context.Context) bool {
	_, err := m.repo.GetByUsername(ctx, "admin")
	return err == nil
}

func (m *AuthManager) ResetPassword(ctx context.Context, password string) error {
	if err := ValidatePasswordStrength(password); err != nil {
		return &model.AppError{Code: 40001, Message: err.Error()}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return authError(ErrAuthPassword, "bcrypt hash", err)
	}
	user, err := m.repo.GetByUsername(ctx, "admin")
	if err != nil {
		return err
	}
	return m.repo.UpdatePassword(ctx, user.ID, string(hash))
}

func (m *AuthManager) GetUserInfo(ctx context.Context) (*model.User, error) {
	return m.repo.GetByUsername(ctx, "admin")
}

func (m *AuthManager) UpdateProfile(ctx context.Context, displayName string) error {
	user, err := m.repo.GetByUsername(ctx, "admin")
	if err != nil {
		return err
	}
	user.DisplayName = displayName
	return m.repo.Update(ctx, user)
}

func (m *AuthManager) SetupAdmin(ctx context.Context, username, password string) error {
	var setupErr error
	m.setupOnce.Do(func() {
		if m.IsInitialized(ctx) {
			setupErr = &model.AppError{Code: 40900, Message: "管理员账号已存在"}
			return
		}
		if username == "" || password == "" {
			setupErr = &model.AppError{Code: 40001, Message: "用户名和密码不能为空"}
			return
		}
		if err := ValidatePasswordStrength(password); err != nil {
			setupErr = &model.AppError{Code: 40001, Message: err.Error()}
			return
		}
		hash, err := HashPassword(password)
		if err != nil {
			setupErr = authError(ErrAuthPassword, "hash password", err)
			return
		}
		user := &model.User{
			Username:     username,
			DisplayName:  username,
			PasswordHash: hash,
		}
		setupErr = m.repo.Create(ctx, user)
	})
	if setupErr == nil && m.IsInitialized(ctx) {
		setupErr = &model.AppError{Code: 40900, Message: "管理员账号已存在"}
	}
	return setupErr
}

func (m *AuthManager) signToken(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.signingKey)
}

func (m *AuthManager) parseToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, authError(ErrAuthToken, fmt.Sprintf("unexpected signing method: %v", t.Header["alg"]), nil)
		}
		return m.signingKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, authError(ErrAuthToken, "invalid token claims", nil)
	}
	return claims, nil
}

func (m *AuthManager) evictOldestIfNeeded() {
	if len(m.refreshTokens) <= maxRefreshTokens {
		return
	}
	var oldestJTI string
	var oldestTime time.Time
	for jti, exp := range m.refreshTokens {
		if oldestJTI == "" || exp.Before(oldestTime) {
			oldestJTI = jti
			oldestTime = exp
		}
	}
	delete(m.refreshTokens, oldestJTI)
}

func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return authError(ErrAuthPassword, "密码长度至少 8 位", nil)
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return authError(ErrAuthPassword, "密码必须包含大写字母、小写字母、数字和特殊符号", nil)
	}
	return nil
}

func GenerateRandomPassword(length int) (string, error) {
	charsLen := len(randomPasswordChars)
	b := make([]byte, length)
	for i := range b {
		for {
			var rb [1]byte
			if _, err := rand.Read(rb[:]); err != nil {
				return "", authError(ErrAuthPassword, "random password generation failed", err)
			}
			v := int(rb[0])
			remainder := 256 % charsLen
			if v < (256 - remainder) {
				b[i] = randomPasswordChars[v%charsLen]
				break
			}
		}
	}
	return string(b), nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", authError(ErrAuthPassword, "bcrypt hash", err)
	}
	return string(hash), nil
}
