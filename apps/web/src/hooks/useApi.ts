import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@recruitmate/api-client';
import type {
  ApplicationInternal,
  ApplicationStage,
  BatchActionRequest,
  JobPosting,
  JobPostingInput,
  JobStatus,
  OfferRequest,
} from '@recruitmate/shared-types';
import { api } from '../lib/api';
import { useAuthStore } from '../stores/auth';

/**
 * 统一请求守卫：401 时清空认证状态并跳转登录页。
 * 所有 queryFn / mutationFn 都经由此包装，保证「401 即登出」行为一致。
 */
export async function request<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      useAuthStore.getState().logout();
      // 部署基路径感知：内部端可能部署在 /hr/ 子路径下，
      // 硬编码 '/login' 会跳出到站点根（外部端）。
      const base = (import.meta.env.VITE_BASE_PATH ?? '/').replace(/\/$/, '');
      const loginPath = `${base}/login`;
      if (window.location.pathname !== loginPath) {
        window.location.assign(loginPath);
      }
    }
    throw err;
  }
}

/** 通用错误提示文案（供 mutation onError 使用） */
export function errorMessage(err: unknown, fallback = '操作失败，请稍后重试'): string {
  return err instanceof ApiError ? err.message : fallback;
}

// ==================== 查询 ====================

/** 部门列表（岗位表单、列表筛选用） */
export function useDepartments() {
  return useQuery({
    queryKey: ['departments'],
    queryFn: () => request(() => api.listDepartments()),
  });
}

export interface JobsQuery {
  status?: JobStatus;
  page: number;
  pageSize: number;
}

/** 内部端岗位分页列表 */
export function useJobs(params: JobsQuery) {
  return useQuery({
    queryKey: ['jobs', params],
    queryFn: () => request(() => api.listJobsInternal(params)),
    placeholderData: (prev) => prev,
  });
}

/** 待审批岗位数量（Sider 徽标），pageSize=1 只取 total */
export function usePendingCount() {
  return useQuery({
    queryKey: ['jobs', 'pendingCount'],
    queryFn: () =>
      request(() => api.listJobsInternal({ status: 'pending', page: 1, pageSize: 1 })),
    refetchInterval: 60_000,
  });
}

/** 单个岗位详情 */
export function useJob(id: string) {
  return useQuery({
    queryKey: ['job', id],
    queryFn: () => request(() => api.getJobInternal(id)),
    enabled: !!id,
  });
}

/** 岗位统计（总投递 / 各阶段计数 / 平均分 / 硬性条件不满足数） */
export function useJobStats(id: string) {
  return useQuery({
    queryKey: ['jobStats', id],
    queryFn: () => request(() => api.jobStats(id)),
    enabled: !!id,
  });
}

export interface ApplicationsQuery {
  stage?: ApplicationStage;
  hardPass?: 'only' | 'exclude';
  q?: string;
  sort?: 'score_desc' | 'score_asc' | 'newest';
  page: number;
  pageSize: number;
}

/**
 * 候选人分页列表。
 * refetchInterval 用于 AI 评分异步就绪后的自动轮询（评分中状态也支持手动刷新）。
 */
export function useApplications(jobId: string, query: ApplicationsQuery, refetchInterval = 30_000) {
  return useQuery({
    queryKey: ['applications', jobId, query],
    queryFn: () => request(() => api.listApplications(jobId, query)),
    enabled: !!jobId,
    refetchInterval,
    placeholderData: (prev) => prev,
  });
}

/** 候选人详情（Drawer 打开时拉取，含简历原文等最新字段） */
export function useApplicationDetail(id: string, enabled: boolean) {
  return useQuery({
    queryKey: ['application', id],
    queryFn: () => request(() => api.getApplication(id)),
    enabled: enabled && !!id,
    staleTime: 15_000,
  });
}

// ==================== 变更 ====================

/** 创建岗位（保存草稿） */
export function useCreateJob() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: JobPostingInput) => request(() => api.createJob(body)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
  });
}

/** 编辑岗位（PATCH） */
export function useUpdateJob(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: JobPostingInput) => request(() => api.updateJob(id, body)),
    onSuccess: (job) => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      queryClient.invalidateQueries({ queryKey: ['job', job.id] });
    },
  });
}

export type JobAction = 'submit' | 'approve' | 'reject' | 'close' | 'reopen';

const JOB_ACTION_FN: Record<JobAction, (id: string) => Promise<JobPosting>> = {
  submit: (id) => api.submitJob(id),
  approve: (id) => api.approveJob(id),
  reject: (id) => api.rejectJob(id),
  close: (id) => api.closeJob(id),
  reopen: (id) => api.reopenJob(id),
};

/** 岗位审批流动作：提交审批 / 审批通过 / 审批驳回 / 关闭 / 重新开启 */
export function useJobAction(action: JobAction) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => request(() => JOB_ACTION_FN[action](id)),
    onSuccess: (_job, id) => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      queryClient.invalidateQueries({ queryKey: ['job', id] });
      queryClient.invalidateQueries({ queryKey: ['jobStats', id] });
    },
  });
}

/** 单个候选人阶段流转（表格行内 Select / Drawer 内 Select / 淘汰原因弹窗共用） */
export function useSetStage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, stage, reason }: { id: string; stage: ApplicationStage; reason?: string }) =>
      request(() => api.setStage(id, stage, reason)),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
      queryClient.invalidateQueries({ queryKey: ['application', updated.id] });
      queryClient.invalidateQueries({ queryKey: ['jobStats', updated.jobId] });
    },
  });
}

/** 发起 Offer 审批（HR/管理员，stage 必须为 interview） */
export function useCreateOffer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: OfferRequest }) =>
      request(() => api.createOffer(id, body)),
    onSuccess: (_offer, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
      queryClient.invalidateQueries({ queryKey: ['application', id] });
      queryClient.invalidateQueries({ queryKey: ['jobStats'] });
    },
  });
}

/** Offer 审批动作：通过（定薪必填）/ 驳回（原因必填） */
export function useDecideOffer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      decision,
      salary,
      reason,
    }: {
      id: string;
      decision: 'approve' | 'reject';
      salary?: string;
      reason?: string;
    }) => request(() => api.decideOffer(id, decision, { salary, reason })),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
      queryClient.invalidateQueries({ queryKey: ['application', updated.id] });
      queryClient.invalidateQueries({ queryKey: ['jobStats', updated.jobId] });
    },
  });
}

/** 批量操作（批量通过到初筛 / 批量淘汰） */
export function useBatchAction(jobId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: BatchActionRequest) => request(() => api.batchAction(body)),
    onSuccess: (_res, body) => {
      queryClient.invalidateQueries({ queryKey: ['applications'] });
      body.ids.forEach((id) => queryClient.invalidateQueries({ queryKey: ['application', id] }));
      queryClient.invalidateQueries({ queryKey: ['jobStats', jobId] });
    },
  });
}

/** 供 MatchDrawer 等一次性调用使用的类型（列表项 + 可选简历原文） */
export type ApplicationDetail = ApplicationInternal & { resumeText?: string };
