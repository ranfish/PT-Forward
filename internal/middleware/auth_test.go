package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/auth"
)

// §59.542 追记八锚: ops token 认证通道四态——
// ①匹配 → 200 放行（admin 身份）
// ②不匹配 → 401
// ③未配置（空）→ ops token 不生效（401）
// ④正常 JWT 不受影响（ops 通道是纯增量）
// 复用 middleware_test.go 的 newTestAuthManager（sqlite 内存库完整构造）。
func TestJWTAuthOpsToken(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(auth.CtxKeyUserID).(string)
		_, _ = w.Write([]byte("uid=" + uid))
	}

	t.Run("ops token 匹配放行", func(t *testing.T) {
		m := newTestAuthManager(t)
		m.SetOpsToken("tok-correct-32bytes-aaaaaaaaaaaaaaaa")
		h := JWTAuth(m)(http.HandlerFunc(handler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
		req.Header.Set("Authorization", "Bearer tok-correct-32bytes-aaaaaaaaaaaaaaaa")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("code=%d（ops token 匹配应 200）body=%s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "uid=admin" {
			t.Errorf("uid=%q（应为 admin）", rec.Body.String())
		}
	})

	t.Run("ops token 不匹配拒绝", func(t *testing.T) {
		m := newTestAuthManager(t)
		m.SetOpsToken("tok-correct-32bytes-aaaaaaaaaaaaaaaa")
		h := JWTAuth(m)(http.HandlerFunc(handler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
		req.Header.Set("Authorization", "Bearer tok-WRONG")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d（不匹配应 401）", rec.Code)
		}
	})

	t.Run("未配置时 ops token 不生效", func(t *testing.T) {
		m := newTestAuthManager(t)
		// 不调用 SetOpsToken（=未配置 PTF_OPS_TOKEN）
		h := JWTAuth(m)(http.HandlerFunc(handler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
		req.Header.Set("Authorization", "Bearer tok-correct-32bytes-aaaaaaaaaaaaaaaa")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d（未配置应 401）", rec.Code)
		}
	})

	t.Run("正常 JWT 通道不受影响", func(t *testing.T) {
		m := newTestAuthManager(t)
		m.SetOpsToken("tok-correct-32bytes-aaaaaaaaaaaaaaaa")
		// 签发一个真 JWT（signingKey 已注入）
		pair, err := m.IssueTokenPair()
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		h := JWTAuth(m)(http.HandlerFunc(handler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("code=%d（JWT 应正常 200）body=%s", rec.Code, rec.Body.String())
		}
	})
}
