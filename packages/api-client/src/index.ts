/**
 * 类型安全的 API 客户端（fetch 封装）。
 * 当前为手写实现，与 Go 后端契约保持一致；
 * 后续可用 openapi-typescript 从后端 OpenAPI 自动生成替换。
 */
import type {
  ApiErrorBody,
  ApplicationInternal,
  ApplicationListQuery,
  ApplicationPublic,
  ApplyRequest,
  BatchActionRequest,
  Department,
  JobListQuery,
  JobPosting,
  JobPostingInput,
  JobStats,
  LoginRequest,
  LoginResponse,
  Paginated,
  SendCodeRequest,
  User,
  VerifyCodeRequest,
  VerifyCodeResponse,
} from '@recruitmate/shared-types';

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

export type TokenProvider = () => string | null;

async function parseError(res: Response): Promise<never> {
  let code = 'unknown';
  let message = `请求失败（HTTP ${res.status}）`;
  try {
    const body = (await res.json()) as ApiErrorBody;
    if (body?.error) {
      code = body.error.code;
      message = body.error.message;
    }
  } catch {
    // 非 JSON 响应，保留默认信息
  }
  throw new ApiError(res.status, code, message);
}

export class ApiClient {
  constructor(
    private readonly baseUrl: string,
    private readonly getToken: TokenProvider = () => null,
  ) {}

  private async request<T>(path: string, init: RequestInit = {}, token?: string | null): Promise<T> {
    const t = token !== undefined ? token : this.getToken();
    const headers = new Headers(init.headers);
    if (t) headers.set('Authorization', `Bearer ${t}`);
    if (init.body && typeof init.body === 'string') headers.set('Content-Type', 'application/json');

    const res = await fetch(`${this.baseUrl}${path}`, { ...init, headers });
    if (!res.ok) await parseError(res);
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  // ============ 外部端（public） ============
  listJobs(query: JobListQuery = {}): Promise<Paginated<JobPosting>> {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== '') qs.set(k, String(v));
    }
    const suffix = qs.size > 0 ? `?${qs.toString()}` : '';
    return this.request<Paginated<JobPosting>>(`/public/jobs${suffix}`);
  }

  getJob(id: string): Promise<JobPosting> {
    return this.request<JobPosting>(`/public/jobs/${id}`);
  }

  /** 投递简历：resumeFile 与 resumeText 至少提供一个（文件优先） */
  applyJob(jobId: string, data: ApplyRequest, resumeFile?: File): Promise<{ id: string }> {
    const form = new FormData();
    form.set('name', data.name);
    form.set('email', data.email);
    form.set('phone', data.phone);
    if (data.source) form.set('source', data.source);
    if (data.resumeText) form.set('resumeText', data.resumeText);
    if (resumeFile) form.set('resume', resumeFile);
    return this.request<{ id: string }>(`/public/jobs/${jobId}/applications`, {
      method: 'POST',
      body: form,
    });
  }

  sendCode(body: SendCodeRequest): Promise<void> {
    return this.request<void>('/public/auth/email-code', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  verifyCode(body: VerifyCodeRequest): Promise<VerifyCodeResponse> {
    return this.request<VerifyCodeResponse>('/public/auth/verify', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  myApplications(token: string): Promise<ApplicationPublic[]> {
    return this.request<ApplicationPublic[]>('/public/my/applications', {}, token);
  }

  // ============ 内部端（internal，JWT + RBAC） ============
  login(body: LoginRequest): Promise<LoginResponse> {
    return this.request<LoginResponse>('/internal/auth/login', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  me(): Promise<User> {
    return this.request<User>('/internal/me');
  }

  listDepartments(): Promise<Department[]> {
    return this.request<Department[]>('/internal/departments');
  }

  listJobsInternal(query: { status?: string; page?: number; pageSize?: number } = {}): Promise<Paginated<JobPosting>> {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== '') qs.set(k, String(v));
    }
    const suffix = qs.size > 0 ? `?${qs.toString()}` : '';
    return this.request<Paginated<JobPosting>>(`/internal/jobs${suffix}`);
  }

  getJobInternal(id: string): Promise<JobPosting> {
    return this.request<JobPosting>(`/internal/jobs/${id}`);
  }

  createJob(body: JobPostingInput): Promise<JobPosting> {
    return this.request<JobPosting>('/internal/jobs', { method: 'POST', body: JSON.stringify(body) });
  }

  updateJob(id: string, body: JobPostingInput): Promise<JobPosting> {
    return this.request<JobPosting>(`/internal/jobs/${id}`, { method: 'PATCH', body: JSON.stringify(body) });
  }

  /** 草稿提交审批 */
  submitJob(id: string): Promise<JobPosting> {
    return this.request<JobPosting>(`/internal/jobs/${id}/submit`, { method: 'POST' });
  }

  approveJob(id: string): Promise<JobPosting> {
    return this.request<JobPosting>(`/internal/jobs/${id}/approve`, { method: 'POST' });
  }

  rejectJob(id: string): Promise<JobPosting> {
    return this.request<JobPosting>(`/internal/jobs/${id}/reject`, { method: 'POST' });
  }

  closeJob(id: string): Promise<JobPosting> {
    return this.request<JobPosting>(`/internal/jobs/${id}/close`, { method: 'POST' });
  }

  reopenJob(id: string): Promise<JobPosting> {
    return this.request<JobPosting>(`/internal/jobs/${id}/reopen`, { method: 'POST' });
  }

  listApplications(jobId: string, query: ApplicationListQuery = {}): Promise<Paginated<ApplicationInternal>> {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== '') qs.set(k, String(v));
    }
    const suffix = qs.size > 0 ? `?${qs.toString()}` : '';
    return this.request<Paginated<ApplicationInternal>>(`/internal/jobs/${jobId}/applications${suffix}`);
  }

  getApplication(id: string): Promise<ApplicationInternal> {
    return this.request<ApplicationInternal>(`/internal/applications/${id}`);
  }

  setStage(id: string, stage: ApplicationInternal['stage']): Promise<ApplicationInternal> {
    return this.request<ApplicationInternal>(`/internal/applications/${id}/stage`, {
      method: 'PATCH',
      body: JSON.stringify({ stage }),
    });
  }

  batchAction(body: BatchActionRequest): Promise<{ updated: number }> {
    return this.request<{ updated: number }>('/internal/applications/batch', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  jobStats(jobId: string): Promise<JobStats> {
    return this.request<JobStats>(`/internal/jobs/${jobId}/stats`);
  }

  /** 简历原文件下载地址（预签名 URL） */
  resumeUrl(applicationId: string): Promise<{ url: string }> {
    return this.request<{ url: string }>(`/internal/applications/${applicationId}/resume-url`);
  }
}

/** 默认单例：Vite dev 下由代理转发到 Go API */
export const api = new ApiClient('/api/v1');
