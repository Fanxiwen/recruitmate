package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/file"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/google/uuid"
)

// TaskQueue 任务队列接口（由 worker.Queue 实现），服务层仅依赖接口。
type TaskQueue interface {
	// EnqueueResumeProcess 入队简历处理任务；version 用于唯一键（岗位更新版本）。
	EnqueueResumeProcess(ctx context.Context, applicationID string, version int64) error
	// EnqueueJobRescore 入队岗位重算任务。
	EnqueueJobRescore(ctx context.Context, jobID string) error
}

// JobService 岗位与候选人业务逻辑（含 RBAC 与部门隔离）。
type JobService struct {
	Jobs    repo.JobRepo
	Apps    repo.ApplicationRepo
	Audit   repo.AuditRepo
	Queue   TaskQueue
	Storage file.Storage
}

// NewJobService 构造岗位服务。
func NewJobService(jobs repo.JobRepo, apps repo.ApplicationRepo, audit repo.AuditRepo, queue TaskQueue, storage file.Storage) *JobService {
	return &JobService{Jobs: jobs, Apps: apps, Audit: audit, Queue: queue, Storage: storage}
}

// audit 写审计日志（actor 可能为 nil，表示匿名/候选人操作）。
func (s *JobService) audit(ctx context.Context, actor *Actor, action, entityType, entityID string, detail any) {
	writeAudit(ctx, s.Audit, actor, action, entityType, entityID, detail)
}

// writeAudit 通用审计写入。
func writeAudit(ctx context.Context, auditRepo repo.AuditRepo, actor *Actor, action, entityType, entityID string, detail any) {
	log := &repo.AuditLog{Action: action, EntityType: entityType, EntityID: entityID, Detail: detail}
	if actor != nil {
		log.ActorID = &actor.UserID
	}
	if err := auditRepo.Insert(ctx, log); err != nil {
		slog.Error("audit log insert failed", "action", action, "entity", entityID, "error", err)
	}
}

// normalizeInput 校验并规范化岗位输入。
func normalizeInput(input *domain.JobPostingInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return domain.NewError(400, domain.CodeBadRequest, "岗位标题不能为空")
	}
	if input.DepartmentID == "" {
		return domain.NewError(400, domain.CodeBadRequest, "请选择所属部门")
	}
	if input.JobType == "" {
		input.JobType = string(domain.JobTypeFullTime)
	}
	if !domain.ValidJobStatuses[domain.JobStatus(input.JobType)] && input.JobType != string(domain.JobTypeFullTime) && input.JobType != string(domain.JobTypeIntern) {
		return domain.NewError(400, domain.CodeBadRequest, "岗位类型不合法")
	}
	if input.Headcount <= 0 {
		input.Headcount = 1
	}
	if input.SalaryMin != nil && input.SalaryMax != nil && *input.SalaryMin > *input.SalaryMax {
		return domain.NewError(400, domain.CodeBadRequest, "薪资下限不能高于上限")
	}
	// 学历要求缺省为 any
	if input.Requirements.MinEducation == "" {
		input.Requirements.MinEducation = string(domain.EducationAny)
	}
	if input.Requirements.MustSkills == nil {
		input.Requirements.MustSkills = []string{}
	}
	if input.Requirements.NiceSkills == nil {
		input.Requirements.NiceSkills = []string{}
	}
	return nil
}

// Create 创建岗位（draft，owner=当前用户；hiring_manager 只能给自己部门建）。
func (s *JobService) Create(ctx context.Context, actor *Actor, input *domain.JobPostingInput) (*domain.JobPosting, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	if err := normalizeInput(input); err != nil {
		return nil, err
	}
	if actor.isHiringManager() && (actor.DepID == nil || *actor.DepID != input.DepartmentID) {
		return nil, domain.NewError(403, domain.CodeForbidden, "部门负责人只能为本部门创建岗位")
	}
	job := &domain.JobPosting{
		ID:           uuid.NewString(),
		Title:        input.Title,
		DepartmentID: input.DepartmentID,
		OwnerID:      actor.UserID,
		Status:       string(domain.JobStatusDraft),
		Headcount:    input.Headcount,
		SalaryMin:    input.SalaryMin,
		SalaryMax:    input.SalaryMax,
		Location:     input.Location,
		JobType:      input.JobType,
		Description:  input.Description,
		Requirements: input.Requirements,
	}
	if err := s.Jobs.Create(ctx, job); err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "创建岗位失败", err)
	}
	full, err := s.Jobs.Get(ctx, job.ID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "读取岗位失败", err)
	}
	s.audit(ctx, actor, "job.create", "job", job.ID, map[string]any{"title": job.Title})
	return full, nil
}

// Get 岗位详情（含权限校验）。
func (s *JobService) Get(ctx context.Context, actor *Actor, id string) (*domain.JobPosting, error) {
	job, err := s.Jobs.Get(ctx, id)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位失败", err)
	}
	if !actor.canAccessJob(job) {
		return nil, domain.NewError(403, domain.CodeForbidden, "无权访问该岗位")
	}
	return job, nil
}

