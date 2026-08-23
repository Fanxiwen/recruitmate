package handler

import (
	"net/http"
	"strconv"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/gin-gonic/gin"
)

// InternalJobHandler 内部端岗位接口（JWT + RBAC）。
type InternalJobHandler struct {
	Jobs InternalJobService
}

// NewInternalJobHandler 构造处理器。
func NewInternalJobHandler(jobs InternalJobService) *InternalJobHandler {
	return &InternalJobHandler{Jobs: jobs}
}

// Create POST /api/v1/internal/jobs —— 创建岗位（draft）。
func (h *InternalJobHandler) Create(c *gin.Context) {
	var input domain.JobPostingInput
	if !bindJSON(c, &input) {
		return
	}
	job, err := h.Jobs.Create(c.Request.Context(), actorFromContext(c), &input)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusCreated, job)
}

// List GET /api/v1/internal/jobs —— 岗位分页列表。
func (h *InternalJobHandler) List(c *gin.Context) {
	status := c.Query("status")
	page, pageSize := pageParams(c)
	actor := actorFromContext(c)
	result, err := h.Jobs.List(c.Request.Context(), actor, status, page, pageSize)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, result)
}

// Get GET /api/v1/internal/jobs/:id —— 岗位详情。
func (h *InternalJobHandler) Get(c *gin.Context) {
	job, err := h.Jobs.Get(c.Request.Context(), actorFromContext(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, job)
}

// Update PATCH /api/v1/internal/jobs/:id —— 更新岗位。
func (h *InternalJobHandler) Update(c *gin.Context) {
	var input domain.JobPostingInput
	if !bindJSON(c, &input) {
		return
	}
	job, err := h.Jobs.Update(c.Request.Context(), actorFromContext(c), c.Param("id"), &input)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, job)
}

// Submit POST /api/v1/internal/jobs/:id/submit —— 提交审批。
func (h *InternalJobHandler) Submit(c *gin.Context) {
	h.transition(c, "submit")
}

// Approve POST /api/v1/internal/jobs/:id/approve —— 审批发布。
func (h *InternalJobHandler) Approve(c *gin.Context) {
	h.transition(c, "approve")
}

// Reject POST /api/v1/internal/jobs/:id/reject —— 驳回。
func (h *InternalJobHandler) Reject(c *gin.Context) {
	h.transition(c, "reject")
}

// Close POST /api/v1/internal/jobs/:id/close —— 关闭岗位。
func (h *InternalJobHandler) Close(c *gin.Context) {
	h.transition(c, "close")
}

// Reopen POST /api/v1/internal/jobs/:id/reopen —— 重新开放。
func (h *InternalJobHandler) Reopen(c *gin.Context) {
	h.transition(c, "reopen")
}

func (h *InternalJobHandler) transition(c *gin.Context, action string) {
	var job *domain.JobPosting
	var err error
	ctx := c.Request.Context()
	actor := actorFromContext(c)
	id := c.Param("id")
	switch action {
	case "submit":
		job, err = h.Jobs.Submit(ctx, actor, id)
	case "approve":
		job, err = h.Jobs.Approve(ctx, actor, id)
	case "reject":
		job, err = h.Jobs.Reject(ctx, actor, id)
	case "close":
		job, err = h.Jobs.Close(ctx, actor, id)
	case "reopen":
		job, err = h.Jobs.Reopen(ctx, actor, id)
	}
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, job)
}

// ListApplications GET /api/v1/internal/jobs/:id/applications —— 岗位候选人列表。
func (h *InternalJobHandler) ListApplications(c *gin.Context) {
	f := repo.ApplicationListFilter{
		Stage:    c.Query("stage"),
		HardPass: c.Query("hardPass"),
		Q:        c.Query("q"),
		Sort:     c.Query("sort"),
	}
	page, pageSize := pageParams(c)
	result, err := h.Jobs.ListApplications(c.Request.Context(), actorFromContext(c), c.Param("id"), f, page, pageSize)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, result)
}

// Stats GET /api/v1/internal/jobs/:id/stats —— 投递漏斗统计。
func (h *InternalJobHandler) Stats(c *gin.Context) {
	stats, err := h.Jobs.Stats(c.Request.Context(), actorFromContext(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, stats)
}

// pageParams 解析 page/pageSize 查询参数（默认 1/20，上限 100）。
func pageParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.Query("pageSize"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			pageSize = n
		}
	}
	return page, pageSize
}
