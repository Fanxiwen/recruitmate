// Package domain 定义领域模型与错误类型。
//
// 所有 JSON 字段名必须与 packages/shared-types/src/index.ts 完全一致（camelCase），
// 这是前后端 JSON 契约的唯一事实来源。
package domain

import (
	"time"
)

// ============ 枚举（与 shared-types 一致） ============

// Role 内部用户角色。
type Role string

const (
	RoleAdmin         Role = "admin"
	RoleHR            Role = "hr"
	RoleHiringManager Role = "hiring_manager"
)

// JobStatus 岗位生命周期状态。
type JobStatus string

const (
	JobStatusDraft   JobStatus = "draft"
	JobStatusPending JobStatus = "pending"
	JobStatusOpen    JobStatus = "open"
	JobStatusClosed  JobStatus = "closed"
)

// JobType 岗位类型。
type JobType string

const (
	JobTypeFullTime JobType = "full_time"
	JobTypeIntern   JobType = "intern"
)

// EducationLevel 学历要求等级（与 shared-types 一致）。
type EducationLevel string

const (
	EducationAny       EducationLevel = "any"
	EducationAssociate EducationLevel = "associate"
	EducationBachelor  EducationLevel = "bachelor"
	EducationMaster    EducationLevel = "master"
	EducationDoctor    EducationLevel = "doctor"
)

// ApplicationStage 候选人投递流转阶段。
type ApplicationStage string

const (
	StageNew       ApplicationStage = "new"
	StageScreening ApplicationStage = "screening"
	StageInterview ApplicationStage = "interview"
	StageOffer     ApplicationStage = "offer"
	StageHired     ApplicationStage = "hired"
	StageRejected  ApplicationStage = "rejected"
)

// ValidStages 合法流转阶段集合。
var ValidStages = map[ApplicationStage]bool{
	StageNew: true, StageScreening: true, StageInterview: true,
	StageOffer: true, StageHired: true, StageRejected: true,
}

// ValidJobStatuses 合法岗位状态集合。
var ValidJobStatuses = map[JobStatus]bool{
	JobStatusDraft: true, JobStatusPending: true, JobStatusOpen: true, JobStatusClosed: true,
}

// CandidateStatusFromStage 由投递阶段推导求职者视角的状态
// （new/screening→processing, interview→interviewing, offer→offered, hired→hired, rejected→rejected）。
func CandidateStatusFromStage(stage string) string {
	switch ApplicationStage(stage) {
	case StageNew, StageScreening:
		return "processing"
	case StageInterview:
		return "interviewing"
	case StageOffer:
		return "offered"
	case StageHired:
		return "hired"
	case StageRejected:
		return "rejected"
	default:
		return "processing"
	}
}

// ============ 领域模型（JSON 契约与 shared-types 一致） ============

// User 内部用户。
type User struct {
	ID             string  `json:"id"`
	Email          string  `json:"email"`
	Name           string  `json:"name"`
	Role           string  `json:"role"`
	DepartmentID   *string `json:"departmentId,omitempty"`
	DepartmentName *string `json:"departmentName,omitempty"`
}

// Department 部门。
type Department struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// JobRequirements 结构化岗位要求 —— 「可解释匹配」的基础。
type JobRequirements struct {
	MustSkills   []string `json:"mustSkills"`
	NiceSkills   []string `json:"niceSkills"`
	MinEducation string   `json:"minEducation"`
	MinYears     int      `json:"minYears"`
}

