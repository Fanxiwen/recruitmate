// Package main 种子数据命令：创建演示部门/用户/岗位/候选人，
// 并同步运行规则引擎写入 match_score（engine='rule'），无 AI Key 也能看到排序效果；
// 若配置了 AI Key 则额外入队 resume:process 全量流水线。
//
// 幂等：检测到已存在 admin@recruitmate.local 时直接退出。
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/ai"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/auth"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/config"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/db"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/matching"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/worker"
	"github.com/google/uuid"
)

// seedPassword 演示账号统一密码。
const seedPassword = "Recruitmate1!"

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if err := db.RunMigrations(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	pg, err := repo.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pg.Close()
	repos := pg.NewRepos()

	// 幂等检查
	if _, err := repos.Users.GetByEmail(ctx, "admin@recruitmate.local"); err == nil {
		slog.Info("seed skipped: already seeded")
		return nil
	}

	// ============ 部门 ============
	techDeptID := upsertDepartment(ctx, pg, "技术部")
	productDeptID := upsertDepartment(ctx, pg, "产品部")
	marketDeptID := upsertDepartment(ctx, pg, "市场部")
	slog.Info("departments ready", "tech", techDeptID, "product", productDeptID, "market", marketDeptID)

	// ============ 用户 ============
	adminID := createUser(ctx, pg, "admin@recruitmate.local", "系统管理员", "admin", nil)
	hrID := createUser(ctx, pg, "hr@recruitmate.local", "人力资源-张敏", "hr", nil)
	managerID := createUser(ctx, pg, "manager@tech.recruitmate.local", "技术部负责人-刘洋", "hiring_manager", &techDeptID)
	slog.Info("users ready", "admin", adminID, "hr", hrID, "manager", managerID)

	// ============ 岗位 ============
	// 后端工程师（Go）—— 技术部，open
	backendJob := &domain.JobPosting{
		ID:           uuid.NewString(),
		Title:        "后端工程师（Go）",
		DepartmentID: techDeptID,
		OwnerID:      hrID,
		Status:       string(domain.JobStatusOpen),
		Headcount:    2,
		Location:     "上海",
		JobType:      string(domain.JobTypeFullTime),
		SalaryMin:    intPtr(25),
		SalaryMax:    intPtr(40),
		Description:  "负责核心业务系统后端研发，主导微服务架构设计与性能优化，参与技术方案评审与代码审查。",
		Requirements: domain.JobRequirements{
			MustSkills:   []string{"Go", "PostgreSQL", "Redis"},
			NiceSkills:   []string{"Kubernetes", "微服务"},
			MinEducation: string(domain.EducationBachelor),
			MinYears:     3,
		},
	}
	// 产品经理 —— 产品部，open
	pmJob := &domain.JobPosting{
		ID:           uuid.NewString(),
		Title:        "产品经理",
		DepartmentID: productDeptID,
		OwnerID:      hrID,
		Status:       string(domain.JobStatusOpen),
		Headcount:    1,
		Location:     "北京",
		JobType:      string(domain.JobTypeFullTime),
		SalaryMin:    intPtr(18),
		SalaryMax:    intPtr(30),
		Description:  "负责 B 端产品规划与迭代，挖掘客户需求，输出 PRD 并推动跨团队落地。",
		Requirements: domain.JobRequirements{
			MustSkills:   []string{"产品设计", "需求分析"},
			NiceSkills:   []string{"数据分析", "项目管理"},
			MinEducation: string(domain.EducationBachelor),
			MinYears:     2,
		},
	}
	// 前端工程师 —— 技术部，open
	frontendJob := &domain.JobPosting{
		ID:           uuid.NewString(),
		Title:        "前端工程师",
		DepartmentID: techDeptID,
		OwnerID:      hrID,
		Status:       string(domain.JobStatusOpen),
		Headcount:    2,
		Location:     "深圳",
		JobType:      string(domain.JobTypeFullTime),
		SalaryMin:    intPtr(20),
		SalaryMax:    intPtr(35),
		Description:  "负责 Web 前端开发，基于 React/TypeScript 构建高质量中后台系统。",
		Requirements: domain.JobRequirements{
			MustSkills:   []string{"React", "TypeScript"},
			NiceSkills:   []string{"Node.js", "TailwindCSS"},
			MinEducation: string(domain.EducationBachelor),
			MinYears:     2,
		},
	}
	// 市场运营专员 —— 市场部，draft（未发布）
	draftJob := &domain.JobPosting{
		ID:           uuid.NewString(),
		Title:        "市场运营专员",
		DepartmentID: marketDeptID,
		OwnerID:      hrID,
		Status:       string(domain.JobStatusDraft),
		Headcount:    1,
		Location:     "广州",
		JobType:      string(domain.JobTypeFullTime),
		Description:  "负责品牌活动策划与新媒体运营（草稿，待完善）。",
		Requirements: domain.JobRequirements{
			MustSkills:   []string{"内容运营"},
			NiceSkills:   []string{},
			MinEducation: string(domain.EducationAny),
			MinYears:     1,
		},
	}
	backendID := createJob(ctx, repos, backendJob)
	pmID := createJob(ctx, repos, pmJob)
	frontendID := createJob(ctx, repos, frontendJob)
	createJob(ctx, repos, draftJob)
	slog.Info("jobs ready", "backend", backendID, "pm", pmID, "frontend", frontendID)

	// ============ 演示候选人 ============
	type demoApp struct {
		candidate  domain.Candidate
		resumeText string
		parsed     domain.ParsedResume
		jobID      string
	}
	apps := []demoApp{
		{
			// 陈伟：资深 Go 后端，全满足
			candidate:  domain.Candidate{Email: "chenwei@example.com", Name: "陈伟", Phone: "13800000001"},
			resumeText: chenweiResume,
			parsed:     chenweiParsed,
			jobID:      backendID,
		},
		{
			// 李娜：产品经理，满足产品岗；投后端则缺必备技能（hard_pass）
			candidate:  domain.Candidate{Email: "lina@example.com", Name: "李娜", Phone: "13800000002"},
			resumeText: linaResume,
			parsed:     linaParsed,
			jobID:      pmID,
		},
		{
			candidate:  domain.Candidate{Email: "lina@example.com", Name: "李娜", Phone: "13800000002"},
			resumeText: linaResume,
			parsed:     linaParsed,
			jobID:      backendID,
		},
		{
			// 王小明：应届前端，投前端缺年限（hard_pass）
			candidate:  domain.Candidate{Email: "wangxiaoming@example.com", Name: "王小明", Phone: "13800000003"},
			resumeText: wangResume,
			parsed:     wangParsed,
			jobID:      frontendID,
		},
		{
			// 王小明：应届投后端 3 年岗（hard_pass，展示排序底部）
			candidate:  domain.Candidate{Email: "wangxiaoming@example.com", Name: "王小明", Phone: "13800000003"},
			resumeText: wangResume,
			parsed:     wangParsed,
			jobID:      backendID,
		},
	}

	queue := worker.NewQueue(cfg.RedisAddr)
	defer queue.Close()
	aiClient, err := ai.New(ctx, ai.Config{
		DeepSeekAPIKey:     cfg.DeepSeekAPIKey,
		DeepSeekBaseURL:    cfg.DeepSeekBaseURL,
		DeepSeekModel:      cfg.DeepSeekModel,
		SiliconFlowAPIKey:  cfg.SiliconFlowAPIKey,
		SiliconFlowBaseURL: cfg.SiliconFlowBaseURL,
		EmbeddingModel:     cfg.EmbeddingModel,
		EmbeddingDim:       cfg.EmbeddingDim,
	})
	if err != nil {
		return err
	}
	chatOK, embedOK := ai.Capabilities(aiClient)

	for _, a := range apps {
		// 查/建候选人（邮箱小写）
		cand, err := repos.Candidates.GetOrCreateByEmail(ctx, a.candidate.Email, a.candidate.Name, a.candidate.Phone)
		if err != nil {
			return err
		}
		// 已存在投递则跳过
		if _, err := repos.Applications.GetByCandidateAndJob(ctx, cand.ID, a.jobID); err == nil {
			continue
		}
		appID := uuid.NewString()
		app := &domain.Application{
			ID:          appID,
			CandidateID: cand.ID,
			JobID:       a.jobID,
			Stage:       string(domain.StageNew),
			Source:      "seed",
			ResumeText:  a.resumeText,
		}
		if err := repos.Applications.Create(ctx, app); err != nil {
			return err
		}

		// 同步运行规则引擎（engine='rule'）
		job, err := repos.Jobs.Get(ctx, a.jobID)
		if err != nil {
			return err
		}
		rule := matching.Evaluate(job.Requirements, a.parsed, a.resumeText)
		composite := matching.CompositeRule(rule.RuleScore, nil)
		detail := &domain.MatchDetail{
			Score:         composite,
			RuleScore:     rule.RuleScore,
			SemanticScore: nil,
			LLMScore:      nil,
			Strengths:     []string{},
			Gaps:          []string{},
			HardChecks:    rule.HardChecks,
			Engine:        "rule",
			ScoredAt:      time.Now().UTC(),
		}
		if err := repos.Applications.UpdateMatch(ctx, appID, float64(composite), detail, rule.HardPass, &a.parsed, nil, false); err != nil {
			return err
		}
		slog.Info("seed application scored",
			"candidate", a.candidate.Email, "job", job.Title,
			"score", composite, "hard_pass", rule.HardPass)

		// 若配置了 AI Key，入队完整流水线（LLM 解析 + 向量 + 评委）
		if chatOK || embedOK {
			if err := queue.EnqueueResumeProcess(ctx, appID, job.UpdatedAt.Unix()); err != nil {
				slog.Error("seed enqueue resume:process failed", "application_id", appID, "error", err)
			}
		}
	}

	slog.Info("seed done", "chat_enabled", chatOK, "embed_enabled", embedOK)
	return nil
}

