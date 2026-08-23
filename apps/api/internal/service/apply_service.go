package service

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/file"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/google/uuid"
)

// ApplyService 外部投递（公开接口）。
type ApplyService struct {
	Jobs       repo.JobRepo
	Candidates repo.CandidateRepo
	Apps       repo.ApplicationRepo
	Storage    file.Storage
	Queue      TaskQueue
	Audit      repo.AuditRepo
}

// NewApplyService 构造投递服务。
func NewApplyService(jobs repo.JobRepo, candidates repo.CandidateRepo, apps repo.ApplicationRepo, storage file.Storage, queue TaskQueue, audit repo.AuditRepo) *ApplyService {
	return &ApplyService{Jobs: jobs, Candidates: candidates, Apps: apps, Storage: storage, Queue: queue, Audit: audit}
}

// Apply 处理投递：校验 → 查/建候选人 → 去重 → 建投递 → 上传简历 → 入队 AI 流水线。
func (s *ApplyService) Apply(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error) {
	// 1. 岗位必须存在且 open
	job, err := s.Jobs.Get(ctx, jobID)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位失败", err)
	}
	if job.Status != string(domain.JobStatusOpen) {
		return nil, domain.NewError(409, domain.CodeConflict, "岗位未在招聘中")
	}

	// 2. 表单校验
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	if in.Name == "" {
		return nil, domain.NewError(400, domain.CodeBadRequest, "请填写姓名")
	}
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		return nil, domain.NewError(400, domain.CodeBadRequest, "请填写正确的邮箱")
	}
	if in.ResumeText == "" && in.ResumeFile == nil {
		return nil, domain.NewError(400, domain.CodeBadRequest, "请提供简历文件或简历文本")
	}
	if in.ResumeFile != nil {
		if !file.IsAllowedExt(in.ResumeFile.FileName) {
			return nil, domain.NewError(400, domain.CodeBadRequest, "仅支持 PDF / DOCX / TXT 格式简历")
		}
		if len(in.ResumeFile.Content) > file.MaxResumeSize {
			return nil, domain.NewError(400, domain.CodeBadRequest, "简历文件不能超过 5MB")
		}
	}

	// 3. 查/建候选人
	candidate, err := s.Candidates.GetOrCreateByEmail(ctx, in.Email, in.Name, in.Phone)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "候选人处理失败", err)
	}

	// 4. 去重：同一候选人同一岗位仅可投递一次
	if _, err := s.Apps.GetByCandidateAndJob(ctx, candidate.ID, jobID); err == nil {
		return nil, domain.NewError(409, domain.CodeConflict, "您已投递过该岗位，请勿重复投递")
	} else if err != repo.ErrNotFound {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询投递失败", err)
	}

	// 5. 创建投递
	app := &domain.Application{
		ID:          uuid.NewString(),
		CandidateID: candidate.ID,
		JobID:       jobID,
		Stage:       string(domain.StageNew),
		Source:      strings.TrimSpace(in.Source),
		ResumeText:  in.ResumeText,
	}
	if err := s.Apps.Create(ctx, app); err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "创建投递失败", err)
	}

	// 6. 上传简历文件（如有）
	if in.ResumeFile != nil {
		ext := strings.ToLower(filepath.Ext(in.ResumeFile.FileName))
		key := "resumes/" + app.ID + ext
		if err := s.Storage.Upload(ctx, key, strings.NewReader(string(in.ResumeFile.Content)), int64(len(in.ResumeFile.Content)), file.ContentTypeFor(in.ResumeFile.FileName)); err != nil {
			slog.Error("resume upload failed", "key", key, "error", err)
			return nil, domain.WrapError(500, domain.CodeInternal, "简历文件上传失败", err)
		}
		if err := s.Apps.UpdateResumeFileKey(ctx, app.ID, key); err != nil {
			return nil, domain.WrapError(500, domain.CodeInternal, "保存简历文件信息失败", err)
		}
	}

	// 7. 入队 AI 匹配流水线（版本 = 岗位 updated_at unix，用于唯一键防重）
	if err := s.Queue.EnqueueResumeProcess(ctx, app.ID, job.UpdatedAt.Unix()); err != nil {
		slog.Error("enqueue resume:process failed", "application_id", app.ID, "error", err)
	}

	// 8. 审计（匿名操作，actor 为空）
	writeAudit(ctx, s.Audit, nil, "application.create", "application", app.ID, map[string]any{"job_id": jobID})

	return &domain.ApplyResult{ID: app.ID}, nil
}
