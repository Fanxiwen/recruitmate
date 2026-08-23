package handler

import (
	"io"
	"net/http"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/file"
	"github.com/gin-gonic/gin"
)

// PublicJobHandler 公开岗位接口（外部求职端，无需认证）。
type PublicJobHandler struct {
	Jobs     PublicJobLister
	ApplySvc ApplyService
}

// NewPublicJobHandler 构造处理器。
func NewPublicJobHandler(jobs PublicJobLister, apply ApplyService) *PublicJobHandler {
	return &PublicJobHandler{Jobs: jobs, ApplySvc: apply}
}

// List GET /api/v1/public/jobs —— 岗位列表（仅 open）。
func (h *PublicJobHandler) List(c *gin.Context) {
	var q domain.JobListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		respondError(c, domain.NewError(http.StatusBadRequest, domain.CodeBadRequest, "查询参数不正确"))
		return
	}
	result, err := h.Jobs.ListPublic(c.Request.Context(), q)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, result)
}

// Get GET /api/v1/public/jobs/:id —— 岗位详情（仅 open）。
func (h *PublicJobHandler) Get(c *gin.Context) {
	job, err := h.Jobs.GetPublic(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, job)
}

// Apply POST /api/v1/public/jobs/:id/applications —— 投递简历（multipart）。
// 表单字段：name/email/phone/source/resumeText + 可选文件 resume（pdf/docx/txt ≤5MB）。
func (h *PublicJobHandler) Apply(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		respondError(c, domain.NewError(http.StatusBadRequest, domain.CodeBadRequest, "缺少岗位 id"))
		return
	}

	// multipart 内存限制：5MB 文件 + 表单
	if err := c.Request.ParseMultipartForm(file.MaxResumeSize + 1<<20); err != nil {
		respondError(c, domain.NewError(http.StatusBadRequest, domain.CodeBadRequest, "简历文件不能超过 5MB"))
		return
	}

	in := &domain.ApplyInput{
		Name:       c.PostForm("name"),
		Email:      c.PostForm("email"),
		Phone:      c.PostForm("phone"),
		Source:     c.PostForm("source"),
		ResumeText: c.PostForm("resumeText"),
	}

	if fh, err := c.FormFile("resume"); err == nil {
		if fh.Size > file.MaxResumeSize {
			respondError(c, domain.NewError(http.StatusBadRequest, domain.CodeBadRequest, "简历文件不能超过 5MB"))
			return
		}
		f, err := fh.Open()
		if err != nil {
			respondError(c, domain.NewError(http.StatusBadRequest, domain.CodeBadRequest, "读取简历文件失败"))
			return
		}
		content, err := io.ReadAll(io.LimitReader(f, file.MaxResumeSize+1))
		f.Close()
		if err != nil {
			respondError(c, domain.NewError(http.StatusBadRequest, domain.CodeBadRequest, "读取简历文件失败"))
			return
		}
		in.ResumeFile = &domain.UploadedFile{FileName: fh.Filename, Content: content}
	}

	result, err := h.ApplySvc.Apply(c.Request.Context(), jobID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusCreated, result)
}