// List 内部岗位列表（admin/hr 全量；hiring_manager 仅本部门）。
func (s *JobService) List(ctx context.Context, actor *Actor, status string, page, pageSize int) (*domain.Paginated[domain.JobPosting], error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	var deptID *string
	if actor.isHiringManager() {
		deptID = actor.DepID
	}
	items, total, err := s.Jobs.ListInternal(ctx, status, deptID, page, pageSize)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位列表失败", err)
	}
	return &domain.Paginated[domain.JobPosting]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Update 更新岗位（仅 draft/open 可改，owner 或 admin；
// open 岗位修改 requirements/description 后触发 job:rescore 重算）。
func (s *JobService) Update(ctx context.Context, actor *Actor, id string, input *domain.JobPostingInput) (*domain.JobPosting, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	if err := normalizeInput(input); err != nil {
		return nil, err
	}
	job, err := s.Jobs.Get(ctx, id)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位失败", err)
	}
	if job.OwnerID != actor.UserID && !actor.isAdmin() {
		return nil, domain.NewError(403, domain.CodeForbidden, "仅岗位负责人或管理员可修改")
	}
	if job.Status != string(domain.JobStatusDraft) && job.Status != string(domain.JobStatusOpen) {
		return nil, domain.NewError(409, domain.CodeConflict, "仅草稿或招聘中岗位可修改")
	}
	if actor.isHiringManager() && (actor.DepID == nil || *actor.DepID != input.DepartmentID) {
		return nil, domain.NewError(403, domain.CodeForbidden, "部门负责人只能修改本部门岗位")
	}

	requirementsChanged := !jsonEqual(job.Requirements, input.Requirements)
	descChanged := job.Description != input.Description
	wasOpen := job.Status == string(domain.JobStatusOpen)

	job.Title = input.Title
	job.DepartmentID = input.DepartmentID
	job.Headcount = input.Headcount
	job.SalaryMin = input.SalaryMin
	job.SalaryMax = input.SalaryMax
	job.Location = input.Location
	job.JobType = input.JobType
	job.Description = input.Description
	job.Requirements = input.Requirements
	if err := s.Jobs.Update(ctx, job); err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "更新岗位失败", err)
	}
	s.audit(ctx, actor, "job.update", "job", id, map[string]any{"title": job.Title})

	if wasOpen && (requirementsChanged || descChanged) {
		if err := s.Queue.EnqueueJobRescore(ctx, id); err != nil {
			slog.Error("enqueue job:rescore failed", "job_id", id, "error", err)
		}
	}
	full, err := s.Jobs.Get(ctx, id)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "读取岗位失败", err)
	}
	return full, nil
}

// Submit 提交审批：draft→pending（owner/admin）。
func (s *JobService) Submit(ctx context.Context, actor *Actor, id string) (*domain.JobPosting, error) {
	return s.transition(ctx, actor, id, "submit")
}

// Approve 审批发布：pending→open（admin 或该部门 hiring_manager）。
func (s *JobService) Approve(ctx context.Context, actor *Actor, id string) (*domain.JobPosting, error) {
	return s.transition(ctx, actor, id, "approve")
}

// Reject 驳回：pending→draft（同 approve 权限）。
func (s *JobService) Reject(ctx context.Context, actor *Actor, id string) (*domain.JobPosting, error) {
	return s.transition(ctx, actor, id, "reject")
}

// Close 关闭：open→closed（owner/admin/hr）。
func (s *JobService) Close(ctx context.Context, actor *Actor, id string) (*domain.JobPosting, error) {
	return s.transition(ctx, actor, id, "close")
}

// Reopen 重新开放：closed→open（owner/admin/hr）。
func (s *JobService) Reopen(ctx context.Context, actor *Actor, id string) (*domain.JobPosting, error) {
	return s.transition(ctx, actor, id, "reopen")
}

