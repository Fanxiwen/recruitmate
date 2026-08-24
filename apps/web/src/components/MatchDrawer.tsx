import type { ApplicationEvent, ApplicationInternal, ApplicationStage, Offer } from '@recruitmate/shared-types';
import { STAGE_LABELS } from '@recruitmate/shared-types';
import {
  Alert,
  Button,
  Card,
  Collapse,
  Descriptions,
  Divider,
  Drawer,
  Empty,
  Form,
  Input,
  List,
  Modal,
  Progress,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Timeline,
  Typography,
  message,
} from 'antd';
import {
  CheckCircleFilled,
  DownloadOutlined,
  FileTextOutlined,
  WarningFilled,
} from '@ant-design/icons';
import { useState } from 'react';
import type { ColumnsType } from 'antd/es/table';
import type { HardCheck } from '@recruitmate/shared-types';
import { api } from '../lib/api';
import { formatDateTime, STAGE_COLORS } from '../lib/format';
import { errorMessage, useApplicationDetail, useDecideOffer, useSetStage, type ApplicationDetail } from '../hooks/useApi';
import { stageOptionsFor } from '../lib/stages';
import { useAuthStore } from '../stores/auth';

/** 时间线动作文案（action → 展示文案） */
const EVENT_ACTION_LABELS: Record<ApplicationEvent['action'], string> = {
  stage_change: '流转阶段',
  offer_request: '发起 Offer 审批',
  offer_approve: 'Offer 审批通过',
  offer_reject: 'Offer 审批驳回',
  feedback: '记录面试评价',
};

/** Offer 审批状态展示 */
const OFFER_STATUS_LABELS: Record<Offer['status'], string> = {
  pending: '审批中',
  approved: '已通过',
  rejected: '已驳回',
};

const OFFER_STATUS_COLORS: Record<Offer['status'], string> = {
  pending: 'processing',
  approved: 'success',
  rejected: 'error',
};

/** 时间线事件文案：stage_change 带 from → to，其余动作固定文案 */
function eventText(ev: ApplicationEvent): string {
  if (ev.action === 'stage_change') {
    const from = ev.fromStage
      ? `${STAGE_LABELS[ev.fromStage as ApplicationStage] ?? ev.fromStage} → `
      : '';
    const to = STAGE_LABELS[ev.toStage as ApplicationStage] ?? ev.toStage;
    return `流转阶段：${from}${to}`;
  }
  return EVENT_ACTION_LABELS[ev.action];
}

interface MatchDrawerProps {
  open: boolean;
  application: ApplicationInternal | null;
  onClose: () => void;
  /** 阶段流转（父级负责 mutation 与数据刷新；转 rejected 请走抽屉内原因弹窗） */
  onStageChange: (app: ApplicationInternal, stage: ApplicationStage, reason?: string) => void;
}

/** 硬性条件检查表列 */
const HARD_CHECK_COLUMNS: ColumnsType<HardCheck> = [
  {
    title: '检查项',
    dataIndex: 'name',
  },
  {
    title: '结果',
    dataIndex: 'pass',
    width: 90,
    render: (pass: boolean) =>
      pass ? <Tag color="green">通过</Tag> : <Tag color="red">未通过</Tag>,
  },
  {
    title: '说明',
    dataIndex: 'detail',
  },
];

/** 分项分进度条 */
function ScoreBar({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
        <span>{label}</span>
        <span style={{ fontWeight: 600 }}>{value}</span>
      </div>
      <Progress percent={value} strokeColor={color} showInfo={false} size="small" />
    </div>
  );
}

/**
 * 候选人详情 Drawer（宽 640）：
 * 联系信息 + 阶段流转 / AI 匹配报告（综合分、分项分、strengths/gaps/risk/summary、
 * 硬性条件逐项、规则模式 Tag、评分时间）/ 简历结构化信息（教育 Timeline、工作 List、
 * 技能 Tag、解析摘要）/ 原文预览 + 下载原件。
 */
