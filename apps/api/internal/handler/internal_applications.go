package handler

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/file"
	"github.com/gin-gonic/gin"
)

// InternalApplicationHandler 内部端候选人（投递）接口。
type InternalApplicationHandler struct {
	Jobs InternalJobService
}

// NewInternalApplicationHandler 构造处理器。
func NewInternalApplicationHandler(jobs InternalJobService) *InternalApplicationHandler {
	return &InternalApplicationHandler{Jobs: jobs}
}

// Get GET /api/v1/internal/applications/:id —— 投递详情。
func (h *InternalApplicationHandler) Get(c *gin.Context) {
	app, err := h.Jobs.GetApplication(c.Request.Context(), actorFromContext(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, app)
}

// SetStage PATCH /api/v1/internal/applications/:id/stage —— 流转阶段（OA 状态机）。
func (h *InternalApplicationHandler) SetStage(c *gin.Context) {
	var req domain.SetStageRequest
	if !bindJSON(c, &req) {
		return
	}
	app, err := h.Jobs.SetStage(c.Request.Context(), actorFromContext(c), c.Param("id"), req.Stage, req.Reason)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, app)
}

// Offer POST /api/v1/internal/applications/:id/offer —— 发起 Offer 审批（HR/管理员）。
func (h *InternalApplicationHandler) Offer(c *gin.Context) {
	var req domain.OfferRequest
	if !bindJSON(c, &req) {
		return
	}
	offer, err := h.Jobs.OfferRequest(c.Request.Context(), actorFromContext(c), c.Param("id"), req.Salary, req.JoinDate, req.Note)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, offer)
}

// OfferApprove POST /api/v1/internal/applications/:id/offer/approve —— 审批通过。
func (h *InternalApplicationHandler) OfferApprove(c *gin.Context) {
	app, err := h.Jobs.OfferApprove(c.Request.Context(), actorFromContext(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, app)
}

// OfferReject POST /api/v1/internal/applications/:id/offer/reject —— 审批驳回（必填原因）。
func (h *InternalApplicationHandler) OfferReject(c *gin.Context) {
	var req domain.OfferDecisionRequest
	if !bindJSON(c, &req) {
		return
	}
	app, err := h.Jobs.OfferReject(c.Request.Context(), actorFromContext(c), c.Param("id"), req.Reason)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, app)
}

// Batch POST /api/v1/internal/applications/batch —— 批量操作。
func (h *InternalApplicationHandler) Batch(c *gin.Context) {
	var req domain.BatchActionRequest
	if !bindJSON(c, &req) {
		return
	}
	updated, err := h.Jobs.Batch(c.Request.Context(), actorFromContext(c), req.IDs, req.Action, req.Stage, req.Reason)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, gin.H{"updated": updated})
}

// ResumeURL GET /api/v1/internal/applications/:id/resume-url —— 简历预签名下载地址。
func (h *InternalApplicationHandler) ResumeURL(c *gin.Context) {
	url, err := h.Jobs.ResumeURL(c.Request.Context(), actorFromContext(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, gin.H{"url": url})
}

// Resume GET /api/v1/internal/applications/:id/resume —— 鉴权流式下载简历原件。
// 预签名 URL 会暴露 MinIO 内网主机名，浏览器不可达；此接口由 API 转发文件内容。
func (h *InternalApplicationHandler) Resume(c *gin.Context) {
	data, filename, err := h.Jobs.ResumeFile(c.Request.Context(), actorFromContext(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Content-Type", file.ContentTypeFor(filename))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(filename)))
	c.Header("Cache-Control", "private, max-age=60")
	c.Data(http.StatusOK, "application/octet-stream", data)
}