// transition 岗位状态流转统一入口。
func (s *JobService) transition(ctx context.Context, actor *Actor, id, action string) (*domain.JobPosting, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	job, err := s.Jobs.Get(ctx, id)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位失败", err)
	}

	var from, to string
	switch action {
	case "submit":
		if job.Status != string(domain.JobStatusDraft) {
			return nil, domain.NewError(409, domain.CodeConflict, "仅草稿岗位可提交审批")
		}
		if job.OwnerID != actor.UserID && !actor.isAdmin() {
			return nil, domain.NewError(403, domain.CodeForbidden, "仅岗位负责人或管理员可提交")
		}
		from, to = string(domain.JobStatusDraft), string(domain.JobStatusPending)
	case "approve":
		if job.Status != string(domain.JobStatusPending) {
			return nil, domain.NewError(409, domain.CodeConflict, "仅待审批岗位可批准")
		}
		if !actor.isAdmin() && !(actor.isHiringManager() && actor.DepID != nil && *actor.DepID == job.DepartmentID) {
			return nil, domain.NewError(403, domain.CodeForbidden, "仅管理员或本部门负责人可审批")
		}
		if job.OwnerID == actor.UserID && !actor.isAdmin() {
			return nil, domain.NewError(403, domain.CodeForbidden, "不能审批自己提交的岗位（四眼原则）")
		}
		from, to = string(domain.JobStatusPending), string(domain.JobStatusOpen)
	case "reject":
		if job.Status != string(domain.JobStatusPending) {
			return nil, domain.NewError(409, domain.CodeConflict, "仅待审批岗位可驳回")
		}
		if !actor.isAdmin() && !(actor.isHiringManager() && actor.DepID != nil && *actor.DepID == job.DepartmentID) {
			return nil, domain.NewError(403, domain.CodeForbidden, "仅管理员或本部门负责人可驳回")
		}
		if job.OwnerID == actor.UserID && !actor.isAdmin() {
			return nil, domain.NewError(403, domain.CodeForbidden, "不能驳回自己提交的岗位（四眼原则）")
		}
		from, to = string(domain.JobStatusPending), string(domain.JobStatusDraft)
	case "close":
		if job.Status != string(domain.JobStatusOpen) {
			return nil, domain.NewError(409, domain.CodeConflict, "仅招聘中岗位可关闭")
		}
		if job.OwnerID != actor.UserID && !actor.isAdmin() && !actor.isHR() {
			return nil, domain.NewError(403, domain.CodeForbidden, "权限不足")
		}
		from, to = string(domain.JobStatusOpen), string(domain.JobStatusClosed)
	case "reopen":
		if job.Status != string(domain.JobStatusClosed) {
			return nil, domain.NewError(409, domain.CodeConflict, "仅已关闭岗位可重新开放")
		}
		if job.OwnerID != actor.UserID && !actor.isAdmin() && !actor.isHR() {
			return nil, domain.NewError(403, domain.CodeForbidden, "权限不足")
		}
		from, to = string(domain.JobStatusClosed), string(domain.JobStatusOpen)
	default:
		return nil, domain.NewError(400, domain.CodeBadRequest, "未知操作")
	}

	var approverID *string
	if action == "approve" {
		approverID = &actor.UserID
	}
	if err := s.Jobs.SetStatus(ctx, id, to, approverID); err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "更新岗位状态失败", err)
	}
	s.audit(ctx, actor, "job."+action, "job", id, map[string]any{"from": from, "to": to})

	// approve/reopen 后触发岗位重算
	if action == "approve" || action == "reopen" {
		if err := s.Queue.EnqueueJobRescore(ctx, id); err != nil {
			slog.Error("enqueue job:rescore failed", "job_id", id, "error", err)
		}
	}
	full, err := s.Jobs.Get(ctx, id)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "读取岗位失败", err)
	}
	return full, nil
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

// normalizePage 分页参数规范化：page≥1，pageSize∈[1,100]，默认 20。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// ============ 公开端（无需登录） ============

// ListPublic 公开岗位列表（仅 status='open'，按 published_at desc）。
func (s *JobService) ListPublic(ctx context.Context, q domain.JobListQuery) (*domain.Paginated[domain.JobPosting], error) {
	page, pageSize := normalizePage(q.Page, q.PageSize)
	items, total, err := s.Jobs.ListPublic(ctx, repo.JobListFilter{
		Q:            q.Q,
		DepartmentID: q.DepartmentID,
		JobType:      q.JobType,
	}, page, pageSize)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位列表失败", err)
	}
	return &domain.Paginated[domain.JobPosting]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetPublic 公开岗位详情（仅 open）。
func (s *JobService) GetPublic(ctx context.Context, id string) (*domain.JobPosting, error) {
	job, err := s.Jobs.Get(ctx, id)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位失败", err)
	}
	if job.Status != string(domain.JobStatusOpen) {
		return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在或未发布")
	}
	return job, nil
}

// ============ 候选人（投递） ============

// requireJobAccess 校验岗位访问权限，返回岗位。
func (s *JobService) requireJobAccess(ctx context.Context, actor *Actor, jobID string) (*domain.JobPosting, error) {
	job, err := s.Jobs.Get(ctx, jobID)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "岗位不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询岗位失败", err)
	}
	if !actor.canAccessJob(job) {
		return nil, domain.NewError(403, domain.CodeForbidden, "无权访问该岗位")
	}
	return job, nil
}

// requireApplicationAccess 校验投递访问权限，返回投递。
func (s *JobService) requireApplicationAccess(ctx context.Context, actor *Actor, appID string) (*domain.ApplicationInternal, error) {
	app, err := s.Apps.GetByID(ctx, appID)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "投递记录不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询投递失败", err)
	}
	if _, err := s.requireJobAccess(ctx, actor, app.JobID); err != nil {
		return nil, err
	}
	return app, nil
}

// ListApplications 岗位候选人分页列表（默认按 match_score 降序）。
func (s *JobService) ListApplications(ctx context.Context, actor *Actor, jobID string, f repo.ApplicationListFilter, page, pageSize int) (*domain.Paginated[domain.ApplicationInternal], error) {
	if _, err := s.requireJobAccess(ctx, actor, jobID); err != nil {
		return nil, err
	}
	items, total, err := s.Apps.ListByJob(ctx, jobID, f, page, pageSize)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询候选人列表失败", err)
	}
	return &domain.Paginated[domain.ApplicationInternal]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetApplication 投递详情（含 Offer 审批单、流转时间线与面试记录）。
