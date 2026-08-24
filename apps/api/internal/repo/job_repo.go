package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// JobStore 岗位仓库。
type JobStore Postgres

const jobSelect = `
SELECT j.id, j.title, j.department_id, d.name AS department_name,
       j.owner_id, u.name AS owner_name,
       j.status, j.headcount, j.salary_min, j.salary_max, j.location, j.job_type,
       j.description, j.requirements,
       j.published_at, j.closed_at, j.created_at, j.updated_at
FROM job_postings j
LEFT JOIN departments d ON d.id = j.department_id
LEFT JOIN users u ON u.id = j.owner_id`

// jobSelectWithCount 内部端列表：附带投递总数与已入职人数。
const jobSelectWithCount = `
SELECT j.id, j.title, j.department_id, d.name AS department_name,
       j.owner_id, u.name AS owner_name,
       j.status, j.headcount, j.salary_min, j.salary_max, j.location, j.job_type,
       j.description, j.requirements,
       j.published_at, j.closed_at, j.created_at, j.updated_at,
       (SELECT COUNT(*) FROM applications a WHERE a.job_id = j.id) AS application_count,
       (SELECT COUNT(*) FROM applications a WHERE a.job_id = j.id AND a.stage = 'hired') AS hired_count
FROM job_postings j
LEFT JOIN departments d ON d.id = j.department_id
LEFT JOIN users u ON u.id = j.owner_id`

