// Package service 业务逻辑层。
//
// 服务层接收 actor（当前用户）做权限判断与部门隔离（hiring_manager 只能访问
// 本部门岗位与投递，服务端强制校验，不依赖前端），所有写操作写入审计日志。
package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/auth"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/redis/go-redis/v9"
)

// Actor 当前登录用户（内部端）。
type Actor struct {
	UserID string
	Role   string
	DepID  *string
}

// NewActorFromClaims 从 JWT Claims 构造 Actor。
func NewActorFromClaims(c *auth.Claims) *Actor {
	return &Actor{UserID: c.Subject, Role: c.Role, DepID: c.DepID}
}

// isAdmin 是否管理员。
func (a *Actor) isAdmin() bool { return a.Role == string(domain.RoleAdmin) }

// isHR 是否 HR。
func (a *Actor) isHR() bool { return a.Role == string(domain.RoleHR) }

// isHiringManager 是否部门负责人。
func (a *Actor) isHiringManager() bool { return a.Role == string(domain.RoleHiringManager) }

// canAccessJob 岗位访问权限：admin/hr 全量；hiring_manager 仅本部门。
func (a *Actor) canAccessJob(j *domain.JobPosting) bool {
	if a.isAdmin() || a.isHR() {
		return true
	}
	return a.DepID != nil && j.DepartmentID != "" && *a.DepID == j.DepartmentID
}

// ============ 认证服务 ============

// AuthService 登录 / 邮箱验证码。
type AuthService struct {
	Users       repo.UserRepo
	Candidates  repo.CandidateRepo
	Applications repo.ApplicationRepo
	JWTSecret   string
	JWTTTL      time.Duration
	Redis       *redis.Client
}

// NewAuthService 构造认证服务。
func NewAuthService(users repo.UserRepo, candidates repo.CandidateRepo, apps repo.ApplicationRepo, secret string, ttl time.Duration, rdb *redis.Client) *AuthService {
	return &AuthService{Users: users, Candidates: candidates, Applications: apps, JWTSecret: secret, JWTTTL: ttl, Redis: rdb}
}

// Login 内部端登录：校验邮箱密码，签发 JWT（aud=internal）。
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.LoginResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, domain.NewError(400, domain.CodeBadRequest, "请输入邮箱与密码")
	}
	user, err := s.Users.GetByEmail(ctx, email)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, domain.NewError(401, domain.CodeUnauthorized, "邮箱或密码错误")
		}
		return nil, domain.WrapError(500, domain.CodeInternal, "查询用户失败", err)
	}
	// 需要 password_hash：User 模型不带哈希，单独查询。
	ok, err := s.verifyPassword(ctx, user.ID, password)
	if err != nil || !ok {
		return nil, domain.NewError(401, domain.CodeUnauthorized, "邮箱或密码错误")
	}
	token, err := auth.Sign(s.JWTSecret, s.JWTTTL, user.ID, user.Role, user.DepartmentID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "签发令牌失败", err)
	}
	return &domain.LoginResponse{Token: token, User: *user}, nil
}

// verifyPassword 按用户 id 读取密码哈希并校验。
func (s *AuthService) verifyPassword(ctx context.Context, userID, password string) (bool, error) {
	hash, err := s.passwordHash(ctx, userID)
	if err != nil {
		return false, err
	}
	return auth.VerifyPassword(password, hash)
}

// passwordHash 读取用户密码哈希。
func (s *AuthService) passwordHash(ctx context.Context, userID string) (string, error) {
	hash, err := s.Users.GetPasswordHash(ctx, userID)
	if err != nil {
		if err == repo.ErrNotFound {
			return "", nil
		}
		return "", domain.WrapError(500, domain.CodeInternal, "查询用户失败", err)
	}
	return hash, nil
}

// SendEmailCode 生成 6 位验证码存 Redis（5 分钟 TTL），60 秒内重复请求拒绝。
// 当前不接 SMTP，用 slog 打印验证码（生产可接邮件服务）。
func (s *AuthService) SendEmailCode(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return domain.NewError(400, domain.CodeBadRequest, "邮箱格式不正确")
	}
	// 60 秒限流
	rateKey := "auth:rate:" + email
	ok, err := s.Redis.SetNX(ctx, rateKey, "1", 60*time.Second).Result()
	if err != nil {
		return domain.WrapError(500, domain.CodeInternal, "验证码服务异常", err)
	}
	if !ok {
		return domain.NewError(429, domain.CodeRateLimited, "请求过于频繁，请 60 秒后再试")
	}

	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	if err := s.Redis.Set(ctx, "auth:code:"+email, code, 5*time.Minute).Err(); err != nil {
		return domain.WrapError(500, domain.CodeInternal, "验证码存储失败", err)
	}
	// 未接入 SMTP：打印验证码到日志
	slog.Info("email code generated", "email", email, "code", code, "ttl", "5m")
	return nil
}

// VerifyEmailCode 校验验证码，成功签发候选人 JWT（aud=candidate, sub=candidate_id）。
func (s *AuthService) VerifyEmailCode(ctx context.Context, email, code string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return "", domain.NewError(400, domain.CodeBadRequest, "请提供邮箱与验证码")
	}
	stored, err := s.Redis.Get(ctx, "auth:code:"+email).Result()
	if err == redis.Nil {
		return "", domain.NewError(400, domain.CodeBadRequest, "验证码不存在或已过期，请重新获取")
	}
	if err != nil {
		return "", domain.WrapError(500, domain.CodeInternal, "验证码服务异常", err)
	}
	if stored != code {
		return "", domain.NewError(400, domain.CodeBadRequest, "验证码不正确")
	}
	_ = s.Redis.Del(ctx, "auth:code:"+email)

	candidate, err := s.Candidates.GetByEmail(ctx, email)
	if err != nil {
		if err == repo.ErrNotFound {
			return "", domain.NewError(404, domain.CodeNotFound, "未找到该邮箱的投递记录")
		}
		return "", domain.WrapError(500, domain.CodeInternal, "查询候选人失败", err)
	}
	token, err := auth.SignCandidate(s.JWTSecret, s.JWTTTL, candidate.ID)
	if err != nil {
		return "", domain.WrapError(500, domain.CodeInternal, "签发令牌失败", err)
	}
	return token, nil
}

// MyApplications 候选人查看本人投递列表。
func (s *AuthService) MyApplications(ctx context.Context, candidateID string) ([]domain.ApplicationPublic, error) {
	apps, err := s.Applications.ListPublicByCandidate(ctx, candidateID)
	if err != nil {
		return nil, domain.WrapError(500, domain.CodeInternal, "查询投递记录失败", err)
	}
	return apps, nil
}

// ============ 部门服务 ============

// DepartmentService 部门查询。
type DepartmentService struct {
	Departments repo.DepartmentRepo
}

// NewDepartmentService 构造部门服务。
func NewDepartmentService(d repo.DepartmentRepo) *DepartmentService {
	return &DepartmentService{Departments: d}
}

// List 列出全部部门。
func (s *DepartmentService) List(ctx context.Context) ([]domain.Department, error) {
	return s.Departments.List(ctx)
}