func (s *JobService) GetApplication(ctx context.Context, actor *Actor, appID string) (*domain.ApplicationInternal, error) {
	app, err := s.requireApplicationAccess(ctx, actor, appID)
	if err != nil {
		return nil, err
	}
	// 最近一次 Offer（任意状态，便于展示历史）
	if offer, err := s.Apps.GetLatestOfferByApplication(ctx, appID); err == nil {
		app.Offer = offer
	}
	// 流转时间线
	if events, err := s.Apps.ListApplicationEvents(ctx, appID); err == nil {
		app.Events = events
	}
	// 面试记录
	if interviews, err := s.Apps.ListInterviews(ctx, appID); err == nil {
		app.Interviews = interviews
	}
	s.audit(ctx, actor, "application.view", "application", appID, nil)
	return app, nil
}

// ============ 候选人中心与我的待办 ============

// ListCandidates 候选人中心：全局候选人列表（阶段/部门/岗位/关键词筛选）。
func (s *JobService) ListCandidates(ctx context.Context, actor *Actor, f repo.CandidateListFilter, page, pageSize int) (*domain.Paginated[domain.ApplicationInternal], error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	if actor.isHiringManager() {
		if f.DepartmentID != nil && *f.DepartmentID != *actor.DepID {
			return nil, domain.NewError(403, domain.CodeForbidden, "无权查看其他部门候选人")
		}
		// 强制本部门范围
		dep := *actor.DepID
		f.DepartmentID = &dep
	}
	items, total, err := s.Apps.ListCandidates(ctx, f, page, pageSize)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询候选人列表失败", err)
	}
	return &domain.Paginated[domain.ApplicationInternal]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListInterviewTodos 我的待办（面试类）：按角色分类进行中的候选人。
