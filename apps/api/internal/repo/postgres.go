package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

// ErrNotFound 记录不存在。
var ErrNotFound = errors.New("repo: not found")

// Postgres 基于 pgxpool 的仓库实现。
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres 创建连接池并注册 pgvector 类型。
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("repo: parse dsn: %w", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("repo: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("repo: ping: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close 关闭连接池。
func (p *Postgres) Close() { p.pool.Close() }

// Pool 暴露底层连接池（goose 迁移等场景使用）。
func (p *Postgres) Pool() *pgxpool.Pool { return p.pool }

// NewRepos 组装全部仓库。
func (p *Postgres) NewRepos() *Repos {
	return &Repos{
		Users:        (*UserStore)(p),
		Departments:  (*DepartmentStore)(p),
		Jobs:         (*JobStore)(p),
		Candidates:   (*CandidateStore)(p),
		Applications: (*ApplicationStore)(p),
		Audit:        (*AuditStore)(p),
	}
}

// ============ 用户 ============

// UserStore 用户仓库。
type UserStore Postgres

const userSelect = `
SELECT u.id, u.email, u.name, u.role, u.department_id, d.name AS department_name
FROM users u LEFT JOIN departments d ON d.id = u.department_id`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.DepartmentID, &u.DepartmentName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// GetByEmail 按邮箱查询用户。
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(s.pool.QueryRow(ctx, userSelect+` WHERE u.email = $1`, email))
}

// GetByID 按 id 查询用户。
func (s *UserStore) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return scanUser(s.pool.QueryRow(ctx, userSelect+` WHERE u.id = $1`, id))
}

// ============ 部门 ============

// DepartmentStore 部门仓库。
type DepartmentStore Postgres

// List 列出全部部门。
func (s *DepartmentStore) List(ctx context.Context) ([]domain.Department, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM departments ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Department
	for rows.Next() {
		var d domain.Department
		if err := rows.Scan(&d.ID, &d.Name); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
