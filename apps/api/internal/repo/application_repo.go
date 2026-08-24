package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
       (a.resume_file_key IS NOT NULL AND a.resume_file_key <> '') AS has_resume_file
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
       COALESCE(a.resume_text, '')
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
		&parsed, &detail, &a.HasResumeFile)
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
		&parsed, &detail, &a.HasResumeFile, &a.ResumeText)
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

// UpdateStage 更新流转阶段。
func (s *ApplicationStore) UpdateStage(ctx context.Context, id, stage string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE applications SET stage = $2 WHERE id = $1`, id, stage)
	if err != nil {
		return fmt.Errorf("repo: update stage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
