package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/file"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
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

// OfferApprove POST /api/v1/internal/applications/:id/offer/approve —— 审批通过（salary 为最终薪资，必填）。
func (h *InternalApplicationHandler) OfferApprove(c *gin.Context) {
	var req domain.OfferDecisionRequest
	if !bindJSON(c, &req) {
		return
	}
	app, err := h.Jobs.OfferApprove(c.Request.Context(), actorFromContext(c), c.Param("id"), req.Salary)
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

// Pending GET /api/v1/internal/applications/pending —— 待处理（新简历）列表。
// 有人投递后 HR 在此集中初筛（按投递时间先进先出）。
func (h *InternalApplicationHandler) Pending(c *gin.Context) {
	actor := actorFromContext(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	result, err := h.Jobs.ListPendingApplications(c.Request.Context(), actor, page, pageSize)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, result)
}

// Candidates GET /api/v1/internal/candidates —— 候选人中心全局列表。
func (h *InternalApplicationHandler) Candidates(c *gin.Context) {
	actor := actorFromContext(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var deptID *string
	if v := c.Query("departmentId"); v != "" {
		deptID = &v
	}
	result, err := h.Jobs.ListCandidates(c.Request.Context(), actor, repo.CandidateListFilter{
		Stage:        c.Query("stage"),
		DepartmentID: deptID,
		JobID:        c.Query("jobId"),
		Q:            c.Query("q"),
		Sort:         c.DefaultQuery("sort", "score_desc"),
	}, page, pageSize)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, result)
}

// Todos GET /api/v1/internal/todos/interviews —— 我的待办（面试类，按角色）。
func (h *InternalApplicationHandler) Todos(c *gin.Context) {
	items, err := h.Jobs.ListInterviewTodos(c.Request.Context(), actorFromContext(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, gin.H{"items": items})
}

// ScheduleInterview POST /api/v1/internal/applications/:id/interviews —— 安排一轮面试（时间必填）。
func (h *InternalApplicationHandler) ScheduleInterview(c *gin.Context) {
	var req domain.ScheduleInterviewRequest
	if !bindJSON(c, &req) {
		return
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		respondError(c, domain.NewError(400, domain.CodeBadRequest, "面试时间格式不正确"))
		return
	}
	iv, err := h.Jobs.ScheduleInterview(c.Request.Context(), actorFromContext(c), c.Param("id"), req.Round, scheduledAt)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, iv)
}

// CompleteInterview POST /api/v1/internal/applications/:id/interviews/:round/complete —— 完成一轮面试（评价+结论）。
func (h *InternalApplicationHandler) CompleteInterview(c *gin.Context) {
	var req domain.CompleteInterviewRequest
	if !bindJSON(c, &req) {
		return
	}
	app, err := h.Jobs.CompleteInterview(c.Request.Context(), actorFromContext(c), c.Param("id"), c.Param("round"), req.Result, req.Feedback)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, app)
}