func (s *JobService) ListInterviewTodos(ctx context.Context, actor *Actor) ([]domain.InterviewTodoItem, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	var deptID *string
	if actor.isHiringManager() {
		deptID = actor.DepID
	}
	apps, err := s.Apps.ListActiveApplications(ctx, deptID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询待办失败", err)
	}

	out := make([]domain.InterviewTodoItem, 0)
	for _, a := range apps {
		hr := interviewOf(a.Interviews, "hr")
		mg := interviewOf(a.Interviews, "manager")
		item := domain.InterviewTodoItem{Application: a}
		switch {
		case a.Stage == string(domain.StageNew):
			item.Kind = "screen" // 待初筛（HR）
		case a.Stage == string(domain.StageScreening):
			item.Kind = "schedule_hr" // 待安排 HR 面（HR）
		case a.Stage == string(domain.StageInterview):
			if hr != nil && hr.Status == "completed" && hr.Result == "pass" {
				item.Kind = "schedule_manager" // HR 面通过，待安排负责人面（HR）
				item.Interview = mg
			} else if hr != nil && hr.Status == "scheduled" {
				item.Kind = "review_hr" // HR 面待评价（HR）
				item.Interview = hr
			} else {
				continue
			}
		case a.Stage == string(domain.StageManagerInterview):
			if mg != nil && mg.Status == "scheduled" {
				item.Kind = "review_manager" // 负责人面待评价（部门负责人）
				item.Interview = mg
			} else if mg != nil && mg.Status == "completed" && mg.Result == "pass" {
				item.Kind = "offer_ready" // 负责人面通过，待发起 Offer（HR）
				item.Interview = mg
			} else {
				continue
			}
		default:
			continue
		}
		// 角色过滤：HR 看 screen/schedule_hr/schedule_manager/offer_ready/review_hr；
		// 部门负责人看 review_manager；管理员全部可见（兜底审批/评价）。
		if actor.isHiringManager() {
			if item.Kind != "review_manager" {
				continue
			}
		} else if !actor.isAdmin() && item.Kind == "review_manager" {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func interviewOf(interviews []domain.Interview, round string) *domain.Interview {
	for i := range interviews {
		if interviews[i].Round == round {
			return &interviews[i]
		}
	}
	return nil
}

// ============ 面试实体（安排 / 完成） ============

// ScheduleInterview 安排一轮面试（时间必填），并推进阶段：
//   - round=hr：stage 须为 screening（或 interview 改期）→ 进入 interview
//   - round=manager：须 HR 面已通过，stage 须为 interview（或 manager_interview 改期）→ 进入 manager_interview
func (s *JobService) ScheduleInterview(ctx context.Context, actor *Actor, appID, round string, scheduledAt time.Time) (*domain.Interview, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	if !actor.isAdmin() && !actor.isHR() {
		return nil, domain.NewError(403, domain.CodeForbidden, "仅 HR 或管理员可安排面试")
	}
	app, err := s.requireApplicationAccess(ctx, actor, appID)
	if err != nil {
		return nil, err
	}
	// 加载面试记录（GetByID 不附带）
	interviews, err := s.Apps.ListInterviews(ctx, appID)
	if err != nil {
		slog.Error("list interviews failed", "application_id", appID, "error", err)
		return nil, domain.WrapError(500, domain.CodeInternal, "查询面试记录失败", err)
	}
	app.Interviews = interviews
	now := time.Now()
	if scheduledAt.Before(now.Add(-time.Minute)) {
		return nil, domain.NewError(400, domain.CodeBadRequest, "面试时间不能早于当前时间")
	}

	fromStage := app.Stage
	switch round {
	case "hr":
		if app.Stage != string(domain.StageScreening) && app.Stage != string(domain.StageInterview) {
			return nil, domain.NewError(409, domain.CodeConflict, "仅「初筛通过」的候选人可安排 HR 面")
		}
		if app.Stage == string(domain.StageScreening) {
			if err := s.Apps.UpdateStage(ctx, appID, string(domain.StageInterview), ""); err != nil {
				return nil, domain.WrapError(500, domain.CodeInternal, "更新阶段失败", err)
			}
			fromStage = string(domain.StageScreening)
		}
	case "manager":
		if app.Stage != string(domain.StageInterview) && app.Stage != string(domain.StageManagerInterview) {
			return nil, domain.NewError(409, domain.CodeConflict, "仅「HR 面通过」的候选人可安排负责人面")
		}
		hr := interviewOf(app.Interviews, "hr")
		if hr == nil || hr.Result != "pass" {
			return nil, domain.NewError(409, domain.CodeConflict, "请先完成 HR 面并确认通过")
		}
		if app.Stage == string(domain.StageInterview) {
			if err := s.Apps.UpdateStage(ctx, appID, string(domain.StageManagerInterview), ""); err != nil {
				return nil, domain.WrapError(500, domain.CodeInternal, "更新阶段失败", err)
			}
			fromStage = string(domain.StageInterview)
		}
	default:
		return nil, domain.NewError(400, domain.CodeBadRequest, "面试轮次不合法")
	}

	iv, err := s.Apps.UpsertInterview(ctx, appID, round, scheduledAt)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "安排面试失败", err)
	}
	s.recordEvent(ctx, appID, fromStage, app.Stage, "interview_schedule", actor,
		fmt.Sprintf("%s 面试时间：%s", roundLabel(round), scheduledAt.Format("2006-01-02 15:04")))
	s.audit(ctx, actor, "application.interview.schedule", "application", appID, map[string]any{"round": round})
	return iv, nil
}

// CompleteInterview 完成一轮面试：评价必填，结论驱动流程。
//   - round=hr：由 HR/管理员评价；通过 → 待安排负责人面；不通过 → rejected
//   - round=manager：由部门负责人/管理员评价；通过 → 待发起 Offer；不通过 → rejected
func (s *JobService) CompleteInterview(ctx context.Context, actor *Actor, appID, round, result, feedback string) (*domain.ApplicationInternal, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	app, err := s.requireApplicationAccess(ctx, actor, appID)
	if err != nil {
		return nil, err
	}
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return nil, domain.NewError(400, domain.CodeBadRequest, "请填写面试评价")
	}
	if result != "pass" && result != "fail" {
		return nil, domain.NewError(400, domain.CodeBadRequest, "面试结论不合法")
	}

	switch round {
	case "hr":
		if !actor.isAdmin() && !actor.isHR() {
			return nil, domain.NewError(403, domain.CodeForbidden, "仅 HR 或管理员可完成 HR 面评价")
		}
		if app.Stage != string(domain.StageInterview) {
			return nil, domain.NewError(409, domain.CodeConflict, "候选人不在 HR 面阶段")
		}
	case "manager":
		if !actor.isAdmin() && !actor.isHiringManager() {
			return nil, domain.NewError(403, domain.CodeForbidden, "仅部门负责人或管理员可完成负责人面评价")
		}
		if app.Stage != string(domain.StageManagerInterview) {
			return nil, domain.NewError(409, domain.CodeConflict, "候选人不在部门负责人面阶段")
		}
	default:
		return nil, domain.NewError(400, domain.CodeBadRequest, "面试轮次不合法")
	}

	if err := s.Apps.CompleteInterview(ctx, appID, round, result, feedback, actor.UserID); err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(409, domain.CodeConflict, "该轮面试尚未安排")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "提交面试评价失败", err)
	}
	s.recordEvent(ctx, appID, app.Stage, app.Stage, "interview_complete", actor,
		fmt.Sprintf("%s：%s · %s", roundLabel(round), resultLabel(result), feedback))

	if result == "fail" {
		if err := s.Apps.UpdateStage(ctx, appID, string(domain.StageRejected),
			fmt.Sprintf("%s未通过：%s", roundLabel(round), feedback)); err != nil {
			return nil, domain.WrapError(500, domain.CodeInternal, "更新阶段失败", err)
		}
		s.recordEvent(ctx, appID, app.Stage, string(domain.StageRejected), "stage_change", actor,
			fmt.Sprintf("%s未通过", roundLabel(round)))
	}
	updated, err := s.Apps.GetByID(ctx, appID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "读取投递失败", err)
	}
	return updated, nil
}

