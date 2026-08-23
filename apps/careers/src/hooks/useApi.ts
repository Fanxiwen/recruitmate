/**
 * 统一的数据请求 hooks 与错误处理。
 * 所有 API 调用一律通过 @recruitmate/api-client 的类型安全客户端（api 单例）完成，
 * 禁止在组件内直接 fetch。
 */
import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query';
import { api, ApiError } from '@recruitmate/api-client';
import type { ApplyRequest, Department, JobListQuery } from '@recruitmate/shared-types';

// ============ 求职者 token 存取（localStorage） ============

const TOKEN_KEY = 'recruitmate.careers.token';

export function getAuthToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setAuthToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token);
  } catch {
    /* 隐私模式等场景下忽略 */
  }
}

export function clearAuthToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {
    /* 忽略 */
  }
}

// ============ 错误处理 ============

/** 统一错误文案：优先取后端 ApiError.message，网络错误给友好提示 */
export function getErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) {
    if (/failed to fetch|network/i.test(err.message)) return '网络异常，请稍后重试';
    return err.message;
  }
  return '请求失败，请稍后重试';
}

/** 是否鉴权失败（token 缺失/过期），用于自动登出 */
export function isUnauthorized(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 401 || err.status === 403);
}

// ============ 岗位 ============

/** 岗位列表（支持 q/departmentId/jobType/page 筛选），切换筛选时保留上一页数据避免闪烁 */
export function useJobs(query: JobListQuery) {
  return useQuery({
    queryKey: ['jobs', query],
    queryFn: () => api.listJobs(query),
    placeholderData: keepPreviousData,
  });
}

/** 岗位详情 */
export function useJob(id: string | undefined) {
  return useQuery({
    queryKey: ['job', id],
    queryFn: () => api.getJob(id as string),
    enabled: Boolean(id),
  });
}

/**
 * 部门选项。
 * 外部端契约中没有公开的部门接口（listDepartments 属内部端），
 * 因此从岗位列表数据中聚合去重得到（取前 100 条，5 分钟缓存）。
 */
export function useDepartments() {
  return useQuery({
    queryKey: ['departments'],
    queryFn: async (): Promise<Department[]> => {
      const res = await api.listJobs({ page: 1, pageSize: 100 });
      const map = new Map<string, Department>();
      for (const job of res.items) {
        if (!map.has(job.departmentId)) {
          map.set(job.departmentId, { id: job.departmentId, name: job.departmentName });
        }
      }
      return [...map.values()];
    },
    staleTime: 5 * 60 * 1000,
  });
}

// ============ 投递 ============

/** 投递简历（文件与文本二选一，文件优先） */
export function useApplyJob(jobId: string) {
  return useMutation({
    mutationFn: ({ data, file }: { data: ApplyRequest; file?: File }) => api.applyJob(jobId, data, file),
  });
}

// ============ 邮箱验证码 ============

/** 发送验证码 */
export function useSendCode() {
  return useMutation({
    mutationFn: (body: { email: string }) => api.sendCode(body),
  });
}

/** 校验验证码换取 token */
export function useVerifyCode() {
  return useMutation({
    mutationFn: (body: { email: string; code: string }) => api.verifyCode(body),
  });
}

/** 我的投递列表（携带求职者 token），无 token 时不请求 */
export function useMyApplications(token: string | null) {
  return useQuery({
    queryKey: ['my-applications', token],
    queryFn: () => api.myApplications(token as string),
    enabled: Boolean(token),
    retry: 0,
  });
}
