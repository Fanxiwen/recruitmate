// Package middleware 提供 HTTP 中间件：CORS、内部端 JWT 鉴权、求职者 JWT 鉴权。
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/auth"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/gin-gonic/gin"
)

// ContextKey 上下文键类型。
type ContextKey string

const (
	// CtxClaims 内部端 Claims 上下文键。
	CtxClaims ContextKey = "claims"
	// CtxCandidateID 求职者候选人 id 上下文键。
	CtxCandidateID ContextKey = "candidate_id"
)

// ClaimsFromContext 从上下文取出内部端 Claims。
func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	v, ok := ctx.Value(CtxClaims).(*auth.Claims)
	return v, ok
}

// CandidateIDFromContext 从上下文取出候选人 id。
func CandidateIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(CtxCandidateID).(string)
	return v, ok
}

// CORS 跨域中间件：允许配置的源（默认本地两个前端端口）。
func CORS(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// writeAuthError 输出统一鉴权错误。
func writeAuthError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// bearerToken 从 Authorization 头提取 Bearer token。
func bearerToken(c *gin.Context) (string, bool) {
	h := c.GetHeader("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")), true
}

// RequireInternal 内部端鉴权：校验 JWT（aud=internal），注入 Claims。
func RequireInternal(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			writeAuthError(c, http.StatusUnauthorized, domain.CodeUnauthorized, "未提供访问令牌")
			return
		}
		claims, err := auth.Parse(secret, token, auth.AudienceInternal)
		if err != nil {
			writeAuthError(c, http.StatusUnauthorized, domain.CodeUnauthorized, "令牌无效或已过期")
			return
		}
		if claims.Subject == "" {
			writeAuthError(c, http.StatusUnauthorized, domain.CodeUnauthorized, "令牌缺少用户标识")
			return
		}
		c.Set(string(CtxClaims), claims)
		c.Next()
	}
}

// RequireCandidate 求职者鉴权：校验 JWT（aud=candidate），注入 candidate_id。
func RequireCandidate(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			writeAuthError(c, http.StatusUnauthorized, domain.CodeUnauthorized, "未提供访问令牌")
			return
		}
		claims, err := auth.Parse(secret, token, auth.AudienceCandidate)
		if err != nil {
			writeAuthError(c, http.StatusUnauthorized, domain.CodeUnauthorized, "令牌无效或已过期")
			return
		}
		if claims.Subject == "" {
			writeAuthError(c, http.StatusUnauthorized, domain.CodeUnauthorized, "令牌缺少候选人标识")
			return
		}
		c.Set(string(CtxCandidateID), claims.Subject)
		c.Next()
	}
}

// RequireRoles 角色白名单检查（需在 RequireInternal 之后使用）。
func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c.Request.Context())
		if !ok || !allowed[claims.Role] {
			writeAuthError(c, http.StatusForbidden, domain.CodeForbidden, "权限不足")
			return
		}
		c.Next()
	}
}