func roundLabel(round string) string {
	if round == "hr" {
		return "HR 面"
	}
	return "部门负责人面"
}

func resultLabel(result string) string {
	if result == "pass" {
		return "通过"
	}
	return "不通过"
}

// SetStage 流转阶段（OA 状态机：非法流转拒绝；转 rejected 必填原因）。
func (s *JobService) SetStage(ctx context.Context, actor *Actor, appID, stage, reason string) (*domain.ApplicationInternal, error) {
	if !domain.ValidStages[domain.ApplicationStage(stage)] {
		return nil, domain.NewError(400, domain.CodeBadRequest, "流转阶段不合法")
	}
	app, err := s.requireApplicationAccess(ctx, actor, appID)
	if err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if stage == string(domain.StageRejected) && reason == "" {
		return nil, domain.NewError(400, domain.CodeBadRequest, "请填写淘汰原因")
	}
	if !domain.CanTransition(domain.ApplicationStage(app.Stage), domain.ApplicationStage(stage)) {
		return nil, domain.NewError(409, domain.CodeConflict,
			fmt.Sprintf("不允许从「%s」流转到「%s」（请按标准流程操作）", stageLabel(app.Stage), stageLabel(stage)))
	}
	// offer_pending 只能通过 Offer 审批接口流转，防止绕过审批链
	if stage == string(domain.StageOffered) || stage == string(domain.StageOfferPending) {
		return nil, domain.NewError(409, domain.CodeConflict, "Offer 阶段请通过 Offer 审批流程操作")
	}

	if err := s.Apps.UpdateStage(ctx, appID, stage, reason); err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(404, domain.CodeNotFound, "投递记录不存在")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "更新阶段失败", err)
	}
	if stage == string(domain.StageHired) {
		s.maybeCloseIfFilled(ctx, app.JobID) // 满编自动关闭岗位
	}
	action := "stage_change"
	if stage == string(domain.StageInterview) && reason != "" {
		action = "feedback" // 进入面试且带备注 → 面试评价
	}
	s.recordEvent(ctx, appID, app.Stage, stage, action, actor, reason)
	s.audit(ctx, actor, "application.stage", "application", appID, map[string]any{"from": app.Stage, "to": stage})
	updated, err := s.Apps.GetByID(ctx, appID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "读取投递失败", err)
	}
	return updated, nil
}

// OfferRequest HR/管理员发起 Offer 审批（候选人须处于部门负责人面阶段；
// salary 仅为建议薪资，最终薪资由部门负责人审批时确定）。
func (s *JobService) OfferRequest(ctx context.Context, actor *Actor, appID, salary, joinDate, note string) (*domain.Offer, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	if !actor.isAdmin() && !actor.isHR() {
		return nil, domain.NewError(403, domain.CodeForbidden, "仅 HR 或管理员可发起 Offer 审批")
	}
	app, err := s.requireApplicationAccess(ctx, actor, appID)
	if err != nil {
		return nil, err
	}
	if app.Stage != string(domain.StageManagerInterview) {
		return nil, domain.NewError(409, domain.CodeConflict, "仅「部门负责人面」的候选人可发起 Offer 审批")
	}
	// 负责人面必须已完成并通过（面试结论驱动流程）
	interviews, err := s.Apps.ListInterviews(ctx, appID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询面试记录失败", err)
	}
	mg := interviewOf(interviews, "manager")
	if mg == nil || mg.Result != "pass" {
		return nil, domain.NewError(409, domain.CodeConflict, "请先完成部门负责人面并确认通过")
	}
	offer := &domain.Offer{Salary: strings.TrimSpace(salary), JoinDate: strings.TrimSpace(joinDate), Note: strings.TrimSpace(note)}
	offer, err = s.Apps.CreateOffer(ctx, offer, appID, actor.UserID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "创建 Offer 失败", err)
	}
	if err := s.Apps.UpdateStage(ctx, appID, string(domain.StageOfferPending), ""); err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "更新阶段失败", err)
	}
	s.recordEvent(ctx, appID, app.Stage, string(domain.StageOfferPending), "offer_request", actor, note)
	s.audit(ctx, actor, "application.offer.request", "application", appID, map[string]any{"offer_id": offer.ID})
	return offer, nil
}

// OfferApprove 部门负责人或管理员审批通过 Offer：确定最终薪资（必填）。
func (s *JobService) OfferApprove(ctx context.Context, actor *Actor, appID, salary string) (*domain.ApplicationInternal, error) {
	return s.decideOffer(ctx, actor, appID, "approved", salary, "", true)
}

// OfferReject 驳回 Offer（必填原因），候选人回退到部门负责人面。
func (s *JobService) OfferReject(ctx context.Context, actor *Actor, appID, reason string) (*domain.ApplicationInternal, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, domain.NewError(400, domain.CodeBadRequest, "请填写驳回原因")
	}
	return s.decideOffer(ctx, actor, appID, "rejected", "", reason, false)
}

