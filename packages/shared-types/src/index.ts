/**
 * recruitmate 前后端共享类型契约（单一事实来源）。
 * Go 后端的 JSON 结构必须与本文件保持一致；前端仅依赖本包。
 */

// ============ 通用 ============
export type ID = string;
export type ISODate = string;

export interface Paginated<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

/** 后端统一错误响应体 */
export interface ApiErrorBody {
  error: { code: string; message: string };
}

// ============ 枚举 ============
export type Role = 'admin' | 'hr' | 'hiring_manager';
export type JobStatus = 'draft' | 'pending' | 'open' | 'closed';
export type JobType = 'full_time' | 'intern';
export type EducationLevel = 'any' | 'associate' | 'bachelor' | 'master' | 'doctor';
export type ApplicationStage =
  | 'new'
  | 'screening'
  | 'interview'
  | 'manager_interview'
  | 'offer_pending'
  | 'offered'
  | 'hired'
  | 'rejected';
/** 求职者视角的投递状态（由后端从 stage 推导，不暴露内部细节） */
export type CandidateApplicationStatus =
  | 'processing'
  | 'screening'
  | 'interviewing'
  | 'offered'
  | 'hired'
  | 'rejected';
export type MatchEngine = 'ai' | 'rule';

export const EDUCATION_LEVEL_LABELS: Record<EducationLevel, string> = {
  any: '不限',
  associate: '大专',
  bachelor: '本科',
  master: '硕士',
  doctor: '博士',
};

export const JOB_STATUS_LABELS: Record<JobStatus, string> = {
  draft: '草稿',
  pending: '待审批',
  open: '招聘中',
  closed: '已关闭',
};

export const STAGE_LABELS: Record<ApplicationStage, string> = {
  new: '新简历',
  screening: '初筛通过',
  interview: 'HR初面',
  manager_interview: '部门负责人面',
  offer_pending: 'Offer审批中',
  offered: '已发Offer',
  hired: '已入职',
  rejected: '已淘汰',
};

/**
 * 阶段流转状态机（服务端强制，前端据此禁用非法流转）。
 * 面试阶段推进由「安排面试/完成面试」动作驱动，不可手动跳转：
 *  - screening→interview：安排 HR 面（含面试时间）
 *  - interview→manager_interview：HR 面通过后安排负责人面（含面试时间）
 *  - manager_interview→offer_pending：负责人面通过后由 HR 发起 Offer
 *  - offer_pending→offered/manager_interview：Offer 审批通过/驳回
 * rejected → new 为「误杀恢复」，仅限 HR 手动执行并填写原因。
 */
export const STAGE_TRANSITIONS: Record<ApplicationStage, ApplicationStage[]> = {
  new: ['screening', 'rejected'],
  screening: ['rejected'],
  interview: ['rejected'],
  manager_interview: ['rejected'],
  offer_pending: ['offered', 'manager_interview'],
  offered: ['hired', 'rejected'],
  hired: [],
  rejected: ['new'],
};

export const JOB_TYPE_LABELS: Record<JobType, string> = {
  full_time: '全职',
  intern: '实习',
};

// ============ 领域模型 ============
export interface User {
  id: ID;
  email: string;
  name: string;
  role: Role;
  departmentId?: ID;
  departmentName?: string;
}

export interface Department {
  id: ID;
  name: string;
}

/** 岗位要求 —— 结构化，是「可解释匹配」的基础 */
export interface JobRequirements {
  /** 必备技能 */
  mustSkills: string[];
  /** 加分技能 */
  niceSkills: string[];
  minEducation: EducationLevel;
  /** 最低工作年限 */
  minYears: number;
}

export interface JobPosting {
  id: ID;
  title: string;
  departmentId: ID;
  departmentName: string;
  location: string;
  jobType: JobType;
  headcount: number;
  /** 月薪（千元）区间，可空 */
  salaryMin?: number;
  salaryMax?: number;
  description: string;
  requirements: JobRequirements;
  status: JobStatus;
  ownerId: ID;
  ownerName: string;
  publishedAt?: ISODate;
  closedAt?: ISODate;
  createdAt: ISODate;
  updatedAt: ISODate;
  /** 投递总数（内部端列表接口返回） */
  applicationCount?: number;
  /** 已入职人数（内部端列表接口返回，用于招聘进度展示） */
  hiredCount?: number;
}

export interface JobPostingInput {
  title: string;
  departmentId: ID;
  location: string;
  jobType: JobType;
  headcount: number;
  salaryMin?: number;
  salaryMax?: number;
  description: string;
  requirements: JobRequirements;
}

export interface EducationItem {
  level: string;
  school: string;
  major: string;
  endYear?: number;
}

export interface WorkExperienceItem {
  company: string;
  title: string;
  startDate?: string;
  endDate?: string;
  description?: string;
}

/** AI 简历结构化解析结果 */
export interface ParsedResume {
  name: string;
  email: string;
  phone: string;
  yearsOfExperience: number;
  education: EducationItem[];
  skills: string[];
  workExperience: WorkExperienceItem[];
  summary: string;
}

/** 硬性条件逐项检查结果 */
export interface HardCheck {
  name: string;
  pass: boolean;
  detail: string;
}

/** 匹配详情 —— 可解释 AI 的载体 */
export interface MatchDetail {
  /** 综合分 0-100 */
  score: number;
  /** 规则分 0-100 */
  ruleScore: number;
  /** 语义相似度分 0-100（无 embedding 时为 null） */
  semanticScore: number | null;
  /** LLM 评委分 0-100（未走评委时为 null） */
  llmScore: number | null;
  strengths: string[];
  gaps: string[];
  risk: string;
  summary: string;
  hardChecks: HardCheck[];
  model: string | null;
  engine: MatchEngine;
  scoredAt: ISODate;
}

