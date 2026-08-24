import { useState } from 'react';
import { Badge, Button, Card, Empty, Form, Input, List, Modal, Space, Tag, Typography, message } from 'antd';
import { Link } from 'react-router-dom';
import type { ApprovalOfferItem } from '@recruitmate/shared-types';
import { JobActions } from '../components/JobActions';
import { errorMessage, useApprovalOffers, useDecideOffer, useJobs } from '../hooks/useApi';
import { useAuthStore } from '../stores/auth';
import { formatDateTime } from '../lib/format';

/**
 * 审批中心：岗位发布审批 + Offer 审批统一收口。
 * 解决「待审批入口重复 / Offer 审批无处可见」的问题。
 */
export function ApprovalsPage() {
  const user = useAuthStore((s) => s.user);
  const jobsPending = useJobs({ status: 'pending', page: 1, pageSize: 50 });
  const offersPending = useApprovalOffers(1, 50);
  const decideOffer = useDecideOffer();

  // ===== Offer 审批弹窗状态 =====
  const [approveTarget, setApproveTarget] = useState<ApprovalOfferItem | null>(null);
  const [rejectTarget, setRejectTarget] = useState<ApprovalOfferItem | null>(null);
  const [approveForm] = Form.useForm();
  const [rejectForm] = Form.useForm();

  const canDecide = (item: ApprovalOfferItem) =>
    !!user &&
    (user.role === 'admin' || user.role === 'hiring_manager') &&
    item.offer.requestedByName !== user.name;

  const confirmApprove = async () => {
    if (!approveTarget) return;
    let salary = '';
    try {
      const v = await approveForm.validateFields();
      salary = v.salary as string;
    } catch {
      return;
    }
    try {
      await decideOffer.mutateAsync({ id: approveTarget.application.id, decision: 'approve', salary });
      message.success('Offer 审批已通过');
      setApproveTarget(null);
    } catch (err) {
      message.error(errorMessage(err, '审批失败'));
    }
  };

  const confirmReject = async () => {
    if (!rejectTarget) return;
    let reason = '';
    try {
      const v = await rejectForm.validateFields();
      reason = v.reason as string;
    } catch {
      return;
    }
    try {
      await decideOffer.mutateAsync({ id: rejectTarget.application.id, decision: 'reject', reason });
      message.success('Offer 已驳回');
      setRejectTarget(null);
    } catch (err) {
      message.error(errorMessage(err, '审批失败'));
    }
  };

  const pendingJobs = jobsPending.data?.items ?? [];
  const pendingOffers = offersPending.data?.items ?? [];

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      {/* ===== 岗位发布审批 ===== */}
      <Card
        title={
          <Space size={8}>
            岗位发布审批
            {pendingJobs.length > 0 && <Badge count={pendingJobs.length} />}
          </Space>
        }
        loading={jobsPending.isLoading}
      >
        {pendingJobs.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无待审批岗位" />
        ) : (
          <List
            dataSource={pendingJobs}
            renderItem={(job) => (
              <List.Item
                actions={[<JobActions key="actions" job={job} />]}
                extra={
                  <Link to={`/jobs/${job.id}`}>
                    <Typography.Link>查看详情</Typography.Link>
                  </Link>
                }
              >
                <List.Item.Meta
                  title={
                    <Space>
                      <span>{job.title}</span>
                      <Tag color="blue">{job.departmentName}</Tag>
                      <Tag>需求 {job.headcount} 人</Tag>
                    </Space>
                  }
                  description={`发布人：${job.ownerName} · 创建于 ${formatDateTime(job.createdAt)}`}
                />
              </List.Item>
            )}
          />
        )}
      </Card>

      {/* ===== Offer 审批 ===== */}
      <Card
        title={
          <Space size={8}>
            Offer 审批
            {pendingOffers.length > 0 && <Badge count={pendingOffers.length} />}
          </Space>
        }
        loading={offersPending.isLoading}
      >
        {pendingOffers.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无待审批 Offer" />
        ) : (
          <List
            dataSource={pendingOffers}
            renderItem={(item) => (
              <List.Item
                actions={
                  canDecide(item)
                    ? [
                        <Button
                          key="approve"
                          type="primary"
                          size="small"
                          onClick={() => {
                            approveForm.setFieldsValue({ salary: item.offer.salary });
                            setApproveTarget(item);
                          }}
                        >
                          通过并确定薪资
                        </Button>,
                        <Button
                          key="reject"
                          danger
                          size="small"
                          onClick={() => {
                            rejectForm.resetFields();
                            setRejectTarget(item);
                          }}
                        >
                          驳回
                        </Button>,
                      ]
                    : [
                        <Typography.Text key="hint" type="secondary" style={{ fontSize: 12 }}>
                          {item.offer.requestedByName === user?.name ? '等待他人审批' : '无审批权限'}
                        </Typography.Text>,
                      ]
                }
                extra={
                  <Link to={`/jobs/${item.application.jobId}`}>
                    <Typography.Link>进入岗位</Typography.Link>
                  </Link>
                }
              >
                <List.Item.Meta
                  title={
                    <Space>
                      <span>{item.application.candidateName}</span>
                      <Tag color="purple">{item.application.jobTitle}</Tag>
                    </Space>
                  }
                  description={
                    <Space direction="vertical" size={0}>
                      <span>
                        建议薪资：{item.offer.salary || '—'} K/月 · 入职时间：{item.offer.joinDate || '—'}
                      </span>
                      <span>
                        发起人：{item.offer.requestedByName} · {formatDateTime(item.offer.requestedAt)}
                        {item.offer.note && ` · 备注：${item.offer.note}`}
                      </span>
                    </Space>
                  }
                />
              </List.Item>
            )}
          />
        )}
      </Card>

      {/* 审批通过（定薪）弹窗 */}
      <Modal
        title="通过 Offer 审批 · 确定最终薪资"
        open={!!approveTarget}
        onOk={confirmApprove}
        onCancel={() => setApproveTarget(null)}
        okText="通过并确定薪资"
        cancelText="取消"
        confirmLoading={decideOffer.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          薪资由部门负责人决定。请为候选人「{approveTarget?.application.candidateName ?? ''}」填写最终薪资
          {approveTarget?.offer.salary ? `（HR 建议：${approveTarget.offer.salary}）` : ''}。
        </Typography.Paragraph>
        <Form form={approveForm} layout="vertical">
          <Form.Item
            name="salary"
            label="最终薪资（千元/月）"
            rules={[{ required: true, whitespace: true, message: '请填写最终薪资' }]}
          >
            <Input placeholder="如：25" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 驳回弹窗 */}
      <Modal
        title="驳回 Offer 审批"
        open={!!rejectTarget}
        onOk={confirmReject}
        onCancel={() => setRejectTarget(null)}
        okText="确认驳回"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        confirmLoading={decideOffer.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          确认驳回候选人「{rejectTarget?.application.candidateName ?? ''}」的 Offer？驳回后候选人将回到部门负责人面环节。
        </Typography.Paragraph>
        <Form form={rejectForm} layout="vertical">
          <Form.Item
            name="reason"
            label="驳回原因"
            rules={[{ required: true, whitespace: true, message: '请填写驳回原因' }]}
          >
            <Input.TextArea rows={3} placeholder="请填写驳回原因（必填）" maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
}
