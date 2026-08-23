import type { JobPosting } from '@recruitmate/shared-types';
import { Button, Popconfirm, Space, message, type ButtonProps } from 'antd';
import { useNavigate } from 'react-router-dom';
import { errorMessage, useJobAction } from '../hooks/useApi';
import { useAuthStore } from '../stores/auth';

interface JobActionsProps {
  job: JobPosting;
  /** 按钮尺寸（列表行内用 small，详情页顶栏用默认） */
  size?: ButtonProps['size'];
}

/**
 * 岗位操作按钮组（列表与详情页顶栏共用）：
 * 编辑（仅草稿）/ 提交审批（仅草稿）/ 审批通过、驳回（仅待审批且有权限）/
 * 关闭（招聘中）/ 重新开启（已关闭）。
 * 无权限的操作按钮直接不渲染；审批类动作带 Popconfirm。
 * 权限规则：hiring_manager 只能审批本部门岗位。
 */
export function JobActions({ job, size }: JobActionsProps) {
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const submit = useJobAction('submit');
  const approve = useJobAction('approve');
  const reject = useJobAction('reject');
  const close = useJobAction('close');
  const reopen = useJobAction('reopen');

  // admin / hr 可审批全部；hiring_manager 仅限本部门
  const canApprove = user?.role !== 'hiring_manager' || job.departmentId === user?.departmentId;

  return (
    <Space size={4} wrap>
      {job.status === 'draft' && (
        <>
          <Button size={size} onClick={() => navigate(`/jobs/${job.id}/edit`)}>
            编辑
          </Button>
          <Button
            size={size}
            loading={submit.isPending}
            onClick={() =>
              submit.mutate(job.id, {
                onSuccess: () => message.success('已提交审批，审批通过后将发布到外部端'),
                onError: (e) => message.error(errorMessage(e, '提交审批失败')),
              })
            }
          >
            提交审批
          </Button>
        </>
      )}
      {job.status === 'pending' && canApprove && (
        <>
          <Popconfirm
            title="确认审批通过并发布该岗位？"
            description="通过后岗位将对外展示，候选人可开始投递。"
            okText="审批通过"
            cancelText="取消"
            onConfirm={() =>
              approve.mutate(job.id, {
                onSuccess: () => message.success('审批通过，岗位已发布'),
                onError: (e) => message.error(errorMessage(e, '审批失败')),
              })
            }
          >
            <Button size={size} type="primary" loading={approve.isPending}>
              审批通过
            </Button>
          </Popconfirm>
          <Popconfirm
            title="确认驳回该岗位？"
            description="驳回后岗位状态将回到草稿，可修改后重新提交。"
            okText="驳回"
            cancelText="取消"
            okButtonProps={{ danger: true }}
            onConfirm={() =>
              reject.mutate(job.id, {
                onSuccess: () => message.success('已驳回，岗位回到草稿'),
                onError: (e) => message.error(errorMessage(e, '驳回失败')),
              })
            }
          >
            <Button size={size} danger loading={reject.isPending}>
              驳回
            </Button>
          </Popconfirm>
        </>
      )}
      {job.status === 'open' && (
        <Popconfirm
          title="确认关闭该岗位？"
          description="关闭后岗位将不再接收新投递。"
          okText="关闭"
          cancelText="取消"
          okButtonProps={{ danger: true }}
          onConfirm={() =>
            close.mutate(job.id, {
              onSuccess: () => message.success('岗位已关闭'),
              onError: (e) => message.error(errorMessage(e, '关闭失败')),
            })
          }
        >
          <Button size={size} loading={close.isPending}>
            关闭
          </Button>
        </Popconfirm>
      )}
      {job.status === 'closed' && (
        <Popconfirm
          title="确认重新开启该岗位？"
          description="重新开启后岗位恢复接收投递。"
          okText="重新开启"
          cancelText="取消"
          onConfirm={() =>
            reopen.mutate(job.id, {
              onSuccess: () => message.success('岗位已重新开启'),
              onError: (e) => message.error(errorMessage(e, '重新开启失败')),
            })
          }
        >
          <Button size={size} loading={reopen.isPending}>
            重新开启
          </Button>
        </Popconfirm>
      )}
    </Space>
  );
}