export interface ApplicationInternal {
  id: ID;
  jobId: ID;
  jobTitle: string;
  candidateId: ID;
  candidateName: string;
  email: string;
  phone: string;
  stage: ApplicationStage;
  source: string;
  submittedAt: ISODate;
  /** 综合匹配分 0-100，未评分时为 null */
  matchScore: number | null;
  hardPass: boolean;
  parseFailed: boolean;
  parsedResume: ParsedResume | null;
  matchDetail: MatchDetail | null;
  hasResumeFile: boolean;
  /** 简历提取文本（内部端详情接口返回，便于快速预览原文） */
  resumeText?: string;
  /** 淘汰原因（阶段流转必填） */
  rejectReason?: string;
  /** 面试评价 */
  interviewFeedback?: string;
  /** 当前 Offer 审批单（详情接口返回） */
  offer?: Offer | null;
  /** 流转时间线（详情接口返回） */
  events?: ApplicationEvent[];
  /** 面试记录（详情/候选人中心返回，按轮次 hr / manager） */
  interviews?: Interview[];
}

/** Offer 审批单 */
export interface Offer {
  id: ID;
  salary: string;
  joinDate: string;
  note: string;
  status: 'pending' | 'approved' | 'rejected';
  requestedByName: string;
  decidedByName?: string;
  requestedAt: ISODate;
  decidedAt?: ISODate;
}

/** 流转时间线事件 */
export interface ApplicationEvent {
  id: number;
  fromStage: string;
  toStage: string;
  action:
    | 'stage_change'
    | 'offer_request'
    | 'offer_approve'
    | 'offer_reject'
    | 'feedback'
    | 'interview_schedule'
    | 'interview_complete';
  actorName: string;
  reason: string;
  createdAt: ISODate;
}

export interface ApplicationPublic {
  id: ID;
  jobId: ID;
  jobTitle: string;
  status: CandidateApplicationStatus;
  submittedAt: ISODate;
}

export interface JobStats {
  total: number;
  byStage: Record<ApplicationStage, number>;
  avgScore: number | null;
  hardPassCount: number;
}

// ============ API 请求/响应 ============
export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface SendCodeRequest {
  email: string;
}

export interface SendCodeResponse {
  message: string;
  /** 非生产环境返回的验证码（演示模式）；生产环境不返回该字段 */
  devCode?: string;
}

export interface VerifyCodeRequest {
  email: string;
  code: string;
}

export interface VerifyCodeResponse {
  token: string;
}

export interface ApplyRequest {
  name: string;
  email: string;
  phone: string;
  source?: string;
  /** 与上传简历文件二选一（可同时提供，文件优先） */
  resumeText?: string;
}

export interface JobListQuery {
  q?: string;
  departmentId?: ID;
  jobType?: JobType;
  page?: number;
  pageSize?: number;
}

export interface ApplicationListQuery {
  stage?: ApplicationStage;
  /** only: 只看不满足硬性要求；exclude: 过滤掉 */
  hardPass?: 'only' | 'exclude';
  q?: string;
  sort?: 'score_desc' | 'score_asc' | 'newest';
  page?: number;
  pageSize?: number;
}

export interface BatchActionRequest {
  ids: ID[];
  action: 'stage' | 'reject' | 'hired';
  /** action 为 stage 时必填 */
  stage?: ApplicationStage;
  /** 淘汰原因（action 为 reject 时建议填写） */
  reason?: string;
}

/** 阶段流转请求（转 rejected 时 reason 必填） */
export interface SetStageRequest {
  stage: ApplicationStage;
  reason?: string;
}

/** 发起 Offer 审批请求（salary 为建议薪资，最终薪资由部门负责人审批时确定） */
export interface OfferRequest {
  salary?: string;
  joinDate?: string;
  note?: string;
}

/** 审批动作请求（通过时 salary 为最终薪资，必填） */
export interface OfferDecisionRequest {
  salary?: string;
  reason?: string;
}

/** 审批中心：Offer 待审批条目（候选人 + 审批单） */
export interface ApprovalOfferItem {
  application: ApplicationInternal;
  offer: Offer;
}

/** 面试轮次 */
export type InterviewRound = 'hr' | 'manager';
export type InterviewResult = 'pending' | 'pass' | 'fail';

/** 一轮面试（含准确面试时间、评价与结论） */
export interface Interview {
  id: ID;
  round: InterviewRound;
  /** 面试时间（安排时必填） */
  scheduledAt?: ISODate;
  status: 'scheduled' | 'completed' | 'cancelled';
  result: InterviewResult;
  feedback: string;
  reviewedByName: string;
  reviewedAt?: ISODate;
}

/** 安排面试请求 */
export interface ScheduleInterviewRequest {
  round: InterviewRound;
  /** ISO 时间，必填 */
  scheduledAt: string;
}

/** 完成面试请求（评价必填） */
export interface CompleteInterviewRequest {
  result: 'pass' | 'fail';
  feedback: string;
}

/** 候选人中心查询参数 */
export interface CandidateListQuery {
  stage?: ApplicationStage | '';
  departmentId?: ID;
  jobId?: ID;
  q?: string;
  sort?: 'score_desc' | 'newest';
  page?: number;
  pageSize?: number;
}

/** 我的待办：面试类待办条目 */
export interface InterviewTodoItem {
  /** screen 待初筛 / schedule_hr 待安排HR面 / review_hr HR面待评价 / schedule_manager 待安排负责人面 / review_manager 负责人面待评价 / offer_ready 待发起Offer */
  kind:
    | 'screen'
    | 'schedule_hr'
    | 'review_hr'
    | 'schedule_manager'
    | 'review_manager'
    | 'offer_ready';
  application: ApplicationInternal;
  interview?: Interview;
}
