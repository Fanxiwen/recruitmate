// Package repo 数据访问层：手写 SQL + pgx（不使用 ORM）。
//
// 所有 SQL 集中在本包；上层（service/worker）只依赖本包定义的接口，便于测试替身。
package repo

import (
	"context"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
)

// ============ 接口定义 ============

// UserRepo 内部用户。
type UserRepo interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

// DepartmentRepo 部门。
type DepartmentRepo interface {
	List(ctx context.Context) ([]domain.Department, error)
}

// JobListFilter 岗位列表筛选（公共端）。
type JobListFilter struct {
	Q            string
	DepartmentID *string
	JobType      string
}

// JobRepo 岗位。
type JobRepo interface {
	Create(ctx context.Context, j *domain.JobPosting) error
	// Update 更新岗位内容（title/description/requirements 等），并刷新 updated_at。
	Update(ctx context.Context, j *domain.JobPosting) error
	// Get 按 id 查询岗位（含部门名/负责人名）。
	Get(ctx context.Context, id string) (*domain.JobPosting, error)
	// ListPublic 公开岗位列表（仅 open，按 published_at desc）。
	ListPublic(ctx context.Context, f JobListFilter, page, pageSize int) ([]domain.JobPosting, int, error)
	// ListInternal 内部岗位列表（可按 status 过滤，hiring_manager 限定部门）。
	ListInternal(ctx context.Context, status string, deptID *string, page, pageSize int) ([]domain.JobPosting, int, error)
	// SetStatus 岗位状态流转（approve/reject/close/reopen 等），维护 approver_id/published_at/closed_at。
	SetStatus(ctx context.Context, id, status string, approverID *string) error
	// SetEmbedding 写入岗位 embedding（向量缓存）。
	SetEmbedding(ctx context.Context, id string, vec []float32) error
	// GetEmbedding 读取岗位 embedding；未生成时返回 nil。
	GetEmbedding(ctx context.Context, id string) ([]float32, error)
}

// CandidateRepo 外部求职者。
type CandidateRepo interface {
	// GetOrCreateByEmail 按邮箱（小写）查或建候选人，返回记录。
	GetOrCreateByEmail(ctx context.Context, email, name, phone string) (*domain.Candidate, error)
}

// ApplicationListFilter 候选人列表筛选。
type ApplicationListFilter struct {
	Stage    string
	HardPass string // only / exclude / 空
	Q        string
	Sort     string // score_desc / score_asc / newest
}

// ScoredApplication 已评分的投递（用于 LLM 评委漏斗判断）。
type ScoredApplication struct {
	ID            string
	RuleScore     int
	SemanticScore *int
}

// ApplicationProcessData worker 处理投递所需的全部数据。
type ApplicationProcessData struct {
	ApplicationID    string
	JobID            string
	ResumeFileKey    string
	ResumeText       string
	ParsedResume     *domain.ParsedResume
	ResumeEmbedding  []float32
}

// ApplicationRepo 投递。
type ApplicationRepo interface {
	// Create 创建投递（stage=new）。
	Create(ctx context.Context, a *domain.Application) error
	// GetByCandidateAndJob 查询候选人是否已投递同一岗位。
	GetByCandidateAndJob(ctx context.Context, candidateID, jobID string) (*domain.Application, error)
	// ListByJob 岗位候选人分页列表（默认按 match_score 降序）。
	ListByJob(ctx context.Context, jobID string, f ApplicationListFilter, page, pageSize int) ([]domain.ApplicationInternal, int, error)
	// GetByID 投递详情（含岗位标题、候选人信息）。
	GetByID(ctx context.Context, id string) (*domain.ApplicationInternal, error)
	// UpdateStage 更新流转阶段。
	UpdateStage(ctx context.Context, id, stage string) error
	// UpdateMatch 写回匹配结果（分数/详情/硬性标记/解析结果/向量）。
	UpdateMatch(ctx context.Context, id string, score float64, detail *domain.MatchDetail, hardPass bool, parsed *domain.ParsedResume, embedding []float32, parseFailed bool) error
	// SetParseFailed 标记解析失败。
	SetParseFailed(ctx context.Context, id string) error
	// ListIDsByJob 岗位下全部投递 id（用于批量重算）。
	ListIDsByJob(ctx context.Context, jobID string) ([]string, error)
	// GetProcessData 获取 worker 流水线所需数据。
	GetProcessData(ctx context.Context, id string) (*ApplicationProcessData, error)
	// Stats 岗位投递漏斗统计。
	Stats(ctx context.Context, jobID string) (*domain.JobStats, error)
	// ListScoredByJob 已评分投递的规则分与语义分（LLM 评委漏斗）。
	ListScoredByJob(ctx context.Context, jobID string) ([]ScoredApplication, error)
	// ListPublicByCandidate 求职者本人的投递列表（状态由 stage 推导）。
	ListPublicByCandidate(ctx context.Context, candidateID string) ([]domain.ApplicationPublic, error)
}

// AuditLog 审计日志记录。
type AuditLog struct {
	ActorID    *string
	Action     string
	EntityType string
	EntityID   string
	Detail     any
	CreatedAt  time.Time
}

// AuditRepo 审计日志。
type AuditRepo interface {
	Insert(ctx context.Context, log *AuditLog) error
}

// Repos 聚合全部仓库。
type Repos struct {
	Users        UserRepo
	Departments  DepartmentRepo
	Jobs         JobRepo
	Candidates   CandidateRepo
	Applications ApplicationRepo
	Audit        AuditRepo
}