// upsertDepartment 按名称查/建部门，返回 id。
func upsertDepartment(ctx context.Context, pg *repo.Postgres, name string) string {
	var id string
	err := pg.Pool().QueryRow(ctx, `
INSERT INTO departments (id, name) VALUES ($1, $2)
ON CONFLICT (name) DO NOTHING
RETURNING id`, uuid.NewString(), name).Scan(&id)
	if err != nil {
		_ = pg.Pool().QueryRow(ctx, `SELECT id FROM departments WHERE name = $1`, name).Scan(&id)
	}
	return id
}

// createUser 创建内部用户（argon2 哈希），返回 id。
func createUser(ctx context.Context, pg *repo.Postgres, email, name, role string, deptID *string) string {
	hash, err := auth.HashPassword(seedPassword)
	if err != nil {
		panic(err)
	}
	var id string
	err = pg.Pool().QueryRow(ctx, `
INSERT INTO users (id, email, name, password_hash, role, department_id)
VALUES ($1,$2,$3,$4,$5,$6)
RETURNING id`, uuid.NewString(), email, name, hash, role, deptID).Scan(&id)
	if err != nil {
		panic(err)
	}
	return id
}

// createJob 创建岗位，返回 id。
func createJob(ctx context.Context, repos *repo.Repos, job *domain.JobPosting) string {
	if err := repos.Jobs.Create(ctx, job); err != nil {
		panic(err)
	}
	return job.ID
}

