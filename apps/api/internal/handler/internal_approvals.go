package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ApprovalsHandler 审批中心（岗位发布审批 + Offer 审批统一收口）。
type ApprovalsHandler struct {
	Jobs InternalJobService
}

// NewApprovalsHandler 构造处理器。
func NewApprovalsHandler(jobs InternalJobService) *ApprovalsHandler {
	return &ApprovalsHandler{Jobs: jobs}
}

// ListOffers GET /api/v1/internal/approvals/offers —— Offer 待审批列表（含候选人信息与审批单）。
func (h *ApprovalsHandler) ListOffers(c *gin.Context) {
	actor := actorFromContext(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	result, err := h.Jobs.ListOfferApprovals(c.Request.Context(), actor, page, pageSize)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, result)
}