func (s *JobService) decideOffer(ctx context.Context, actor *Actor, appID, decision, salary, reason string, approve bool) (*domain.ApplicationInternal, error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	app, err := s.requireApplicationAccess(ctx, actor, appID)
	if err != nil {
		return nil, err
	}
	offer, err := s.Apps.GetPendingOfferByApplication(ctx, appID)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(409, domain.CodeConflict, "该候选人没有待审批的 Offer")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询 Offer 失败", err)
	}
	// 审批权限：管理员或该岗位部门的负责人
	if !actor.isAdmin() && !(actor.isHiringManager() && actor.DepID != nil) {
		return nil, domain.NewError(403, domain.CodeForbidden, "仅管理员或部门负责人可审批 Offer")
	}
	// 四眼原则：发起人不能审批自己的 Offer
	requestedBy, err := s.Apps.GetOfferRequestedBy(ctx, offer.ID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "读取 Offer 信息失败", err)
	}
	if requestedBy == actor.UserID && !actor.isAdmin() {
		return nil, domain.NewError(403, domain.CodeForbidden, "不能审批自己发起的 Offer（请由部门负责人或管理员审批）")
	}
	// 薪资由部门负责人决定：审批通过时必填最终薪资
	finalSalary := ""
	if approve {
		finalSalary = strings.TrimSpace(salary)
		if finalSalary == "" {
			return nil, domain.NewError(400, domain.CodeBadRequest, "请填写最终薪资（薪资由部门负责人确定）")
		}
	}

	if err := s.Apps.DecideOffer(ctx, offer.ID, decision, actor.UserID, finalSalary); err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(409, domain.CodeConflict, "Offer 已被处理")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "审批 Offer 失败", err)
	}

	var targetStage, action string
	if approve {
		targetStage, action = string(domain.StageOffered), "offer_approve"
	} else {
		targetStage, action = string(domain.StageManagerInterview), "offer_reject"
	}
	if err := s.Apps.UpdateStage(ctx, appID, targetStage, reason); err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "更新阶段失败", err)
	}
	s.recordEvent(ctx, appID, app.Stage, targetStage, action, actor, reason)
	s.audit(ctx, actor, "application.offer."+decision, "application", appID, map[string]any{"offer_id": offer.ID})
	updated, err := s.Apps.GetByID(ctx, appID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "读取投递失败", err)
	}
	return updated, nil
}

// recordEvent 写入流转时间线。
func (s *JobService) recordEvent(ctx context.Context, appID, fromStage, toStage, action string, actor *Actor, reason string) {
	actorID, actorName := "", ""
	if actor != nil {
		actorID, actorName = actor.UserID, actor.Name
	}
	if err := s.Apps.InsertApplicationEvent(ctx, appID, fromStage, toStage, action, actorID, actorName, reason); err != nil {
		slog.Error("record application event failed", "application_id", appID, "error", err)
	}
}

// stageLabel 阶段中文名（错误提示用）。
func stageLabel(stage string) string {
	labels := map[string]string{
		"new": "新简历", "screening": "初筛通过", "interview": "HR初面",
		"manager_interview": "部门负责人面", "offer_pending": "Offer审批中",
		"offered": "已发Offer", "hired": "已入职", "rejected": "已淘汰",
	}
	if l, ok := labels[stage]; ok {
		return l
	}
	return stage
}

// Batch 批量操作：action=stage|reject|hired；越权 ids 拒绝，返回实际更新数。
func (s *JobService) Batch(ctx context.Context, actor *Actor, ids []string, action, stage, reason string) (int, error) {
	if actor == nil {
		return 0, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	if len(ids) == 0 {
		return 0, domain.NewError(400, domain.CodeBadRequest, "请选择要操作的投递")
	}
	var target string
	switch action {
	case "stage":
		if !domain.ValidStages[domain.ApplicationStage(stage)] {
			return 0, domain.NewError(400, domain.CodeBadRequest, "流转阶段不合法")
		}
		target = stage
	case "reject":
		target = string(domain.StageRejected)
	case "hired":
		target = string(domain.StageHired)
	default:
		return 0, domain.NewError(400, domain.CodeBadRequest, "批量操作类型不合法")
	}
	reason = strings.TrimSpace(reason)
	if target == string(domain.StageRejected) && reason == "" {
		return 0, domain.NewError(400, domain.CodeBadRequest, "请填写淘汰原因")
	}

	updated := 0
	for _, id := range ids {
		app, err := s.Apps.GetByID(ctx, id)
		if err != nil {
			continue // 跳过不存在/越权
		}
		job, err := s.Jobs.Get(ctx, app.JobID)
		if err != nil || !actor.canAccessJob(job) {
			continue // 越权投递拒绝
		}
		if !domain.CanTransition(domain.ApplicationStage(app.Stage), domain.ApplicationStage(target)) {
			continue // 状态机不允许（如批量通过一个已 Offer 阶段的候选人）
		}
		if err := s.Apps.UpdateStage(ctx, id, target, reason); err == nil {
			updated++
			s.recordEvent(ctx, id, app.Stage, target, "stage_change", actor, reason)
			s.audit(ctx, actor, "application.batch", "application", id, map[string]any{"action": action, "to": target})
			if target == string(domain.StageHired) {
				s.maybeCloseIfFilled(ctx, app.JobID) // 满编自动关闭岗位
			}
		}
	}
	return updated, nil
}

// ListOfferApprovals 审批中心：Offer 待审批列表（hiring_manager 仅本部门）。
func (s *JobService) ListOfferApprovals(ctx context.Context, actor *Actor, page, pageSize int) (*domain.Paginated[domain.ApprovalOfferItem], error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	var deptID *string
	if actor.isHiringManager() {
		deptID = actor.DepID
	}
	items, total, err := s.Apps.ListOfferPending(ctx, deptID, page, pageSize)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询 Offer 待审批列表失败", err)
	}
	return &domain.Paginated[domain.ApprovalOfferItem]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListPendingApplications 待处理候选人列表（stage=new 新简历；
// hiring_manager 仅本部门）。投递后 HR 在此集中初筛。
func (s *JobService) ListPendingApplications(ctx context.Context, actor *Actor, page, pageSize int) (*domain.Paginated[domain.ApplicationInternal], error) {
	if actor == nil {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "未登录")
	}
	var deptID *string
	if actor.isHiringManager() {
		deptID = actor.DepID
	}
	items, total, err := s.Apps.ListPendingApplications(ctx, deptID, page, pageSize)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询待处理候选人失败", err)
	}
	return &domain.Paginated[domain.ApplicationInternal]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// maybeCloseIfFilled 入职人数达到岗位需求数时自动关闭岗位（招聘进度推进）。