func intPtr(n int) *int { return &n }

// ============ 演示简历文本与解析结果 ============

const chenweiResume = `陈伟
电话：13800000001 | 邮箱：chenwei@example.com | 上海

求职意向：资深后端工程师（Go）

教育背景
2013.09 - 2017.06  上海交通大学  计算机科学与技术  本科

工作经历
2019.03 - 至今  某知名互联网公司  高级后端工程师
- 负责订单中台核心服务研发，基于 Go 与 PostgreSQL 构建高并发订单系统（QPS 1w+）
- 使用 Redis 设计分布式缓存与限流方案，缓存命中率达 99%
- 主导微服务拆分与 Kubernetes 容器化改造，服务可用性提升至 99.99%

2017.07 - 2019.02  某创业公司  后端工程师
- 使用 Go 开发 REST API 服务，支撑百万级用户产品

技能
Go、PostgreSQL、Redis、Kubernetes、微服务、Docker、gRPC、消息队列

自我评价
5 年 Go 后端研发经验，熟悉高并发系统设计与微服务架构，具备良好的工程化与团队协作能力。`

var chenweiParsed = domain.ParsedResume{
	Name:              "陈伟",
	Email:             "chenwei@example.com",
	Phone:             "13800000001",
	YearsOfExperience: 5,
	Education: []domain.EducationItem{
		{Level: "本科", School: "上海交通大学", Major: "计算机科学与技术"},
	},
	Skills: []string{"Go", "PostgreSQL", "Redis", "Kubernetes", "微服务", "Docker", "gRPC", "消息队列"},
	WorkExperience: []domain.WorkExperienceItem{
		{Company: "某知名互联网公司", Title: "高级后端工程师", StartDate: strPtr("2019-03")},
		{Company: "某创业公司", Title: "后端工程师", StartDate: strPtr("2017-07"), EndDate: strPtr("2019-02")},
	},
	Summary: "5 年 Go 后端研发经验，熟悉高并发系统设计与微服务架构。",
}

