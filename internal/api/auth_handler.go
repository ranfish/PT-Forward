package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ranfish/pt-forward/internal/auth"
	"github.com/ranfish/pt-forward/internal/middleware"
	"github.com/ranfish/pt-forward/internal/model"
)

type AuthHandler struct {
	authManager *auth.AuthManager
}

func NewAuthHandler(authManager *auth.AuthManager) *AuthHandler {
	return &AuthHandler{authManager: authManager}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		Error(w, http.StatusBadRequest, 40001, "用户名和密码不能为空")
		return
	}

	clientIP := extractIP(r)
	pair, err := h.authManager.Login(r.Context(), req.Username, req.Password, clientIP)
	if err != nil {
		writeAppError(w, err)
		return
	}

	Success(w, pair)
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *AuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.RefreshToken == "" {
		Error(w, http.StatusBadRequest, 40001, "refreshToken 不能为空")
		return
	}

	pair, err := h.authManager.RefreshTokens(req.RefreshToken)
	if err != nil {
		writeAppError(w, err)
		return
	}

	Success(w, pair)
}

func (h *AuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	initialized := h.authManager.IsInitialized(r.Context())
	resp := struct {
		Initialized bool   `json:"initialized"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	}{
		Initialized: initialized,
	}

	if initialized {
		user, err := h.authManager.GetUserInfo(r.Context())
		if err == nil {
			resp.Username = user.Username
			resp.DisplayName = user.DisplayName
		}
	}

	Success(w, resp)
}

func (h *AuthHandler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}
	Error(w, http.StatusConflict, 40900, "密码已设置")
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (h *AuthHandler) HandlePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		Error(w, http.StatusBadRequest, 40001, "旧密码和新密码不能为空")
		return
	}

	if err := h.authManager.VerifyPassword(r.Context(), req.OldPassword); err != nil {
		writeAppError(w, err)
		return
	}

	if err := h.authManager.SetPassword(r.Context(), req.NewPassword); err != nil {
		writeAppError(w, err)
		return
	}

	h.authManager.RevokeAllRefreshTokens()
	Success(w, nil)
}

type updateProfileRequest struct {
	DisplayName string `json:"displayName"`
}

func (h *AuthHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.DisplayName == "" {
		Error(w, http.StatusBadRequest, 40001, "显示名不能为空")
		return
	}

	if err := h.authManager.UpdateProfile(r.Context(), req.DisplayName); err != nil {
		writeAppError(w, err)
		return
	}

	Success(w, nil)
}

func extractIP(r *http.Request) string {
	return middleware.ExtractIP(r)
}

func writeAppError(w http.ResponseWriter, err error) {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		httpStatus := mapCodeToHTTP(appErr.Code)
		Error(w, httpStatus, appErr.Code, appErr.Message)
		return
	}
	Error(w, http.StatusInternalServerError, 50000, "内部错误")
}

func mapCodeToHTTP(code int) int {
	switch {
	case code >= 40001 && code <= 40099:
		return http.StatusBadRequest
	case code >= 40100 && code <= 40199:
		return http.StatusUnauthorized
	case code == 40900:
		return http.StatusConflict
	case code == 42901:
		return http.StatusTooManyRequests
	case code >= 40400 && code <= 40499:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
