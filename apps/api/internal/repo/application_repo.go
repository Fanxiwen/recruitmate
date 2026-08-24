package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CandidateStore 求职者仓库。
type CandidateStore Postgres

// GetOrCreateByEmail 按邮箱（小写）查或建候选人。
func (s *CandidateStore) GetOrCreateByEmail(ctx context.Context, email, name, phone string) (*domain.Candidate, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
INSERT INTO candidates (id, email, name, phone)
VALUES ($1, $2, $3, $4)
ON CONFLICT (email) DO NOTHING
RETURNING id`, uuid.NewString(), email, name, phone).Scan(&id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repo: create candidate: %w", err)
		}
		// 已存在：查询现有记录
		if err := s.pool.QueryRow(ctx, `SELECT id, email, name, phone FROM candidates WHERE email = $1`, email).
			Scan(&id, &email, &name, &phone); err != nil {
			return nil, fmt.Errorf("repo: get candidate: %w", err)
		}
	}
	return &domain.Candidate{ID: id, Email: email, Name: name, Phone: phone}, nil
}

// GetByEmail 按邮箱查询候选人。
func (s *CandidateStore) GetByEmail(ctx context.Context, email string) (*domain.Candidate, error) {
	var c domain.Candidate
	err := s.pool.QueryRow(ctx, `SELECT id, email, name, phone FROM candidates WHERE email = $1`, email).
		Scan(&c.ID, &c.Email, &c.Name, &c.Phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

var _ CandidateRepo = (*CandidateStore)(nil)

// ============ 投递 ============

// ApplicationStore 投递仓库。
type ApplicationStore Postgres

// Create 创建投递。
func (s *ApplicationStore) Create(ctx context.Context, a *domain.Application) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO applications (id, candidate_id, job_id, stage, source, resume_file_key, resume_text)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		a.ID, a.CandidateID, a.JobID, a.Stage, a.Source, nullableString(a.ResumeFileKey), nullableString(a.ResumeText))
	if err != nil {
		return fmt.Errorf("repo: create application: %w", err)
	}
	return nil
}

// GetByCandidateAndJob 查询候选人是否已投递同一岗位。
func (s *ApplicationStore) GetByCandidateAndJob(ctx context.Context, candidateID, jobID string) (*domain.Application, error) {
	var a domain.Application
	err := s.pool.QueryRow(ctx, `
SELECT id, candidate_id, job_id, stage, source, COALESCE(resume_file_key,''), COALESCE(resume_text,'')
FROM applications WHERE candidate_id = $1 AND job_id = $2`, candidateID, jobID).
		Scan(&a.ID, &a.CandidateID, &a.JobID, &a.Stage, &a.Source, &a.ResumeFileKey, &a.ResumeText)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// 候选人列表查询用字段（不含 resume_text，保持列表响应轻量）。
const appInternalSelect = `
SELECT a.id, a.job_id, j.title AS job_title,
       a.candidate_id, c.name AS candidate_name, c.email, c.phone,
       a.stage, a.source, a.submitted_at, a.match_score, a.hard_pass, a.parse_failed,
       a.parsed_resume, a.match_detail,
       (a.resume_file_key IS NOT NULL AND a.resume_file_key <> '') AS has_resume_file,
       a.reject_reason, a.interview_feedback
FROM applications a
JOIN candidates c ON c.id = a.candidate_id
JOIN job_postings j ON j.id = a.job_id`

// 候选人详情查询字段（含 resume_text，供详情接口快速预览简历原文）。
const appInternalDetailSelect = `
SELECT a.id, a.job_id, j.title AS job_title,
       a.candidate_id, c.name AS candidate_name, c.email, c.phone,
       a.stage, a.source, a.submitted_at, a.match_score, a.hard_pass, a.parse_failed,
       a.parsed_resume, a.match_detail,
       (a.resume_file_key IS NOT NULL AND a.resume_file_key <> '') AS has_resume_file,
       COALESCE(a.resume_text, ''),
       a.reject_reason, a.interview_feedback
FROM applications a
JOIN candidates c ON c.id = a.candidate_id
JOIN job_postings j ON j.id = a.job_id`

func scanApplicationInternal(row pgx.Row) (*domain.ApplicationInternal, error) {
	var a domain.ApplicationInternal
	var parsed []byte
	var detail []byte
	err := row.Scan(&a.ID, &a.JobID, &a.JobTitle,
		&a.CandidateID, &a.CandidateName, &a.Email, &a.Phone,
		&a.Stage, &a.Source, &a.SubmittedAt, &a.MatchScore, &a.HardPass, &a.ParseFailed,
		&parsed, &detail, &a.HasResumeFile, &a.RejectReason, &a.InterviewFeedback)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	fillApplicationJSON(&a, parsed, detail)
	return &a, nil
}

// scanApplicationInternalDetail 详情行扫描（额外含 resume_text）。
func scanApplicationInternalDetail(row pgx.Row) (*domain.ApplicationInternal, error) {
	var a domain.ApplicationInternal
	var parsed []byte
	var detail []byte
	err := row.Scan(&a.ID, &a.JobID, &a.JobTitle,
		&a.CandidateID, &a.CandidateName, &a.Email, &a.Phone,
		&a.Stage, &a.Source, &a.SubmittedAt, &a.MatchScore, &a.HardPass, &a.ParseFailed,
		&parsed, &detail, &a.HasResumeFile, &a.ResumeText, &a.RejectReason, &a.InterviewFeedback)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	fillApplicationJSON(&a, parsed, detail)
	return &a, nil
}

// fillApplicationJSON 解析 parsed_resume / match_detail 两列 JSON。
// 解析后统一做空数组归一化（null → []），保证前端契约：嵌套数组永远不是 null。
func fillApplicationJSON(a *domain.ApplicationInternal, parsed, detail []byte) {
	if len(parsed) > 0 && string(parsed) != "null" {
		var p domain.ParsedResume
		if err := json.Unmarshal(parsed, &p); err == nil {
			if p.Skills == nil {
				p.Skills = []string{}
			}
			if p.Education == nil {
				p.Education = []domain.EducationItem{}
			}
			if p.WorkExperience == nil {
				p.WorkExperience = []domain.WorkExperienceItem{}
			}
			a.ParsedResume = &p
		}
	}
	if len(detail) > 0 && string(detail) != "null" {
		var m domain.MatchDetail
		if err := json.Unmarshal(detail, &m); err == nil {
			if m.Strengths == nil {
				m.Strengths = []string{}
			}
			if m.Gaps == nil {
				m.Gaps = []string{}
			}
			if m.HardChecks == nil {
				m.HardChecks = []domain.HardCheck{}
			}
			a.MatchDetail = &m
		}
	}
}

// ListByJob 岗位候选人分页列表。
func (s *ApplicationStore) ListByJob(ctx context.Context, jobID string, f ApplicationListFilter, page, pageSize int) ([]domain.ApplicationInternal, int, error) {
	where := ` WHERE a.job_id = $1`
	args := []any{jobID, f.Stage, f.HardPass, f.Q}
	where += `
  AND ($2::text = '' OR a.stage = $2)
  AND ($3::text NOT IN ('only','exclude') OR ($3 = 'only' AND a.hard_pass) OR ($3 = 'exclude' AND NOT a.hard_pass))
  AND ($4::text = '' OR c.name ILIKE '%'||$4||'%' OR c.email ILIKE '%'||$4||'%' OR COALESCE(a.parsed_resume::text,'') ILIKE '%'||$4||'%')`

	var total int
	if err := s.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM applications a JOIN candidates c ON c.id = a.candidate_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := `
ORDER BY
  CASE $5 WHEN 'score_asc' THEN a.match_score END ASC NULLS LAST,
  CASE $5 WHEN 'score_desc' THEN a.match_score END DESC NULLS LAST,
  a.submitted_at DESC`
	if f.Sort == "newest" {
		orderBy = ` ORDER BY a.submitted_at DESC`
	}
	args = append(args, f.Sort, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, appInternalSelect+where+orderBy+` LIMIT $6 OFFSET $7`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// 空结果序列化为 [] 而非 null（前端契约）
	out := make([]domain.ApplicationInternal, 0)
	for rows.Next() {
		a, err := scanApplicationInternal(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	return out, total, rows.Err()
}

// GetByID 投递详情（含 resume_text）。
func (s *ApplicationStore) GetByID(ctx context.Context, id string) (*domain.ApplicationInternal, error) {
	return scanApplicationInternalDetail(s.pool.QueryRow(ctx, appInternalDetailSelect+` WHERE a.id = $1`, id))
}

// UpdateStage 更新流转阶段（OA：转 rejected 记录淘汰原因；进面试轮次且带备注时记录面试评价）。
func (s *ApplicationStore) UpdateStage(ctx context.Context, id, stage, reason string) error {
	rejectReason := ""
	interviewFeedback := ""
	if stage == string(domain.StageRejected) {
		rejectReason = reason
	}
	if (stage == string(domain.StageInterview) || stage == string(domain.StageManagerInterview)) && reason != "" {
		interviewFeedback = reason
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE applications SET
  stage = $2,
  reject_reason = CASE WHEN $3 <> '' THEN $3 ELSE reject_reason END,
  interview_feedback = CASE WHEN $4 <> '' THEN $4 ELSE interview_feedback END
WHERE id = $1`, id, stage, rejectReason, interviewFeedback)
	if err != nil {
		return fmt.Errorf("repo: update stage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============ Offer 审批 ============

// CreateOffer 创建 Offer 审批单（状态 pending）。
func (s *ApplicationStore) CreateOffer(ctx context.Context, o *domain.Offer, applicationID, requestedBy string) (*domain.Offer, error) {
	err := s.pool.QueryRow(ctx, `
INSERT INTO offers (id, application_id, salary, join_date, note, status, requested_by)
VALUES (gen_random_uuid(), $1, $2, $3, $4, 'pending', $5)
RETURNING id, status, requested_at`,
		applicationID, o.Salary, o.JoinDate, o.Note, requestedBy).Scan(&o.ID, &o.Status, &o.RequestedAt)
	if err != nil {
		return nil, fmt.Errorf("repo: create offer: %w", err)
	}
	return o, nil
}

// GetPendingOfferByApplication 查询投递的待审批 Offer（无则 ErrNotFound）。
func (s *ApplicationStore) GetPendingOfferByApplication(ctx context.Context, applicationID string) (*domain.Offer, error) {
	var o domain.Offer
	var decidedAt *time.Time
	var requestedByName, decidedByName string
	err := s.pool.QueryRow(ctx, `
SELECT o.id, o.salary, o.join_date, o.note, o.status,
       u1.name, COALESCE(u2.name, ''), o.requested_at, o.decided_at
FROM offers o
JOIN users u1 ON u1.id = o.requested_by
LEFT JOIN users u2 ON u2.id = o.decided_by
WHERE o.application_id = $1 AND o.status = 'pending'
ORDER BY o.requested_at DESC LIMIT 1`,
		applicationID).Scan(&o.ID, &o.Salary, &o.JoinDate, &o.Note, &o.Status,
		&requestedByName, &decidedByName, &o.RequestedAt, &decidedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	o.RequestedByName = requestedByName
	o.DecidedByName = decidedByName
	o.DecidedAt = decidedAt
	return &o, nil
}

// GetLatestOfferByApplication 查询投递最近一次 Offer（任意状态；无则 ErrNotFound）。
func (s *ApplicationStore) GetLatestOfferByApplication(ctx context.Context, applicationID string) (*domain.Offer, error) {
	var o domain.Offer
	var decidedAt *time.Time
	var requestedByName, decidedByName string
	err := s.pool.QueryRow(ctx, `
SELECT o.id, o.salary, o.join_date, o.note, o.status,
       u1.name, COALESCE(u2.name, ''), o.requested_at, o.decided_at
FROM offers o
JOIN users u1 ON u1.id = o.requested_by
LEFT JOIN users u2 ON u2.id = o.decided_by
WHERE o.application_id = $1
ORDER BY o.requested_at DESC LIMIT 1`,
		applicationID).Scan(&o.ID, &o.Salary, &o.JoinDate, &o.Note, &o.Status,
		&requestedByName, &decidedByName, &o.RequestedAt, &decidedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	o.RequestedByName = requestedByName
	o.DecidedByName = decidedByName
	o.DecidedAt = decidedAt
	return &o, nil
}

// GetOfferRequestedBy 读取 Offer 发起人（四眼原则校验用）。
func (s *ApplicationStore) GetOfferRequestedBy(ctx context.Context, offerID string) (string, error) {
	var requestedBy string
	err := s.pool.QueryRow(ctx, `SELECT requested_by FROM offers WHERE id = $1`, offerID).Scan(&requestedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return requestedBy, nil
}

// DecideOffer 审批 Offer：通过（approved）/ 驳回（rejected）。
// salary：审批通过时确定的最终薪资（非空时覆盖建议薪资）。
func (s *ApplicationStore) DecideOffer(ctx context.Context, offerID, status, decidedBy, salary string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE offers SET
  status = $2,
  decided_by = $3,
  decided_at = now(),
  salary = CASE WHEN $4 <> '' THEN $4 ELSE salary END
WHERE id = $1 AND status = 'pending'`,
		offerID, status, decidedBy, salary)
	if err != nil {
		return fmt.Errorf("repo: decide offer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============ 流转时间线 ============

// InsertApplicationEvent 写入流转事件。
func (s *ApplicationStore) InsertApplicationEvent(ctx context.Context, applicationID, fromStage, toStage, action, actorID, actorName, reason string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO application_events (application_id, from_stage, to_stage, action, actor_id, actor_name, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		applicationID, fromStage, toStage, action, nullableString(actorID), actorName, reason)
	if err != nil {
		return fmt.Errorf("repo: insert application event: %w", err)
	}
	return nil
}

// ListApplicationEvents 查询流转时间线（时间正序）。
func (s *ApplicationStore) ListApplicationEvents(ctx context.Context, applicationID string) ([]domain.ApplicationEvent, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, from_stage, to_stage, action, COALESCE(actor_name, ''), reason, created_at
FROM application_events WHERE application_id = $1 ORDER BY created_at ASC, id ASC`, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ApplicationEvent, 0)
	for rows.Next() {
		var e domain.ApplicationEvent
		if err := rows.Scan(&e.ID, &e.FromStage, &e.ToStage, &e.Action, &e.ActorName, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListOfferPending 审批中心：Offer 待审批列表（deptID 非空时仅本部门，供 hiring_manager 隔离）。
func (s *ApplicationStore) ListOfferPending(ctx context.Context, deptID *string, page, pageSize int) ([]domain.ApprovalOfferItem, int, error) {
	where := ` WHERE a.stage = 'offer_pending' AND ($1::uuid IS NULL OR j.department_id = $1)`
	args := []any{nullableStringPtr(deptID)}

	var total int
	if err := s.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM applications a JOIN job_postings j ON j.id = a.job_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, `
SELECT a.id, a.job_id, j.title AS job_title,
       a.candidate_id, c.name AS candidate_name, c.email, c.phone,
       a.stage, a.source, a.submitted_at, a.match_score, a.hard_pass, a.parse_failed,
       a.parsed_resume, a.match_detail,
       (a.resume_file_key IS NOT NULL AND a.resume_file_key <> '') AS has_resume_file,
       a.reject_reason, a.interview_feedback,
       o.id, o.salary, o.join_date, o.note, o.status,
       COALESCE(u.name, ''), o.requested_at
FROM applications a
JOIN candidates c ON c.id = a.candidate_id
JOIN job_postings j ON j.id = a.job_id
JOIN offers o ON o.id = (
    SELECT id FROM offers WHERE application_id = a.id AND status = 'pending'
    ORDER BY requested_at DESC LIMIT 1)
LEFT JOIN users u ON u.id = o.requested_by`+where+`
ORDER BY o.requested_at ASC LIMIT $2 OFFSET $3`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]domain.ApprovalOfferItem, 0)
	for rows.Next() {
		var a domain.ApplicationInternal
		var o domain.Offer
		var parsed, detail []byte
		if err := rows.Scan(&a.ID, &a.JobID, &a.JobTitle,
			&a.CandidateID, &a.CandidateName, &a.Email, &a.Phone,
			&a.Stage, &a.Source, &a.SubmittedAt, &a.MatchScore, &a.HardPass, &a.ParseFailed,
			&parsed, &detail, &a.HasResumeFile, &a.RejectReason, &a.InterviewFeedback,
			&o.ID, &o.Salary, &o.JoinDate, &o.Note, &o.Status,
			&o.RequestedByName, &o.RequestedAt); err != nil {
			return nil, 0, err
		}
		fillApplicationJSON(&a, parsed, detail)
		out = append(out, domain.ApprovalOfferItem{Application: a, Offer: o})
	}
	return out, total, rows.Err()
}

func nullableStringPtr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// ListPendingApplications 待处理候选人列表（stage=new，按投递时间先进先出；
// deptID 非空时仅本部门，供 hiring_manager 隔离）。
func (s *ApplicationStore) ListPendingApplications(ctx context.Context, deptID *string, page, pageSize int) ([]domain.ApplicationInternal, int, error) {
	where := ` WHERE a.stage = 'new' AND ($1::uuid IS NULL OR j.department_id = $1)`
	args := []any{nullableStringPtr(deptID)}

	var total int
	if err := s.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM applications a JOIN job_postings j ON j.id = a.job_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, appInternalSelect+where+` ORDER BY a.submitted_at ASC LIMIT $2 OFFSET $3`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]domain.ApplicationInternal, 0)
	for rows.Next() {
		a, err := scanApplicationInternal(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	return out, total, rows.Err()
}

// ============ 候选人中心 ============

// CandidateListFilter 候选人中心筛选条件。
type CandidateListFilter struct {
	Stage        string
	DepartmentID *string
	JobID        string
	Q            string
	Sort         string // score_desc / newest
}

// ListCandidates 候选人中心：全局候选人列表（跨岗位，按阶段/部门/岗位/关键词筛选）。
func (s *ApplicationStore) ListCandidates(ctx context.Context, f CandidateListFilter, page, pageSize int) ([]domain.ApplicationInternal, int, error) {
	where := ` WHERE 1=1
  AND ($1::text = '' OR a.stage = $1)
  AND ($2::uuid IS NULL OR j.department_id = $2)
  AND ($3::text = '' OR a.job_id = $3::uuid)
  AND ($4::text = '' OR c.name ILIKE '%'||$4||'%' OR c.email ILIKE '%'||$4||'%')`
	args := []any{f.Stage, nullableStringPtr(f.DepartmentID), f.JobID, f.Q}

	var total int
	if err := s.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM applications a JOIN candidates c ON c.id = a.candidate_id JOIN job_postings j ON j.id = a.job_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := ` ORDER BY a.match_score DESC NULLS LAST, a.submitted_at DESC`
	if f.Sort == "newest" {
		orderBy = ` ORDER BY a.submitted_at DESC`
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, appInternalSelect+where+orderBy+` LIMIT $5 OFFSET $6`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]domain.ApplicationInternal, 0)
	for rows.Next() {
		a, err := scanApplicationInternal(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	s.attachInterviews(ctx, out)
	return out, total, nil
}

// ListActiveApplications 进行中（非终态）投递列表，供我的待办分类（含面试记录）。
func (s *ApplicationStore) ListActiveApplications(ctx context.Context, deptID *string) ([]domain.ApplicationInternal, error) {
	where := ` WHERE a.stage IN ('new','screening','interview','manager_interview') AND ($1::uuid IS NULL OR j.department_id = $1)`
	rows, err := s.pool.Query(ctx, appInternalSelect+where+` ORDER BY a.submitted_at ASC`, nullableStringPtr(deptID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ApplicationInternal, 0)
	for rows.Next() {
		a, err := scanApplicationInternal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.attachInterviews(ctx, out)
	return out, nil
}

// attachInterviews 批量附带面试记录（避免 N+1）。
func (s *ApplicationStore) attachInterviews(ctx context.Context, apps []domain.ApplicationInternal) {
	if len(apps) == 0 {
		return
	}
	ids := make([]string, 0, len(apps))
	for i := range apps {
		ids = append(ids, apps[i].ID)
	}
	rows, err := s.pool.Query(ctx, `
SELECT i.application_id, i.id, i.round, i.scheduled_at, i.status, i.result, i.feedback,
       COALESCE(u.name, ''), i.reviewed_at
FROM interviews i LEFT JOIN users u ON u.id = i.reviewed_by
WHERE i.application_id = ANY($1::uuid[]) ORDER BY i.round`, ids)
	if err != nil {
		return
	}
	defer rows.Close()
	byApp := map[string][]domain.Interview{}
	for rows.Next() {
		var appID string
		var iv domain.Interview
		if err := rows.Scan(&appID, &iv.ID, &iv.Round, &iv.ScheduledAt, &iv.Status, &iv.Result,
			&iv.Feedback, &iv.ReviewedByName, &iv.ReviewedAt); err != nil {
			return
		}
		byApp[appID] = append(byApp[appID], iv)
	}
	for i := range apps {
		if ivs, ok := byApp[apps[i].ID]; ok {
			apps[i].Interviews = ivs
		}
	}
}

// ============ 面试实体 ============

// UpsertInterview 安排/改期一轮面试（存在则更新时间为 scheduled）。
func (s *ApplicationStore) UpsertInterview(ctx context.Context, applicationID, round string, scheduledAt time.Time) (*domain.Interview, error) {
	var iv domain.Interview
	err := s.pool.QueryRow(ctx, `
INSERT INTO interviews (application_id, round, scheduled_at, status)
VALUES ($1, $2, $3, 'scheduled')
ON CONFLICT (application_id, round) DO UPDATE
  SET scheduled_at = $3, status = 'scheduled'
RETURNING id, round, scheduled_at, status, result, feedback`,
		applicationID, round, scheduledAt).Scan(&iv.ID, &iv.Round, &iv.ScheduledAt, &iv.Status, &iv.Result, &iv.Feedback)
	if err != nil {
		return nil, fmt.Errorf("repo: upsert interview: %w", err)
	}
	return &iv, nil
}

// GetInterviewByRound 查询某投递某轮面试（无则 ErrNotFound）。
func (s *ApplicationStore) GetInterviewByRound(ctx context.Context, applicationID, round string) (*domain.Interview, error) {
	var iv domain.Interview
	err := s.pool.QueryRow(ctx, `
SELECT id, round, scheduled_at, status, result, feedback
FROM interviews WHERE application_id = $1 AND round = $2`,
		applicationID, round).Scan(&iv.ID, &iv.Round, &iv.ScheduledAt, &iv.Status, &iv.Result, &iv.Feedback)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &iv, nil
}

// CompleteInterview 完成一轮面试：写入评价与结论。
func (s *ApplicationStore) CompleteInterview(ctx context.Context, applicationID, round, result, feedback, reviewedBy string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE interviews SET status='completed', result=$3, feedback=$4, reviewed_by=$5, reviewed_at=now()
WHERE application_id = $1 AND round = $2 AND status <> 'completed'`,
		applicationID, round, result, feedback, reviewedBy)
	if err != nil {
		return fmt.Errorf("repo: complete interview: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListInterviews 查询某投递的全部面试记录（含评价人姓名）。
func (s *ApplicationStore) ListInterviews(ctx context.Context, applicationID string) ([]domain.Interview, error) {
	rows, err := s.pool.Query(ctx, `
SELECT i.id, i.round, i.scheduled_at, i.status, i.result, i.feedback, COALESCE(u.name, ''), i.reviewed_at
FROM interviews i LEFT JOIN users u ON u.id = i.reviewed_by
WHERE i.application_id = $1 ORDER BY i.round`, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Interview, 0)
	for rows.Next() {
		var iv domain.Interview
		if err := rows.Scan(&iv.ID, &iv.Round, &iv.ScheduledAt, &iv.Status, &iv.Result,
			&iv.Feedback, &iv.ReviewedByName, &iv.ReviewedAt); err != nil {
			return nil, err
		}
		out = append(out, iv)
	}
	return out, rows.Err()
}

// UpdateResumeFileKey 投递创建后写入简历文件 key。
func (s *ApplicationStore) UpdateResumeFileKey(ctx context.Context, id, fileKey string) error {
	_, err := s.pool.Exec(ctx, `UPDATE applications SET resume_file_key = $2 WHERE id = $1`, id, fileKey)
	if err != nil {
		return fmt.Errorf("repo: update resume file key: %w", err)
	}
	return nil
}

// UpdateMatch 写回匹配结果。
func (s *ApplicationStore) UpdateMatch(ctx context.Context, id string, score float64, detail *domain.MatchDetail, hardPass bool, parsed *domain.ParsedResume, embedding []float32, parseFailed bool) error {
	var detailJSON, parsedJSON []byte
	var err error
	if detail != nil {
		detailJSON, err = json.Marshal(detail)
		if err != nil {
			return err
		}
	}
	if parsed != nil {
		parsedJSON, err = json.Marshal(parsed)
		if err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `
UPDATE applications SET
  match_score = $2, match_detail = $3, hard_pass = $4,
  parsed_resume = $5, resume_embedding = $6, parse_failed = $7
WHERE id = $1`,
		id, score, nullableJSON(detailJSON), hardPass, nullableJSON(parsedJSON), embedding, parseFailed)
	if err != nil {
		return fmt.Errorf("repo: update match: %w", err)
	}
	return nil
}

// SetParseFailed 标记解析失败。
func (s *ApplicationStore) SetParseFailed(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE applications SET parse_failed = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("repo: set parse failed: %w", err)
	}
	return nil
}

// ListIDsByJob 岗位下全部投递 id。
func (s *ApplicationStore) ListIDsByJob(ctx context.Context, jobID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM applications WHERE job_id = $1`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// GetProcessData 获取 worker 流水线所需数据。
func (s *ApplicationStore) GetProcessData(ctx context.Context, id string) (*ApplicationProcessData, error) {
	var d ApplicationProcessData
	var parsed []byte
	err := s.pool.QueryRow(ctx, `
SELECT id, job_id, COALESCE(resume_file_key,''), COALESCE(resume_text,''), parsed_resume, resume_embedding
FROM applications WHERE id = $1`, id).
		Scan(&d.ApplicationID, &d.JobID, &d.ResumeFileKey, &d.ResumeText, &parsed, &d.ResumeEmbedding)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(parsed) > 0 && string(parsed) != "null" {
		var p domain.ParsedResume
		if err := json.Unmarshal(parsed, &p); err == nil {
			d.ParsedResume = &p
		}
	}
	return &d, nil
}

// Stats 岗位投递漏斗统计。
func (s *ApplicationStore) Stats(ctx context.Context, jobID string) (*domain.JobStats, error) {
	st := &domain.JobStats{
		ByStage: map[string]int{
			"new": 0, "screening": 0, "interview": 0, "offer": 0, "hired": 0, "rejected": 0,
		},
	}
	var newC, screeningC, interviewC, offerC, hiredC, rejectedC int
	err := s.pool.QueryRow(ctx, `
SELECT COUNT(*),
       COUNT(*) FILTER (WHERE stage = 'new'),
       COUNT(*) FILTER (WHERE stage = 'screening'),
       COUNT(*) FILTER (WHERE stage = 'interview'),
       COUNT(*) FILTER (WHERE stage = 'offer'),
       COUNT(*) FILTER (WHERE stage = 'hired'),
       COUNT(*) FILTER (WHERE stage = 'rejected'),
       AVG(match_score) FILTER (WHERE match_score IS NOT NULL),
       COUNT(*) FILTER (WHERE hard_pass)
FROM applications WHERE job_id = $1`, jobID).
		Scan(&st.Total, &newC, &screeningC, &interviewC, &offerC, &hiredC, &rejectedC, &st.AvgScore, &st.HardPassCount)
	if err != nil {
		return nil, fmt.Errorf("repo: stats: %w", err)
	}
	st.ByStage["new"] = newC
	st.ByStage["screening"] = screeningC
	st.ByStage["interview"] = interviewC
	st.ByStage["offer"] = offerC
	st.ByStage["hired"] = hiredC
	st.ByStage["rejected"] = rejectedC
	return st, nil
}

// ListScoredByJob 已评分投递的规则分与语义分。
func (s *ApplicationStore) ListScoredByJob(ctx context.Context, jobID string) ([]ScoredApplication, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, match_detail FROM applications WHERE job_id = $1 AND match_detail IS NOT NULL`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ScoredApplication, 0)
	for rows.Next() {
		var id string
		var detail []byte
		if err := rows.Scan(&id, &detail); err != nil {
			return nil, err
		}
		var m domain.MatchDetail
		if err := json.Unmarshal(detail, &m); err != nil {
			continue
		}
		out = append(out, ScoredApplication{ID: id, RuleScore: m.RuleScore, SemanticScore: m.SemanticScore})
	}
	return out, rows.Err()
}

// ListPublicByCandidate 求职者本人的投递列表。
func (s *ApplicationStore) ListPublicByCandidate(ctx context.Context, candidateID string) ([]domain.ApplicationPublic, error) {
	rows, err := s.pool.Query(ctx, `
SELECT a.id, a.job_id, j.title AS job_title, a.stage, a.submitted_at
FROM applications a JOIN job_postings j ON j.id = a.job_id
WHERE a.candidate_id = $1 ORDER BY a.submitted_at DESC`, candidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ApplicationPublic, 0)
	for rows.Next() {
		var a domain.ApplicationPublic
		var stage string
		if err := rows.Scan(&a.ID, &a.JobID, &a.JobTitle, &stage, &a.SubmittedAt); err != nil {
			return nil, err
		}
		a.Status = domain.CandidateStatusFromStage(stage)
		out = append(out, a)
	}
	return out, rows.Err()
}

var _ ApplicationRepo = (*ApplicationStore)(nil)

// ============ 审计日志 ============

// AuditStore 审计日志仓库。
type AuditStore Postgres

// Insert 写入审计日志。
func (s *AuditStore) Insert(ctx context.Context, log *AuditLog) error {
	var detailJSON []byte
	var err error
	if log.Detail != nil {
		detailJSON, err = json.Marshal(log.Detail)
		if err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, detail)
VALUES ($1,$2,$3,$4,$5)`,
		log.ActorID, log.Action, log.EntityType, log.EntityID, nullableJSON(detailJSON))
	if err != nil {
		return fmt.Errorf("repo: insert audit log: %w", err)
	}
	return nil
}

var _ AuditRepo = (*AuditStore)(nil)

// nullableString 空字符串转为 NULL。
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableJSON 空字节转为 NULL。
func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
