import type { Interview } from '@recruitmate/shared-types';
import { Tag } from 'antd';

/** 面试状态 Tag：待面试 / 已通过 / 未通过 / 已完成 / 已取消 */
function statusOf(iv: Interview): { color: string; label: string } {
  if (iv.status === 'scheduled') return { color: 'processing', label: '待面试' };
  if (iv.status === 'cancelled') return { color: 'default', label: '已取消' };
  // completed
  if (iv.result === 'pass') return { color: 'success', label: '已通过' };
  if (iv.result === 'fail') return { color: 'error', label: '未通过' };
  return { color: 'default', label: '已完成' };
}

/** 面试状态 Tag（无面试记录时渲染 null，由调用方决定占位文案） */
export function InterviewStatusTag({ interview }: { interview?: Interview }) {
  if (!interview) return null;
  const { color, label } = statusOf(interview);
  return <Tag color={color}>{label}</Tag>;
}