func scanJob(row pgx.Row) (*domain.JobPosting, error) {
	var j domain.JobPosting
	var requirements []byte
	err := row.Scan(&j.ID, &j.Title, &j.DepartmentID, &j.DepartmentName,
		&j.OwnerID, &j.OwnerName,
		&j.Status, &j.Headcount, &j.SalaryMin, &j.SalaryMax, &j.Location, &j.JobType,
		&j.Description, &requirements,
		&j.PublishedAt, &j.ClosedAt, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(requirements) > 0 {
		if err := json.Unmarshal(requirements, &j.Requirements); err != nil {
			return nil, fmt.Errorf("repo: unmarshal requirements: %w", err)
		}
	}
	return &j, nil
}

// scanJobWithCount 同 scanJob，额外扫描投递总数与已入职人数。
func scanJobWithCount(row pgx.Row) (*domain.JobPosting, error) {
	var j domain.JobPosting
	var requirements []byte
	var count, hired *int
	err := row.Scan(&j.ID, &j.Title, &j.DepartmentID, &j.DepartmentName,
		&j.OwnerID, &j.OwnerName,
		&j.Status, &j.Headcount, &j.SalaryMin, &j.SalaryMax, &j.Location, &j.JobType,
		&j.Description, &requirements,
		&j.PublishedAt, &j.ClosedAt, &j.CreatedAt, &j.UpdatedAt, &count, &hired)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(requirements) > 0 {
		if err := json.Unmarshal(requirements, &j.Requirements); err != nil {
			return nil, fmt.Errorf("repo: unmarshal requirements: %w", err)
		}
	}
	j.ApplicationCount = count
	j.HiredCount = hired
	return &j, nil
}

// Create 创建岗位（status=draft）。
func (s *JobStore) Create(ctx context.Context, j *domain.JobPosting) error {
	reqJSON, err := json.Marshal(j.Requirements)
	if err != nil {
		return err
	}
	err = s.pool.QueryRow(ctx, `
INSERT INTO job_postings (id, title, department_id, owner_id, status, headcount, salary_min, salary_max,
                          location, job_type, description, requirements)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING created_at, updated_at`,
		j.ID, j.Title, j.DepartmentID, j.OwnerID, j.Status, j.Headcount,
		j.SalaryMin, j.SalaryMax, j.Location, j.JobType, j.Description, reqJSON,
	).Scan(&j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return fmt.Errorf("repo: create job: %w", err)
	}
	return nil
}

// Update 更新岗位内容并刷新 updated_at。
func (s *JobStore) Update(ctx context.Context, j *domain.JobPosting) error {
	reqJSON, err := json.Marshal(j.Requirements)
	if err != nil {
		return err
	}
	err = s.pool.QueryRow(ctx, `
UPDATE job_postings SET
  title=$2, department_id=$3, headcount=$4, salary_min=$5, salary_max=$6,
  location=$7, job_type=$8, description=$9, requirements=$10, updated_at=now()
WHERE id=$1
RETURNING updated_at`,
		j.ID, j.Title, j.DepartmentID, j.Headcount, j.SalaryMin, j.SalaryMax,
		j.Location, j.JobType, j.Description, reqJSON,
	).Scan(&j.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("repo: update job: %w", err)
	}
	return nil
}

// Get 按 id 查询岗位（含部门名/负责人名）。
func (s *JobStore) Get(ctx context.Context, id string) (*domain.JobPosting, error) {
	return scanJob(s.pool.QueryRow(ctx, jobSelect+` WHERE j.id = $1`, id))
}

// ListPublic 公开岗位列表（仅 open，按 published_at desc）。
func (s *JobStore) ListPublic(ctx context.Context, f JobListFilter, page, pageSize int) ([]domain.JobPosting, int, error) {
	where := ` WHERE j.status = 'open'`
	args := []any{}
	args = append(args, f.Q, f.DepartmentID, f.JobType)
	where += ` AND ($1::text = '' OR j.title ILIKE '%'||$1||'%' OR COALESCE(j.description,'') ILIKE '%'||$1||'%')
	           AND ($2::uuid IS NULL OR j.department_id = $2)
	           AND ($3::text = '' OR j.job_type = $3)`

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM job_postings j`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, jobSelect+where+` ORDER BY j.published_at DESC NULLS LAST LIMIT $4 OFFSET $5`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// 注意：必须用 make 初始化，空结果序列化为 [] 而非 null（前端契约）
	out := make([]domain.JobPosting, 0)
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *j)
	}
	return out, total, rows.Err()
}

// ListInternal 内部岗位列表（可按 status 过滤；hiring_manager 限定部门）。
func (s *JobStore) ListInternal(ctx context.Context, status string, deptID *string, page, pageSize int) ([]domain.JobPosting, int, error) {
	where := ` WHERE 1=1`
	args := []any{}
	args = append(args, status, deptID)
	where += ` AND ($1::text = '' OR j.status = $1)
	           AND ($2::uuid IS NULL OR j.department_id = $2)`

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM job_postings j`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, jobSelectWithCount+where+` ORDER BY j.created_at DESC LIMIT $3 OFFSET $4`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// 注意：必须用 make 初始化，空结果序列化为 [] 而非 null（前端契约）
	out := make([]domain.JobPosting, 0)
	for rows.Next() {
		j, err := scanJobWithCount(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *j)
	}
	return out, total, rows.Err()
}

// SetStatus 岗位状态流转。
func (s *JobStore) SetStatus(ctx context.Context, id, status string, approverID *string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE job_postings SET
  status = $2,
  approver_id = CASE WHEN $3::uuid IS NOT NULL THEN $3 ELSE approver_id END,
  published_at = CASE WHEN $2 = 'open' THEN COALESCE(published_at, now()) ELSE published_at END,
  closed_at = CASE WHEN $2 = 'closed' THEN now() WHEN $2 = 'open' THEN NULL ELSE closed_at END,
  updated_at = now()
WHERE id = $1`, id, status, approverID)
	if err != nil {
		return fmt.Errorf("repo: set job status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetEmbedding 写入岗位 embedding（向量缓存），并刷新 updated_at 使版本号变化。
func (s *JobStore) SetEmbedding(ctx context.Context, id string, vec []float32) error {
	_, err := s.pool.Exec(ctx, `UPDATE job_postings SET embedding = $2, updated_at = now() WHERE id = $1`, id, vec)
	if err != nil {
		return fmt.Errorf("repo: set job embedding: %w", err)
	}
	return nil
}

// GetEmbedding 读取岗位 embedding；未生成返回 nil。
func (s *JobStore) GetEmbedding(ctx context.Context, id string) ([]float32, error) {
	var vec []float32
	err := s.pool.QueryRow(ctx, `SELECT embedding FROM job_postings WHERE id = $1`, id).Scan(&vec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return vec, nil
}

// ensure JobStore 实现 JobRepo 接口。
var _ JobRepo = (*JobStore)(nil)