// JobPosting 岗位。
type JobPosting struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	DepartmentID   string          `json:"departmentId"`
	DepartmentName string          `json:"departmentName"`
	Location       string          `json:"location"`
	JobType        string          `json:"jobType"`
	Headcount      int             `json:"headcount"`
	SalaryMin      *int            `json:"salaryMin,omitempty"`
	SalaryMax      *int            `json:"salaryMax,omitempty"`
	Description    string          `json:"description"`
	Requirements   JobRequirements `json:"requirements"`
	Status         string          `json:"status"`
	OwnerID        string          `json:"ownerId"`
	OwnerName      string          `json:"ownerName"`
	PublishedAt    *time.Time      `json:"publishedAt,omitempty"`
	ClosedAt       *time.Time      `json:"closedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// JobPostingInput 创建/更新岗位的请求体。
type JobPostingInput struct {
	Title        string          `json:"title"`
	DepartmentID string          `json:"departmentId"`
	Location     string          `json:"location"`
	JobType      string          `json:"jobType"`
	Headcount    int             `json:"headcount"`
	SalaryMin    *int            `json:"salaryMin,omitempty"`
	SalaryMax    *int            `json:"salaryMax,omitempty"`
	Description  string          `json:"description"`
	Requirements JobRequirements `json:"requirements"`
}

// Candidate 外部求职者。
type Candidate struct {
	ID    string
	Email string
	Phone string
	Name  string
}

// Application 投递记录（创建与 worker 写入使用）。
type Application struct {
	ID            string
	CandidateID   string
	JobID         string
	Stage         string
	Source        string
	ResumeFileKey string
	ResumeText    string
}

// EducationItem AI 简历解析：教育经历条目。
type EducationItem struct {
	Level   string `json:"level"`
	School  string `json:"school"`
	Major   string `json:"major"`
	EndYear *int   `json:"endYear,omitempty"`
}

// WorkExperienceItem AI 简历解析：工作经历条目。
type WorkExperienceItem struct {
	Company     string  `json:"company"`
	Title       string  `json:"title"`
	StartDate   *string `json:"startDate,omitempty"`
	EndDate     *string `json:"endDate,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ParsedResume AI 简历结构化解析结果。
type ParsedResume struct {
	Name              string                `json:"name"`
	Email             string                `json:"email"`
	Phone             string                `json:"phone"`
	YearsOfExperience float64               `json:"yearsOfExperience"`
	Education         []EducationItem       `json:"education"`
	Skills            []string              `json:"skills"`
	WorkExperience    []WorkExperienceItem  `json:"workExperience"`
	Summary           string                `json:"summary"`
}

// HardCheck 硬性条件逐项检查结果。
type HardCheck struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// MatchDetail 匹配详情 —— 可解释 AI 的载体。
type MatchDetail struct {
	Score         int         `json:"score"`
	RuleScore     int         `json:"ruleScore"`
	SemanticScore *int        `json:"semanticScore"`
	LLMScore      *int        `json:"llmScore"`
	Strengths     []string    `json:"strengths"`
	Gaps          []string    `json:"gaps"`
	Risk          string      `json:"risk"`
	Summary       string      `json:"summary"`
	HardChecks    []HardCheck `json:"hardChecks"`
	Model         *string     `json:"model"`
	Engine        string      `json:"engine"`
	ScoredAt      time.Time   `json:"scoredAt"`
}

// ApplicationInternal 内部端候选人投递视图。
type ApplicationInternal struct {
	ID            string           `json:"id"`
	JobID         string           `json:"jobId"`
	JobTitle      string           `json:"jobTitle"`
	CandidateID   string           `json:"candidateId"`
	CandidateName string           `json:"candidateName"`
	Email         string           `json:"email"`
	Phone         string           `json:"phone"`
	Stage         string           `json:"stage"`
	Source        string           `json:"source"`
	SubmittedAt   time.Time        `json:"submittedAt"`
	MatchScore    *float64         `json:"matchScore"`
	HardPass      bool             `json:"hardPass"`
	ParseFailed   bool             `json:"parseFailed"`
	ParsedResume  *ParsedResume    `json:"parsedResume"`
	MatchDetail   *MatchDetail     `json:"matchDetail"`
	HasResumeFile bool             `json:"hasResumeFile"`
}

// ApplicationPublic 求职者视角的投递状态视图。
type ApplicationPublic struct {
	ID          string    `json:"id"`
	JobID       string    `json:"jobId"`
	JobTitle    string    `json:"jobTitle"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submittedAt"`
}

// JobStats 岗位投递漏斗统计。
type JobStats struct {
	Total         int             `json:"total"`
	ByStage       map[string]int  `json:"byStage"`
	AvgScore      *float64        `json:"avgScore"`
	HardPassCount int             `json:"hardPassCount"`
}

// Paginated 分页响应。
type Paginated[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// ============ 错误类型 ============

// 错误码（与前端 ApiErrorBody 契约一致）。
const (
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeBadRequest   = "bad_request"
	CodeConflict     = "conflict"
	CodeRateLimited  = "rate_limited"
	CodeInternal     = "internal"
)

// AppError 业务错误：携带 HTTP 状态码与前端错误码。
type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap 支持 errors.Is/As。
func (e *AppError) Unwrap() error { return e.Err }

// NewError 构造业务错误。
func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

// WrapError 构造带底层错误的业务错误。
func WrapError(status int, code, message string, err error) *AppError {
	return &AppError{Status: status, Code: code, Message: message, Err: err}
}
