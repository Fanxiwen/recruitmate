import { useMemo, useState } from 'react';
import {
  Badge,
  Button,
  Card,
  DatePicker,
  Empty,
  Form,
  Input,
  List,
  Modal,
  Radio,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import { Link } from 'react-router-dom';
import type { ApprovalOfferItem, InterviewRound, InterviewTodoItem } from '@recruitmate/shared-types';
import { JobActions } from '../components/JobActions';
import { OfferCreateModal } from '../components/OfferCreateModal';
import {
  errorMessage,
  useApprovalOffers,
  useCompleteInterview,
  useDecideOffer,
  useInterviewTodos,
  useJobs,
  useScheduleInterview,
  useSetStage,
} from '../hooks/useApi';
import { formatDateTime } from '../lib/format';
import { useAuthStore } from '../stores/auth';

type TodoKind = InterviewTodoItem['kind'];

/** 待办分组元信息：标题 / 说明 / 是否显示面试时间 / 关联面试轮次（用于安排与评价动作） */
const TODO_GROUP_META: Record<TodoKind, { title: string; hint: string; showTime?: boolean; round?: InterviewRound }> = {
  screen: { title: '待初筛', hint: '新投递简历，先初筛后进入面试流程' },
  schedule_hr: { title: '待安排 HR 面', hint: '初筛已通过，安排第一轮面试', round: 'hr' },
  review_hr: { title: 'HR 面待评价', hint: '面试已完成，填写评价与结论', showTime: true, round: 'hr' },
  schedule_manager: { title: '待安排负责人面', hint: 'HR 面已通过，安排第二轮面试', round: 'manager' },
  review_manager: { title: '负责人面待评价', hint: '面试已完成，填写评价与结论', showTime: true, round: 'manager' },
  offer_ready: { title: '待发起 Offer', hint: '负责人面已通过，发起 Offer 审批' },
};

/** 分组展示顺序（与后端 kind 语义一致） */
const TODO_KIND_ORDER: TodoKind[] = [
  'screen',
  'schedule_hr',
  'review_hr',
  'schedule_manager',
  'review_manager',
  'offer_ready',
];

/**
 * 我的待办（/approvals）：
 *  - 面试待办区：/internal/todos/interviews 按 kind 分组（后端已按角色返回），组内快捷操作
 *    （初筛通过/淘汰、安排面试、面试评价、发起 Offer）；
 *  - 审批区：岗位发布审批 + Offer 审批（原审批中心内容保留）。
 */
export function ToDoPage() {
  const user = useAuthStore((s) => s.user);
  const todos = useInterviewTodos();

  // ===== 面试待办变更 =====
  const setStage = useSetStage();
  const schedule = useScheduleInterview();
  const complete = useCompleteInterview();

  // 淘汰弹窗（screen 组）
  const [rejectTarget, setRejectTarget] = useState<InterviewTodoItem | null>(null);
  const [rejectForm] = Form.useForm();
  // 安排面试弹窗（schedule_hr / schedule_manager）
  const [scheduleTarget, setScheduleTarget] = useState<InterviewTodoItem | null>(null);
  const [scheduleForm] = Form.useForm();
  // 面试评价弹窗（review_hr / review_manager）
  const [completeTarget, setCompleteTarget] = useState<InterviewTodoItem | null>(null);
  const [completeForm] = Form.useForm();
  // 发起 Offer 弹窗（offer_ready）
  const [offerTarget, setOfferTarget] = useState<InterviewTodoItem | null>(null);

  const groups = useMemo(() => {
    const map = new Map<TodoKind, InterviewTodoItem[]>();
    for (const kind of TODO_KIND_ORDER) map.set(kind, []);
    for (const item of todos.data?.items ?? []) {
      const list = map.get(item.kind);
      if (list) list.push(item);
    }
    return TODO_KIND_ORDER.map((kind) => ({ kind, items: map.get(kind) ?? [] })).filter(
      (g) => g.items.length > 0,
    );
  }, [todos.data]);
  const totalTodos = todos.data?.items?.length ?? 0;

  // ===== 审批区（岗位发布 + Offer 审批） =====
  const jobsPending = useJobs({ status: 'pending', page: 1, pageSize: 50 });
  const offersPending = useApprovalOffers(1, 50);
  const decideOffer = useDecideOffer();
  const [approveTarget, setApproveTarget] = useState<ApprovalOfferItem | null>(null);
  const [rejectOfferTarget, setRejectOfferTarget] = useState<ApprovalOfferItem | null>(null);
  const [approveForm] = Form.useForm();
  const [rejectOfferForm] = Form.useForm();

  const canDecide = (item: ApprovalOfferItem) =>
    !!user &&
    (user.role === 'admin' || user.role === 'hiring_manager') &&
    item.offer.requestedByName !== user.name;

  // ===== 面试待办操作 =====
  const passScreening = async (item: InterviewTodoItem) => {
    try {
      await setStage.mutateAsync({ id: item.application.id, stage: 'screening' });
      message.success('已通过初筛');
    } catch (err) {
      message.error(errorMessage(err, '操作失败'));
    }
  };

  const confirmReject = async () => {
    if (!rejectTarget) return;
    let reason = '';
    try {
      const v = await rejectForm.validateFields();
      reason = v.reason as string;
    } catch {
      return; // 表单校验未通过
    }
    try {
      await setStage.mutateAsync({ id: rejectTarget.application.id, stage: 'rejected', reason });
      message.success('已淘汰候选人');
      setRejectTarget(null);
    } catch (err) {
      message.error(errorMessage(err, '淘汰失败'));
    }
  };

  const confirmSchedule = async () => {
    if (!scheduleTarget) return;
    const round = TODO_GROUP_META[scheduleTarget.kind].round;
    if (!round) return;
    let scheduledAt = '';
    try {
      const v = await scheduleForm.validateFields();
      scheduledAt = (v.scheduledAt as Dayjs).toISOString();
    } catch {
      return; // 表单校验未通过
    }
    try {
      await schedule.mutateAsync({ id: scheduleTarget.application.id, round, scheduledAt });
      message.success('面试已安排');
      setScheduleTarget(null);
    } catch (err) {
      message.error(errorMessage(err, '安排面试失败'));
    }
  };

  const confirmComplete = async () => {
    if (!completeTarget) return;
    const round = TODO_GROUP_META[completeTarget.kind].round;
    if (!round) return;
    let result: 'pass' | 'fail' = 'pass';
    let feedback = '';
    try {
      const v = await completeForm.validateFields();
      result = v.result as 'pass' | 'fail';
      feedback = v.feedback as string;
    } catch {
      return; // 表单校验未通过
    }
    try {
      await complete.mutateAsync({ id: completeTarget.application.id, round, result, feedback });
      message.success('面试评价已提交');
      setCompleteTarget(null);
    } catch (err) {
      message.error(errorMessage(err, '提交面试评价失败'));
    }
  };

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

  const confirmOfferReject = async () => {
    if (!rejectOfferTarget) return;
    let reason = '';
    try {
      const v = await rejectOfferForm.validateFields();
      reason = v.reason as string;
    } catch {
      return;
    }
    try {
      await decideOffer.mutateAsync({ id: rejectOfferTarget.application.id, decision: 'reject', reason });
      message.success('Offer 已驳回');
      setRejectOfferTarget(null);
    } catch (err) {
      message.error(errorMessage(err, '审批失败'));
    }
  };

  /** 组内条目快捷操作（按 kind） */
  const renderActions = (item: InterviewTodoItem) => {
    switch (item.kind) {
      case 'screen':
        return (
          <Space>
            <Button type="primary" size="small" onClick={() => passScreening(item)}>
              通过初筛
            </Button>
            <Button
              danger
              size="small"
              onClick={() => {
                rejectForm.resetFields();
                setRejectTarget(item);
              }}
            >
              淘汰
            </Button>
          </Space>
        );
      case 'schedule_hr':
      case 'schedule_manager':
        return (
          <Button
            type="primary"
            size="small"
            onClick={() => {
              scheduleForm.resetFields();
              setScheduleTarget(item);
            }}
          >
            安排面试
          </Button>
        );
      case 'review_hr':
      case 'review_manager':
        return (
          <Button
            type="primary"
            size="small"
            onClick={() => {
              completeForm.resetFields();
              setCompleteTarget(item);
            }}
          >
            填写评价
          </Button>
        );
      case 'offer_ready':
        return (
          <Button type="primary" size="small" onClick={() => setOfferTarget(item)}>
            发起 Offer
          </Button>
        );
      default:
        return null;
    }
  };

  const pendingJobs = jobsPending.data?.items ?? [];
  const pendingOffers = offersPending.data?.items ?? [];

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      {/* ===== 面试待办统计（各待办数合计） ===== */}
      <Card size="small">
        <Space size={12} align="center" wrap>
          <Typography.Text strong style={{ fontSize: 15 }}>
            面试待办
          </Typography.Text>
          <Badge count={totalTodos} showZero />
          <Typography.Text type="secondary">各待办数合计（接口按角色返回）</Typography.Text>
        </Space>
      </Card>

      {/* ===== 面试待办区（按 kind 分组，组标题带数量 Badge） ===== */}
      {groups.length === 0 ? (
        <Card size="small">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无面试待办" />
        </Card>
      ) : (
        groups.map(({ kind, items }) => {
          const meta = TODO_GROUP_META[kind];
          return (
            <Card
              key={kind}
              size="small"
              title={
                <Space size={8}>
                  <span>{meta.title}</span>
                  <Badge count={items.length} size="small" />
                </Space>
              }
              extra={
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {meta.hint}
                </Typography.Text>
              }
            >
              <List
                dataSource={items}
                renderItem={(item) => (
                  <List.Item actions={[renderActions(item)]}>
                    <List.Item.Meta
                      title={
                        <Space size={8} wrap>
                          <Typography.Text strong>{item.application.candidateName}</Typography.Text>
                          <Tag color="purple">{item.application.jobTitle}</Tag>
                          {item.application.matchScore != null && (
                            <Tag>匹配度 {item.application.matchScore}</Tag>
                          )}
                        </Space>
                      }
                      description={
                        <Space direction="vertical" size={0}>
                          {item.interview && (
                            <span>面试时间：{formatDateTime(item.interview.scheduledAt)}</span>
                          )}
                          <span>投递于 {formatDateTime(item.application.submittedAt)}</span>
                        </Space>
                      }
                    />
                  </List.Item>
                )}
              />
            </Card>
          );
        })
      )}

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
                            rejectOfferForm.resetFields();
                            setRejectOfferTarget(item);
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

      {/* ===== 面试待办：淘汰弹窗 ===== */}
      <Modal
        title={`淘汰候选人「${rejectTarget?.application.candidateName ?? ''}」`}
        open={!!rejectTarget}
        onOk={confirmReject}
        onCancel={() => setRejectTarget(null)}
        okText="确认淘汰"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        confirmLoading={setStage.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          淘汰原因将记录在候选人的流程时间线中。
        </Typography.Paragraph>
        <Form form={rejectForm} layout="vertical">
          <Form.Item
            name="reason"
            label="淘汰原因"
            rules={[{ required: true, whitespace: true, message: '请填写淘汰原因' }]}
          >
            <Input.TextArea rows={3} placeholder="请填写淘汰原因（必填）" maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>

      {/* ===== 面试待办：安排面试弹窗（时间必填） ===== */}
      <Modal
        title={`安排面试 · ${scheduleTarget?.application.candidateName ?? ''}`}
        open={!!scheduleTarget}
        onOk={confirmSchedule}
        onCancel={() => setScheduleTarget(null)}
        okText="确认安排"
        cancelText="取消"
        confirmLoading={schedule.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {scheduleTarget
            ? TODO_GROUP_META[scheduleTarget.kind].round === 'hr'
              ? '安排 HR 初面：确认时间后候选人将进入「HR 初面」阶段。'
              : '安排部门负责人面：需 HR 面已通过，确认时间后候选人将进入「部门负责人面」阶段。'
            : ''}
        </Typography.Paragraph>
        <Form form={scheduleForm} layout="vertical">
          <Form.Item
            name="scheduledAt"
            label="面试时间"
            rules={[{ required: true, message: '请选择面试时间' }]}
          >
            <DatePicker
              showTime
              style={{ width: '100%' }}
              placeholder="选择面试时间（必填）"
              disabledDate={(d) => d.isBefore(new Date(), 'day')}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* ===== 面试待办：评价弹窗（结论 + 评价必填） ===== */}
      <Modal
        title={`面试评价 · ${completeTarget?.application.candidateName ?? ''}`}
        open={!!completeTarget}
        onOk={confirmComplete}
        onCancel={() => setCompleteTarget(null)}
        okText="提交评价"
        cancelText="取消"
        confirmLoading={complete.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          填写「{completeTarget?.application.candidateName ?? ''}」的面试评价与结论：不通过将直接淘汰候选人。
        </Typography.Paragraph>
        <Form form={completeForm} layout="vertical">
          <Form.Item
            name="result"
            label="面试结论"
            rules={[{ required: true, message: '请选择面试结论' }]}
          >
            <Radio.Group>
              <Radio value="pass">通过</Radio>
              <Radio value="fail">不通过</Radio>
            </Radio.Group>
          </Form.Item>
          <Form.Item
            name="feedback"
            label="面试评价"
            rules={[{ required: true, whitespace: true, message: '请填写面试评价' }]}
          >
            <Input.TextArea rows={4} placeholder="请填写面试评价（必填）" maxLength={1000} showCount />
          </Form.Item>
        </Form>
      </Modal>

      {/* ===== 面试待办：发起 Offer 弹窗（offer_ready） ===== */}
      <OfferCreateModal
        open={!!offerTarget}
        application={offerTarget?.application ?? null}
        onClose={() => setOfferTarget(null)}
      />

      {/* ===== Offer 审批通过（定薪）弹窗 ===== */}
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

      {/* ===== Offer 驳回弹窗 ===== */}
      <Modal
        title="驳回 Offer 审批"
        open={!!rejectOfferTarget}
        onOk={confirmOfferReject}
        onCancel={() => setRejectOfferTarget(null)}
        okText="确认驳回"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        confirmLoading={decideOffer.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          确认驳回候选人「{rejectOfferTarget?.application.candidateName ?? ''}」的 Offer？驳回后候选人将回到部门负责人面环节。
        </Typography.Paragraph>
        <Form form={rejectOfferForm} layout="vertical">
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
