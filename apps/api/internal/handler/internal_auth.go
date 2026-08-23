package handler

import (
	"net/http"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InternalAuthHandler 内部端认证接口。
type InternalAuthHandler struct {
	Auth        InternalAuthService
	Departments DepartmentLister
}

// NewInternalAuthHandler 构造处理器。
func NewInternalAuthHandler(auth InternalAuthService, depts DepartmentLister) *InternalAuthHandler {
	return &InternalAuthHandler{Auth: auth, Departments: depts}
}

// Login POST /api/v1/internal/auth/login —— 内部端登录。
func (h *InternalAuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := h.Auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, resp)
}

// Me GET /api/v1/internal/me —— 当前用户信息。
func (h *InternalAuthHandler) Me(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c.Request.Context())
	if !ok {
		respondError(c, domain.NewError(http.StatusUnauthorized, domain.CodeUnauthorized, "未登录"))
		return
	}
	user, err := h.Auth.Me(c.Request.Context(), claims.Subject)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, user)
}

// ListDepartments GET /api/v1/internal/departments —— 部门列表。
func (h *InternalAuthHandler) ListDepartments(c *gin.Context) {
	depts, err := h.Departments.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, depts)
}
