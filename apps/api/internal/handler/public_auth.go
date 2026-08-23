package handler

import (
	"net/http"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// PublicAuthHandler 公开认证接口（邮箱验证码 + 投递状态查询）。
type PublicAuthHandler struct {
	Auth PublicAuthService
}

// NewPublicAuthHandler 构造处理器。
func NewPublicAuthHandler(auth PublicAuthService) *PublicAuthHandler {
	return &PublicAuthHandler{Auth: auth}
}

// SendEmailCode POST /api/v1/public/auth/email-code —— 发送 6 位验证码。
func (h *PublicAuthHandler) SendEmailCode(c *gin.Context) {
	var req domain.SendCodeRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Auth.SendEmailCode(c.Request.Context(), req.Email); err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, gin.H{"message": "验证码已发送"})
}

// Verify POST /api/v1/public/auth/verify —— 校验验证码，返回候选人 JWT。
func (h *PublicAuthHandler) Verify(c *gin.Context) {
	var req domain.VerifyCodeRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := h.Auth.VerifyEmailCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, domain.VerifyCodeResponse{Token: token})
}

// MyApplications GET /api/v1/public/my/applications —— 候选人本人投递列表（Bearer 候选人 token）。
func (h *PublicAuthHandler) MyApplications(c *gin.Context) {
	candidateID, ok := middleware.CandidateIDFromContext(c.Request.Context())
	if !ok {
		respondError(c, domain.NewError(http.StatusUnauthorized, domain.CodeUnauthorized, "未登录"))
		return
	}
	apps, err := h.Auth.MyApplications(c.Request.Context(), candidateID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, apps)
}
