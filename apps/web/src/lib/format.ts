import type { ApplicationStage, InterviewRound, Role } from '@recruitmate/shared-types';
import dayjs from 'dayjs';

/** 角色中文名（shared-types 未提供，此处为前端展示用本地映射） */
export const ROLE_LABELS: Record<Role, string> = {
  admin: '管理员',
  hr: 'HR',
  hiring_manager: '部门负责人',
};

/** 面试轮次中文名 */
export const INTERVIEW_ROUND_LABELS: Record<InterviewRound, string> = {
  hr: 'HR 初面',
  manager: '部门负责人面',
};

/** 阶段对应的 Tag 颜色（仅展示用） */
export const STAGE_COLORS: Record<ApplicationStage, string> = {
  new: 'blue',
  screening: 'cyan',
  interview: 'geekblue',
  manager_interview: 'volcano',
  offer_pending: 'purple',
  offered: 'gold',
  hired: 'green',
  rejected: 'red',
};

/** 统一时间格式化（dayjs），空值显示占位符 */
export function formatDateTime(value?: string | null): string {
  if (!value) return '—';
  return dayjs(value).format('YYYY-MM-DD HH:mm');
}

/**
 * 薪资展示：千元/月。
 * 两端都有值 → 区间；只有一端 → 开区间；都没有 → 面议。
 */
export function formatSalary(min?: number, max?: number): string {
  if (min == null && max == null) return '面议';
  if (min != null && max != null) return `${min}-${max} 千元/月`;
  if (min != null) return `${min}+ 千元/月`;
  return `≤${max} 千元/月`;
}