export function MatchDrawer({ open, application, onClose, onStageChange }: MatchDrawerProps) {
  const [downloadLoading, setDownloadLoading] = useState(false);
  const user = useAuthStore((s) => s.user);

  const setStageMutation = useSetStage();
  const decideOfferMutation = useDecideOffer();

  // ===== 阶段流转 → 淘汰原因弹窗 =====
  const [stageRejectOpen, setStageRejectOpen] = useState(false);
  const [stageRejectForm] = Form.useForm();

  // ===== Offer 驳回原因弹窗 =====
  const [offerRejectOpen, setOfferRejectOpen] = useState(false);
  const [offerRejectForm] = Form.useForm();
  // ===== Offer 审批通过弹窗（最终薪资由部门负责人确定） =====
  const [offerApproveOpen, setOfferApproveOpen] = useState(false);
  const [offerApproveForm] = Form.useForm();

  // 打开时拉取最新详情（含简历原文 resumeText，后端若未返回则回退为列表数据）
  const { data: detail } = useApplicationDetail(application?.id ?? '', open);
  const app = (detail ?? application) as ApplicationDetail | null;

  const handleDownload = async () => {
    if (!app) return;
    setDownloadLoading(true);
    try {
      // 鉴权流式下载：后端转发文件内容（预签名 URL 会暴露 MinIO 内网主机名，浏览器不可达）
      const { blob, filename } = await api.downloadResume(app.id);
      const objectUrl = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = objectUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(objectUrl);
    } catch (err) {
      message.error(errorMessage(err, '下载简历失败'));
    } finally {
      setDownloadLoading(false);
    }
  };

  /** 阶段下拉选择：转 rejected 必须先填原因，其余直接流转 */
  const handleStageSelect = (nextStage: ApplicationStage) => {
    if (!app) return;
    if (nextStage === 'rejected') {
      stageRejectForm.resetFields();
      setStageRejectOpen(true);
      return;
    }
    onStageChange(app, nextStage);
  };

  /** 确认淘汰（原因必填） */
  const confirmStageReject = async () => {
    if (!app) return;
    let reason = '';
    try {
      const values = await stageRejectForm.validateFields();
      reason = values.reason as string;
    } catch {
      return; // 表单校验未通过，Modal 内已有必填提示
    }
    try {
      await setStageMutation.mutateAsync({ id: app.id, stage: 'rejected', reason });
      message.success('已淘汰候选人');
      setStageRejectOpen(false);
    } catch (err) {
      message.error(errorMessage(err, '淘汰失败'));
    }
  };

  /** Offer 审批人判断（简化）：admin / 部门负责人，且不能是发起人本人 */
  const offer = app?.offer ?? null;
  const isOfferApprover =
    !!offer &&
    offer.status === 'pending' &&
    !!user &&
    (user.role === 'admin' || user.role === 'hiring_manager') &&
    offer.requestedByName !== user.name;

  /** Offer 审批通过（打开定薪弹窗：薪资由部门负责人确定） */
  const openOfferApprove = () => {
    offerApproveForm.setFieldsValue({ salary: offer?.salary ?? '' });
    setOfferApproveOpen(true);
  };

  const confirmOfferApprove = async () => {
    if (!app) return;
    let salary = '';
    try {
      const values = await offerApproveForm.validateFields();
      salary = values.salary as string;
    } catch {
      return; // 表单校验未通过，Modal 内已有必填提示
    }
    try {
      await decideOfferMutation.mutateAsync({ id: app.id, decision: 'approve', salary });
      message.success('Offer 审批已通过');
      setOfferApproveOpen(false);
    } catch (err) {
      message.error(errorMessage(err, '审批失败'));
    }
  };

  /** Offer 审批驳回（原因必填） */
  const confirmOfferReject = async () => {
    if (!app) return;
    let reason = '';
    try {
      const values = await offerRejectForm.validateFields();
      reason = values.reason as string;
    } catch {
      return; // 表单校验未通过
    }
    try {
      await decideOfferMutation.mutateAsync({ id: app.id, decision: 'reject', reason });
      message.success('Offer 已驳回');
      setOfferRejectOpen(false);
    } catch (err) {
      message.error(errorMessage(err, '审批失败'));
    }
  };

  const md = app?.matchDetail;
  const pr = app?.parsedResume;

  // 嵌套数组防御性归一化：后端契约保证 []，但旧缓存/异常数据可能为 null
  const strengths = md?.strengths ?? [];
  const gaps = md?.gaps ?? [];
  const hardChecks = md?.hardChecks ?? [];
  const education = pr?.education ?? [];
  const workExperience = pr?.workExperience ?? [];
  const skills = pr?.skills ?? [];

  return (
    <Drawer
      title={app ? `${app.candidateName} · 候选人详情` : '候选人详情'}
      width={640}
      open={open}
      onClose={onClose}
      extra={
        app ? (
          <Button
            icon={<DownloadOutlined />}
            disabled={!app.hasResumeFile}
            loading={downloadLoading}
            onClick={handleDownload}
          >
            下载简历原件
          </Button>
        ) : null
      }
    >
      {app && (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          {/* ===== 联系信息 + 阶段流转 ===== */}
          <section>
            <Typography.Title level={5}>候选人信息</Typography.Title>
            <Descriptions
              size="small"
              column={2}
              items={[
                { key: 'name', label: '姓名', children: app.candidateName },
                { key: 'email', label: '邮箱', children: app.email },
                { key: 'phone', label: '电话', children: app.phone || '—' },
                { key: 'source', label: '投递来源', children: app.source || '—' },
                { key: 'time', label: '投递时间', children: formatDateTime(app.submittedAt) },
                {
                  key: 'stage',
                  label: '当前阶段',
                  children: <Tag color={STAGE_COLORS[app.stage]}>{STAGE_LABELS[app.stage]}</Tag>,
                },
              ]}
            />
            <Space style={{ marginTop: 8 }}>
              <span>阶段流转：</span>
              <Select
                style={{ width: 160 }}
                value={app.stage}
                options={stageOptionsFor(app.stage)}
                onChange={handleStageSelect}
              />
            </Space>
          </section>

          <Divider style={{ margin: 0 }} />

          {/* ===== 流程时间线（含淘汰原因 / 面试评价 / Offer 审批单） ===== */}
          <section>
            <Typography.Title level={5}>流程时间线</Typography.Title>
            {!app.events || app.events.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无流程记录" />
            ) : (
              <Timeline
                style={{ marginTop: 8 }}
                items={app.events.map((ev) => ({
                  key: ev.id,
                  children: (
                    <Space direction="vertical" size={2} style={{ width: '100%' }}>
                      <span>
                        <Typography.Text strong>{ev.actorName}</Typography.Text>
                        {' · '}
                        {eventText(ev)}
                      </span>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {formatDateTime(ev.createdAt)}
                      </Typography.Text>
                      {ev.reason && (
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          原因：{ev.reason}
                        </Typography.Text>
                      )}
                    </Space>
                  ),
                }))}
              />
            )}
            {app.rejectReason && (
              <Alert
                type="error"
                showIcon
                style={{ marginTop: 12 }}
                message="淘汰原因"
                description={app.rejectReason}
              />
            )}
            {app.interviewFeedback && (
              <Alert
                type="info"
                showIcon
                style={{ marginTop: 12 }}
                message="面试评价"
                description={app.interviewFeedback}
              />
            )}
          </section>

          {/* ===== Offer 审批单 ===== */}
          {offer && (
            <section>
              <Typography.Title level={5}>Offer 审批单</Typography.Title>
              <Card size="small">
                <Descriptions
                  size="small"
                  column={1}
                  items={[
                    { key: 'salary', label: '薪资（千元/月）', children: offer.salary || '—' },
                    { key: 'joinDate', label: '入职时间', children: offer.joinDate || '—' },
                    { key: 'note', label: '备注', children: offer.note || '—' },
                    {
                      key: 'status',
                      label: '状态',
                      children: <Tag color={OFFER_STATUS_COLORS[offer.status]}>{OFFER_STATUS_LABELS[offer.status]}</Tag>,
                    },
                    { key: 'requestedBy', label: '发起人', children: offer.requestedByName },
                    { key: 'decidedBy', label: '决定人', children: offer.decidedByName || '—' },
                  ]}
                />
                {isOfferApprover && (
                  <Space style={{ marginTop: 12 }}>
                    <Button
                      type="primary"
                      loading={decideOfferMutation.isPending}
                      onClick={openOfferApprove}
                    >
                      通过并确定薪资
                    </Button>
                    <Button
                      danger
                      loading={decideOfferMutation.isPending}
                      onClick={() => {
                        offerRejectForm.resetFields();
                        setOfferRejectOpen(true);
                      }}
                    >
                      驳回
                    </Button>
                  </Space>
                )}
              </Card>
            </section>
          )}

          <Divider style={{ margin: 0 }} />

          {/* ===== AI 匹配报告 ===== */}
          <section>
            <Typography.Title level={5}>AI 匹配报告</Typography.Title>
            {!md ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="AI 评分处理中，请稍后刷新查看"
              />
            ) : (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                <Space align="start" size={32} wrap>
                  <Statistic
                    title="综合匹配分"
                    value={md.score}
                    valueStyle={{ fontSize: 36, color: '#1677ff', fontWeight: 700 }}
                  />
                  <Space direction="vertical" size={4}>
                    {md.engine === 'rule' && <Tag color="default">规则模式</Tag>}
                    {md.model && <Tag color="geekblue">{md.model}</Tag>}
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      评分时间：{formatDateTime(md.scoredAt)}
                    </Typography.Text>
                  </Space>
                </Space>

                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <ScoreBar label="结构化规则分" value={md.ruleScore} color="#1677ff" />
                  {md.semanticScore != null && (
                    <ScoreBar label="语义相似度" value={md.semanticScore} color="#722ed1" />
                  )}
                  {md.llmScore != null && <ScoreBar label="LLM 评委分" value={md.llmScore} color="#13c2c2" />}
                </Space>

                {strengths.length > 0 && (
                  <List
                    size="small"
                    header={
                      <Typography.Text strong style={{ color: '#389e0d' }}>
                        匹配优势
                      </Typography.Text>
                    }
                    dataSource={strengths}
                    renderItem={(item) => (
                      <List.Item style={{ border: 'none', padding: '2px 0' }}>
                        <Space>
                          <CheckCircleFilled style={{ color: '#52c41a' }} />
                          <span>{item}</span>
                        </Space>
                      </List.Item>
                    )}
                  />
                )}

                {gaps.length > 0 && (
                  <List
                    size="small"
                    header={
                      <Typography.Text strong style={{ color: '#d46b08' }}>
                        待改进点
                      </Typography.Text>
                    }
                    dataSource={gaps}
                    renderItem={(item) => (
                      <List.Item style={{ border: 'none', padding: '2px 0' }}>
                        <Space>
                          <WarningFilled style={{ color: '#fa8c16' }} />
                          <span>{item}</span>
                        </Space>
                      </List.Item>
                    )}
                  />
                )}

                {md.risk && (
                  <Alert
                    type="error"
                    showIcon
                    message="风险提示"
                    description={<Typography.Text type="danger">{md.risk}</Typography.Text>}
                  />
                )}

                {md.summary && (
                  <div>
                    <Typography.Text strong>综合评语</Typography.Text>
                    <Typography.Paragraph style={{ marginTop: 4 }}>{md.summary}</Typography.Paragraph>
                  </div>
                )}

                {hardChecks.length > 0 && (
                  <Table<HardCheck>
                    size="small"
                    rowKey="name"
                    columns={HARD_CHECK_COLUMNS}
                    dataSource={hardChecks}
                    pagination={false}
                  />
                )}
              </Space>
            )}
          </section>

          <Divider style={{ margin: 0 }} />

          {/* ===== 简历结构化信息 ===== */}
          <section>
            <Typography.Title level={5}>简历信息</Typography.Title>
            {!pr ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  app.parseFailed ? '简历解析失败，请查看原文' : '简历解析中，请稍后刷新'
                }
              />
            ) : (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                {education.length > 0 && (
                  <div>
                    <Typography.Text strong>教育经历</Typography.Text>
                    <Timeline
                      style={{ marginTop: 8 }}
                      items={education.map((edu, i) => ({
                        key: i,
                        children: (
                          <span>
                            {edu.school}
                            {edu.major && ` · ${edu.major}`}
                            {edu.level && ` · ${edu.level}`}
                            {edu.endYear != null && ` · ${edu.endYear} 年毕业`}
                          </span>
                        ),
                      }))}
                    />
                  </div>
                )}

                {workExperience.length > 0 && (
                  <div>
                    <Typography.Text strong>工作经历</Typography.Text>
                    <List
                      size="small"
                      style={{ marginTop: 4 }}
                      dataSource={workExperience}
                      renderItem={(exp, i) => (
                        <List.Item key={i} style={{ paddingLeft: 0 }}>
                          <Space direction="vertical" size={0} style={{ width: '100%' }}>
                            <span>
                              <Typography.Text strong>{exp.title}</Typography.Text>
                              {exp.company && ` · ${exp.company}`}
                            </span>
                            {exp.startDate && (
                              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                                {exp.startDate}
                                {exp.endDate ? ` ~ ${exp.endDate}` : ' 至今'}
                              </Typography.Text>
                            )}
                            {exp.description && (
                              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                                {exp.description}
                              </Typography.Paragraph>
                            )}
                          </Space>
                        </List.Item>
                      )}
                    />
                  </div>
                )}

                {skills.length > 0 && (
                  <div>
                    <Typography.Text strong>技能</Typography.Text>
                    <div style={{ marginTop: 8 }}>
                      <Space size={[4, 4]} wrap>
                        {skills.map((s) => (
                          <Tag key={s} color="blue">
                            {s}
                          </Tag>
                        ))}
                      </Space>
                    </div>
                  </div>
                )}

                {pr.summary && (
                  <div>
                    <Typography.Text strong>AI 解析摘要</Typography.Text>
                    <Typography.Paragraph style={{ marginTop: 4 }}>{pr.summary}</Typography.Paragraph>
                  </div>
                )}
              </Space>
            )}
          </section>

          <Divider style={{ margin: 0 }} />

          {/* ===== 原文预览 ===== */}
          <section>
            <Typography.Title level={5}>简历原文</Typography.Title>
            <Collapse
              items={[
                {
                  key: 'text',
                  label: (
                    <Space>
                      <FileTextOutlined />
                      查看简历原文
                    </Space>
                  ),
                  children: app.resumeText ? (
                    <Typography.Paragraph
                      style={{ whiteSpace: 'pre-wrap', marginBottom: 0, maxHeight: 320, overflow: 'auto' }}
                    >
                      {app.resumeText}
                    </Typography.Paragraph>
                  ) : (
                    <Typography.Text type="secondary">
                      {app.hasResumeFile ? '暂无提取文本，可点击右上角下载原件查看。' : '该投递未提供简历原文。'}
                    </Typography.Text>
                  ),
                },
              ]}
            />
          </section>
        </Space>
      )}

      {/* 淘汰原因弹窗（阶段流转 → rejected 必填原因） */}
      <Modal
        title="淘汰候选人"
        open={stageRejectOpen}
        onOk={confirmStageReject}
        onCancel={() => setStageRejectOpen(false)}
        okText="确认淘汰"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        confirmLoading={setStageMutation.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          确认将候选人「{app?.candidateName ?? ''}」淘汰？淘汰原因将记录在流程时间线中。
        </Typography.Paragraph>
        <Form form={stageRejectForm} layout="vertical">
          <Form.Item
            name="reason"
            label="淘汰原因"
            rules={[{ required: true, whitespace: true, message: '请填写淘汰原因' }]}
          >
            <Input.TextArea rows={3} placeholder="请填写淘汰原因（必填）" maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>

      {/* Offer 驳回原因弹窗（必填） */}
      <Modal
        title="驳回 Offer 审批"
        open={offerRejectOpen}
        onOk={confirmOfferReject}
        onCancel={() => setOfferRejectOpen(false)}
        okText="确认驳回"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        confirmLoading={decideOfferMutation.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          确认驳回候选人「{app?.candidateName ?? ''}」的 Offer 审批？驳回后候选人将回到部门负责人面环节。
        </Typography.Paragraph>
        <Form form={offerRejectForm} layout="vertical">
          <Form.Item
            name="reason"
            label="驳回原因"
            rules={[{ required: true, whitespace: true, message: '请填写驳回原因' }]}
          >
            <Input.TextArea rows={3} placeholder="请填写驳回原因（必填）" maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>

      {/* Offer 审批通过弹窗（最终薪资由部门负责人确定，必填） */}
      <Modal
        title="通过 Offer 审批 · 确定最终薪资"
        open={offerApproveOpen}
        onOk={confirmOfferApprove}
        onCancel={() => setOfferApproveOpen(false)}
        okText="通过并确定薪资"
        cancelText="取消"
        confirmLoading={decideOfferMutation.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          薪资由部门负责人决定。请在下方填写候选人「{app?.candidateName ?? ''}」的最终薪资
          {offer?.salary ? `（HR 建议：${offer.salary}）` : ''}。
        </Typography.Paragraph>
        <Form form={offerApproveForm} layout="vertical">
          <Form.Item
            name="salary"
            label="最终薪资（千元/月）"
            rules={[{ required: true, whitespace: true, message: '请填写最终薪资' }]}
          >
            <Input placeholder="如：25" />
          </Form.Item>
        </Form>
      </Modal>
    </Drawer>
  );
}
