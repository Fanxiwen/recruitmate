package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

const testSecret = "test-secret-for-unit-tests"

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestEngine 构建带中间件的测试引擎。
func newTestEngine(handlers ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.GET("/ping", handlers...)
	return r
}

// doRequest 发送请求并返回响应。
func doRequest(r http.Handler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequireInternal(t *testing.T) {
	r := newTestEngine(RequireInternal(testSecret), func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "claims missing"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"sub": claims.Subject, "role": claims.Role})
	})

	t.Run("有效令牌", func(t *testing.T) {
		token, err := auth.Sign(testSecret, time.Hour, "user-1", "hr", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
		}
	})

	t.Run("缺少令牌", func(t *testing.T) {
		w := doRequest(r, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("非法令牌", func(t *testing.T) {
		w := doRequest(r, "not-a-jwt")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("过期令牌", func(t *testing.T) {
		token, err := auth.Sign(testSecret, -time.Hour, "user-1", "hr", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("错误密钥签名", func(t *testing.T) {
		token, err := auth.Sign("other-secret", time.Hour, "user-1", "hr", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("候选人令牌不可用于内部接口", func(t *testing.T) {
		token, err := auth.SignCandidate(testSecret, time.Hour, "candidate-1")
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})
}

func TestRequireCandidate(t *testing.T) {
	r := newTestEngine(RequireCandidate(testSecret), func(c *gin.Context) {
		id, ok := CandidateIDFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "candidate id missing"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"candidateId": id})
	})

	t.Run("有效候选人令牌", func(t *testing.T) {
		token, err := auth.SignCandidate(testSecret, time.Hour, "candidate-1")
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("内部令牌不可用于候选人接口", func(t *testing.T) {
		token, err := auth.Sign(testSecret, time.Hour, "user-1", "hr", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})
}

func TestRequireRoles(t *testing.T) {
	r := newTestEngine(
		RequireInternal(testSecret),
		RequireRoles("admin", "hr"),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	t.Run("角色允许", func(t *testing.T) {
		token, err := auth.Sign(testSecret, time.Hour, "user-1", "hr", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("角色不允许返回 403", func(t *testing.T) {
		token, err := auth.Sign(testSecret, time.Hour, "user-1", "hiring_manager", strPtr("dept-1"))
		if err != nil {
			t.Fatal(err)
		}
		w := doRequest(r, token)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func strPtr(s string) *string { return &s }
