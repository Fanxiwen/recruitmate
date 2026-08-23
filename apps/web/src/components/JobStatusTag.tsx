import { JOB_STATUS_LABELS, type JobStatus } from '@recruitmate/shared-types';
import { Tag } from 'antd';

/** 岗位状态 → Tag 颜色（草稿灰 / 待审批橙 / 招聘中绿 / 已关闭默认） */
const STATUS_COLORS: Record<JobStatus, string> = {
  draft: 'default',
  pending: 'orange',
  open: 'green',
  closed: 'default',
};

/** 岗位状态 Tag（标签文案来自 shared-types 的 JOB_STATUS_LABELS） */
export function JobStatusTag({ status }: { status: JobStatus }) {
  return <Tag color={STATUS_COLORS[status]}>{JOB_STATUS_LABELS[status]}</Tag>;
}