func (s *JobService) maybeCloseIfFilled(ctx context.Context, jobID string) {
	job, err := s.Jobs.Get(ctx, jobID)
	if err != nil || job.Status != string(domain.JobStatusOpen) {
		return
	}
	st, err := s.Apps.Stats(ctx, jobID)
	if err != nil {
		return
	}
	if st.ByStage[string(domain.StageHired)] >= job.Headcount {
		if err := s.Jobs.SetStatus(ctx, jobID, string(domain.JobStatusClosed), nil); err != nil {
			slog.Error("auto close job failed", "job_id", jobID, "error", err)
			return
		}
		slog.Info("job auto-closed: headcount filled", "job_id", jobID,
			"hired", st.ByStage[string(domain.StageHired)], "headcount", job.Headcount)
	}
}

// Stats 岗位投递漏斗统计。
func (s *JobService) Stats(ctx context.Context, actor *Actor, jobID string) (*domain.JobStats, error) {
	if _, err := s.requireJobAccess(ctx, actor, jobID); err != nil {
		return nil, err
	}
	st, err := s.Apps.Stats(ctx, jobID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "统计失败", err)
	}
	return st, nil
}

// ResumeURL 简历文件预签名下载地址（1 小时）。
// 注意：预签名 URL 的主机取自 S3_ENDPOINT（容器内为 minio:9000），浏览器无法访问；
// 前端下载请使用鉴权流式接口 ResumeFile（GET /internal/applications/:id/resume）。
func (s *JobService) ResumeURL(ctx context.Context, actor *Actor, appID string) (string, error) {
	if _, err := s.requireApplicationAccess(ctx, actor, appID); err != nil {
		return "", err
	}
	key, err := s.resumeFileKey(ctx, appID)
	if err != nil {
		return "", err
	}
	url, err := s.Storage.PresignedGetURL(ctx, key, time.Hour)
	if err != nil {
		return "", domain.WrapError(500, domain.CodeInternal, "生成下载链接失败", err)
	}
	return url, nil
}

// ResumeFile 读取简历文件内容（鉴权流式下载）：
// 通过 API 自身转发文件，避免把 MinIO 内网地址暴露给浏览器，
// 同时复用内部端的 JWT 鉴权与部门数据隔离。
func (s *JobService) ResumeFile(ctx context.Context, actor *Actor, appID string) ([]byte, string, error) {
	app, err := s.requireApplicationAccess(ctx, actor, appID)
	if err != nil {
		return nil, "", err
	}
	key, err := s.resumeFileKey(ctx, appID)
	if err != nil {
		return nil, "", err
	}
	data, err := s.Storage.Download(ctx, key)
	if err != nil {
		return nil, "", domain.WrapError(500, domain.CodeInternal, "读取简历文件失败", err)
	}
	ext := filepath.Ext(key)
	filename := fmt.Sprintf("%s-简历%s", app.CandidateName, ext)
	return data, filename, nil
}

// resumeFileKey 读取投递的简历文件 key。
func (s *JobService) resumeFileKey(ctx context.Context, appID string) (string, error) {
	data, err := s.Apps.GetProcessData(ctx, appID)
	if err != nil {
		if err == repo.ErrNotFound {
			return "", domain.NewError(404, domain.CodeNotFound, "投递记录不存在")
		}
		return "", domain.WrapError(500, domain.CodeInternal, "查询投递失败", err)
	}
	if data.ResumeFileKey == "" {
		return "", domain.NewError(404, domain.CodeNotFound, "该投递没有简历文件")
	}
	return data.ResumeFileKey, nil
}
