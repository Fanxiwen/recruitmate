package handler

import (
	"context"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/service"
)

// 处理器依赖的服务接口（由 service 包实现，便于 httptest 用 stub 替身）。

// PublicJobLister 公开岗位查询服务。
type PublicJobLister interface {
	ListPublic(ctx context.Context, q domain.JobListQuery) (*domain.Paginated[domain.JobPosting], error)
	GetPublic(ctx context.Context, id string) (*domain.JobPosting, error)
}

// ApplyService 投递服务。
type ApplyService interface {
	Apply(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error)
}

// PublicAuthService 公开认证服务（邮箱验证码 + 候选人投递查询）。
type PublicAuthService interface {
	SendEmailCode(ctx context.Context, email string) (string, error)
	VerifyEmailCode(ctx context.Context, email, code string) (string, error)
	MyApplications(ctx context.Context, candidateID string) ([]domain.ApplicationPublic, error)
}

// InternalAuthService 内部认证服务。
type InternalAuthService interface {
	Login(ctx context.Context, email, password string) (*domain.LoginResponse, error)
	Me(ctx context.Context, userID string) (*domain.User, error)
}

// DepartmentLister 部门查询。
type DepartmentLister interface {
	List(ctx context.Context) ([]domain.Department, error)
}

// InternalJobService 内部岗位服务。
type InternalJobService interface {
	Create(ctx context.Context, actor *service.Actor, input *domain.JobPostingInput) (*domain.JobPosting, error)
	Update(ctx context.Context, actor *service.Actor, id string, input *domain.JobPostingInput) (*domain.JobPosting, error)
	Get(ctx context.Context, actor *service.Actor, id string) (*domain.JobPosting, error)
	List(ctx context.Context, actor *service.Actor, status string, page, pageSize int) (*domain.Paginated[domain.JobPosting], error)
	Submit(ctx context.Context, actor *service.Actor, id string) (*domain.JobPosting, error)
	Approve(ctx context.Context, actor *service.Actor, id string) (*domain.JobPosting, error)
	Reject(ctx context.Context, actor *service.Actor, id string) (*domain.JobPosting, error)
	Close(ctx context.Context, actor *service.Actor, id string) (*domain.JobPosting, error)
	Reopen(ctx context.Context, actor *service.Actor, id string) (*domain.JobPosting, error)
	ListApplications(ctx context.Context, actor *service.Actor, jobID string, f repo.ApplicationListFilter, page, pageSize int) (*domain.Paginated[domain.ApplicationInternal], error)
	GetApplication(ctx context.Context, actor *service.Actor, id string) (*domain.ApplicationInternal, error)
	SetStage(ctx context.Context, actor *service.Actor, id, stage, reason string) (*domain.ApplicationInternal, error)
	Batch(ctx context.Context, actor *service.Actor, ids []string, action, stage, reason string) (int, error)
	Stats(ctx context.Context, actor *service.Actor, jobID string) (*domain.JobStats, error)
	ResumeURL(ctx context.Context, actor *service.Actor, appID string) (string, error)
	ResumeFile(ctx context.Context, actor *service.Actor, appID string) ([]byte, string, error)
	// OA Offer 审批链
	OfferRequest(ctx context.Context, actor *service.Actor, appID, salary, joinDate, note string) (*domain.Offer, error)
	OfferApprove(ctx context.Context, actor *service.Actor, appID string) (*domain.ApplicationInternal, error)
	OfferReject(ctx context.Context, actor *service.Actor, appID, reason string) (*domain.ApplicationInternal, error)
}
