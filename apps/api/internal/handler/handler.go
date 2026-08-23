// Package handler HTTP 处理层（Gin）。
//
// 处理器只做参数解析与响应序列化；业务逻辑与权限判断在 service 层。
// 错误响应统一为 {"error":{"code","message"}}。
package handler

import (
	"errors"
	"net/http"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/middleware"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/service"
	"github.com/gin-gonic/gin"
)

// respondJSON 输出 JSON。
func respondJSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

// respondError 输出统一错误体 {"error":{code,message}}。
func respondError(c *gin.Context, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		respondJSON(c, appErr.Status, gin.H{"error": gin.H{"code": appErr.Code, "message": appErr.Message}})
		return
	}
	if errors.Is(err, repo.ErrNotFound) {
		respondJSON(c, http.StatusNotFound, gin.H{"error": gin.H{"code": domain.CodeNotFound, "message": "资源不存在"}})
		return
	}
	respondJSON(c, http.StatusInternalServerError, gin.H{"error": gin.H{"code": domain.CodeInternal, "message": "服务器内部错误"}})
}

// actorFromContext 从请求上下文取出当前用户 Actor。
func actorFromContext(c *gin.Context) *service.Actor {
	claims, ok := middleware.ClaimsFromContext(c.Request.Context())
	if !ok {
		return nil
	}
	return service.NewActorFromClaims(claims)
}

// bindJSON 解析 JSON 请求体，失败返回 400。
func bindJSON(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		respondError(c, domain.NewError(http.StatusBadRequest, domain.CodeBadRequest, "请求体格式不正确"))
		return false
	}
	return true
}