const linaResume = `李娜
电话：13800000002 | 邮箱：lina@example.com | 北京

求职意向：产品经理

教育背景
2014.09 - 2018.06  中国人民大学  市场营销  本科

工作经历
2020.06 - 至今  某 SaaS 公司  高级产品经理
- 负责 B 端客户成功平台产品规划，通过需求分析输出 PRD 并推动研发落地
- 设计数据看板功能，付费转化率提升 20%

2018.07 - 2020.05  某互联网公司  产品助理
- 协助产品设计，参与用户调研与竞品分析

技能
产品设计、需求分析、用户调研、数据分析、Axure、项目管理

自我评价
3 年 B 端产品经验，擅长需求分析与产品设计，数据驱动决策。`

var linaParsed = domain.ParsedResume{
	Name:              "李娜",
	Email:             "lina@example.com",
	Phone:             "13800000002",
	YearsOfExperience: 3,
	Education: []domain.EducationItem{
		{Level: "本科", School: "中国人民大学", Major: "市场营销"},
	},
	Skills: []string{"产品设计", "需求分析", "用户调研", "数据分析", "Axure", "项目管理"},
	WorkExperience: []domain.WorkExperienceItem{
		{Company: "某 SaaS 公司", Title: "高级产品经理", StartDate: strPtr("2020-06")},
		{Company: "某互联网公司", Title: "产品助理", StartDate: strPtr("2018-07"), EndDate: strPtr("2020-05")},
	},
	Summary: "3 年 B 端产品经验，擅长需求分析与产品设计。",
}

const wangResume = `王小明
电话：13800000003 | 邮箱：wangxiaoming@example.com | 深圳

求职意向：前端工程师

教育背景
2020.09 - 2024.06  深圳大学  软件工程  本科

项目经历
2023.06 - 2023.09  校园二手交易平台  前端开发实习生
- 使用 React + TypeScript 开发前端页面，负责商品列表与详情模块
- 使用 TailwindCSS 实现响应式布局

2022.09 - 2023.05  课程设计项目 团队成员
- 基于 Node.js 搭建简易后端服务

技能
React、TypeScript、JavaScript、TailwindCSS、Node.js、HTML/CSS

自我评价
2024 届应届毕业生，热爱前端开发，学习能力强，希望在实践中快速成长。`

var wangParsed = domain.ParsedResume{
	Name:              "王小明",
	Email:             "wangxiaoming@example.com",
	Phone:             "13800000003",
	YearsOfExperience: 0,
	Education: []domain.EducationItem{
		{Level: "本科", School: "深圳大学", Major: "软件工程"},
	},
	Skills: []string{"React", "TypeScript", "JavaScript", "TailwindCSS", "Node.js"},
	WorkExperience: []domain.WorkExperienceItem{
		{Company: "校园二手交易平台", Title: "前端开发实习生", StartDate: strPtr("2023-06"), EndDate: strPtr("2023-09")},
	},
	Summary: "2024 届应届毕业生，熟悉 React/TypeScript 前端开发。",
}

func strPtr(s string) *string { return &s }
