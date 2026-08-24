import type { ApplicationInternal, ApplicationStage } from '@recruitmate/shared-types';
import { STAGE_TRANSITIONS } from '@recruitmate/shared-types';
import { Button, Form, Input, Modal, Progress, Select, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useMemo, useRef, useState } from 'react';
import type { Key } from 'react';
import { errorMessage, useCreateOffer, useSetStage } from '../hooks/useApi';
import { formatDateTime } from '../lib/format';
import { stageOptionsFor } from '../lib/stages';
import { useAuthStore } from '../stores/auth';

interface ApplicationTableProps {
  data: ApplicationInternal[] | undefined;
  loading: boolean;
  total: number;
  page: number;
  pageSize: number;
  selectedKeys: Key[];
  onSelectionChange: (keys: Key[]) => void;
  onPageChange: (page: number, pageSize: number) => void;
  /** 行内阶段流转（转 rejected 请走淘汰弹窗，本回调只处理非淘汰流转） */
  onStageChange: (app: ApplicationInternal, stage: ApplicationStage, reason?: string) => void;
  /** 打开详情 Drawer */
  onOpenDetail: (app: ApplicationInternal) => void;
}

/**
 * 候选人表格（岗位详情页候选人 Tab 复用组件）。
 * 特性：
 * - 匹配度圆环 + 硬性条件 Tag；评分中显示「评分中…」；
 * - 行内阶段 Select 按 STAGE_TRANSITIONS 过滤合法流转，直接流转；
 * - 淘汰（键盘 x / 操作列按钮）弹 Modal，必须填写淘汰原因；
 * - HR/管理员在面试中阶段可发起 Offer 审批（薪资/入职时间/备注）；
 * - 键盘快捷键（容器获得焦点时）：j/k 上下移动高亮行、Enter 打开 Drawer、x 淘汰当前行。
 */
export function ApplicationTable({
  data,
  loading,
  total,
  page,
  pageSize,
  selectedKeys,
  onSelectionChange,
  onPageChange,
  onStageChange,
  onOpenDetail,
}: ApplicationTableProps) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [highlight, setHighlight] = useState(0);
  const user = useAuthStore((s) => s.user);

  const setStageMutation = useSetStage();
  const createOfferMutation = useCreateOffer();

  // ===== 淘汰弹窗 =====
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectTarget, setRejectTarget] = useState<ApplicationInternal | null>(null);
  const [rejectForm] = Form.useForm();

  // ===== 发起 Offer 弹窗 =====
  const [offerOpen, setOfferOpen] = useState(false);
  const [offerTarget, setOfferTarget] = useState<ApplicationInternal | null>(null);
  const [offerForm] = Form.useForm();

  const rows = useMemo(() => data ?? [], [data]);

  // 数据变化时把高亮行收敛到有效范围内
  const safeHighlight = Math.min(highlight, Math.max(rows.length - 1, 0));
  const highlighted = rows[safeHighlight] ?? null;

  /** 当前阶段是否允许流转到 rejected（决定是否展示「淘汰」按钮） */
  const canReject = (stage: ApplicationStage) => STAGE_TRANSITIONS[stage].includes('rejected');

  /** 当前用户是否可发起 Offer（hr / admin，且阶段为部门负责人面） */
  const canCreateOffer = (app: ApplicationInternal) =>
    (user?.role === 'hr' || user?.role === 'admin') && app.stage === 'manager_interview';

  const openReject = (app: ApplicationInternal) => {
    setRejectTarget(app);
    rejectForm.resetFields();
    setRejectOpen(true);
  };

  const confirmReject = async () => {
    if (!rejectTarget) return;
    let reason = '';
    try {
      const values = await rejectForm.validateFields();
      reason = values.reason as string;
    } catch {
      return; // 表单校验未通过，Modal 内已有必填提示
    }
    try {
      await setStageMutation.mutateAsync({ id: rejectTarget.id, stage: 'rejected', reason });
      message.success('已淘汰候选人');
      setRejectOpen(false);
    } catch (err) {
      message.error(errorMessage(err, '淘汰失败'));
    }
  };

  const openOffer = (app: ApplicationInternal) => {
    setOfferTarget(app);
    offerForm.resetFields();
    setOfferOpen(true);
  };

  const confirmOffer = async () => {
    if (!offerTarget) return;
    let values: { salary?: string; joinDate?: string; note?: string } = {};
    try {
      values = (await offerForm.validateFields()) as { salary?: string; joinDate?: string; note?: string };
    } catch {
      return; // 表单校验未通过
    }
    try {
      await createOfferMutation.mutateAsync({ id: offerTarget.id, body: values });
      message.success('Offer 审批已提交');
      setOfferOpen(false);
    } catch (err) {
      message.error(errorMessage(err, '发起 Offer 失败'));
    }
  };

  const columns: ColumnsType<ApplicationInternal> = [
    {
      title: '匹配度',
      dataIndex: 'matchScore',
      width: 170,
      render: (_, app) => {
        if (app.matchScore == null) {
          return <Tag color="processing">评分中…</Tag>;
        }
        return (
          <Space size={4} direction="vertical">
            <Space size={6}>
              <Progress
                type="circle"
                size={34}
                percent={app.matchScore}
                strokeWidth={9}
                format={(p) => `${p ?? 0}`}
              />
              <span style={{ fontWeight: 600 }}>{app.matchScore}</span>
            </Space>
            {app.hardPass && <Tag color="orange">不满足硬性要求</Tag>}
          </Space>
        );
      },
    },
    {
      title: '候选人',
      dataIndex: 'candidateName',
      width: 220,
      render: (_, app) => (
        <Space direction="vertical" size={0}>
          <span>{app.candidateName}</span>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {app.email}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '技能命中预览',
      dataIndex: ['parsedResume', 'skills'],
      width: 240,
      render: (_, app) => {
        if (app.parsedResume == null) {
          return app.parseFailed ? <Tag color="red">解析失败</Tag> : <Tag color="processing">解析中…</Tag>;
        }
        const skills = app.parsedResume?.skills ?? [];
        if (skills.length === 0) return <Typography.Text type="secondary">—</Typography.Text>;
        return (
          <Space size={[4, 4]} wrap>
            {skills.slice(0, 3).map((s) => (
              <Tag key={s}>{s}</Tag>
            ))}
            {skills.length > 3 && <Tag>+{skills.length - 3}</Tag>}
          </Space>
        );
      },
    },
    {
      title: '阶段',
      dataIndex: 'stage',
      width: 130,
      render: (_, app) => (
        <Select
          size="small"
          style={{ width: 110 }}
          value={app.stage}
          options={stageOptionsFor(app.stage)}
          onClick={(e) => e.stopPropagation()}
          onChange={(v) => {
            // 转 rejected 必须填写原因，统一走淘汰弹窗
            if (v === 'rejected') {
              openReject(app);
            } else {
              onStageChange(app, v);
            }
          }}
        />
      ),
    },
    {
      title: '投递时间',
      dataIndex: 'submittedAt',
      width: 150,
      render: (v: string) => formatDateTime(v),
    },
    {
      title: '操作',
      key: 'action',
      width: 190,
      fixed: 'right',
      render: (_, app) => (
        <Space size={0}>
          <Button
            type="link"
            size="small"
            onClick={(e) => {
              e.stopPropagation();
              onOpenDetail(app);
            }}
          >
            详情
          </Button>
          {canReject(app.stage) && (
            <Button
              type="link"
              size="small"
              danger
              onClick={(e) => {
                e.stopPropagation();
                openReject(app);
              }}
            >
              淘汰
            </Button>
          )}
          {canCreateOffer(app) && (
            <Button
              type="link"
              size="small"
              onClick={(e) => {
                e.stopPropagation();
                openOffer(app);
              }}
            >
              发起 Offer
            </Button>
          )}
        </Space>
      ),
    },
  ];

  /** 键盘快捷键：仅当事件源不在输入类控件上时生效 */
  const handleKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const target = e.target as HTMLElement;
    if (target.closest('input, textarea, .ant-select, button, a')) return;
    if (rows.length === 0) return;
    if (e.key === 'j' || e.key === 'ArrowDown') {
      e.preventDefault();
      setHighlight((h) => Math.min(h + 1, rows.length - 1));
    } else if (e.key === 'k' || e.key === 'ArrowUp') {
      e.preventDefault();
      setHighlight((h) => Math.max(h - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (highlighted) onOpenDetail(highlighted);
    } else if (e.key === 'x' || e.key === 'X') {
      e.preventDefault();
      if (highlighted) openReject(highlighted);
    }
  };

  return (
    <>
      <div
        ref={wrapRef}
        tabIndex={0}
        className="application-table"
        onKeyDown={handleKeyDown}
        onMouseDown={(e) => {
          // 为键盘快捷键获取焦点，但禁止浏览器 focus 滚动：
          // 否则 sticky 列随滚动重定位，mousedown/mouseup 之间元素位移，
          // 导致「详情」等按钮第一次点击失效。
          e.preventDefault();
          wrapRef.current?.focus({ preventScroll: true });
        }}
      >
        <Table<ApplicationInternal>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          columns={columns}
          scroll={{ x: 1100 }}
          rowSelection={{
            selectedRowKeys: selectedKeys,
            onChange: onSelectionChange,
          }}
          onRow={(record, index) => ({
            onClick: (e) => {
              const target = e.target as HTMLElement;
              // 点击行内控件（选择框/下拉/按钮）时不打开 Drawer
              if (target.closest('.ant-select, .ant-checkbox, button, a')) return;
              onOpenDetail(record);
            },
            className: index === safeHighlight ? 'application-table__row--highlight' : undefined,
          })}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50, 100],
            showTotal: (t) => `共 ${t} 条`,
            onChange: onPageChange,
          }}
        />
      </div>

      {/* 淘汰原因弹窗（必填） */}
      <Modal
        title="淘汰候选人"
        open={rejectOpen}
        onOk={confirmReject}
        onCancel={() => setRejectOpen(false)}
        okText="确认淘汰"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        confirmLoading={setStageMutation.isPending}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          确认将候选人「{rejectTarget?.candidateName ?? ''}」淘汰？淘汰原因将记录在流程时间线中。
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

      {/* 发起 Offer 审批弹窗 */}
      <Modal
        title={`发起 Offer 审批 · ${offerTarget?.candidateName ?? ''}`}
        open={offerOpen}
        onOk={confirmOffer}
        onCancel={() => setOfferOpen(false)}
        okText="提交审批"
        cancelText="取消"
        confirmLoading={createOfferMutation.isPending}
        destroyOnHidden
      >
        <Form form={offerForm} layout="vertical">
          <Form.Item name="salary" label="建议薪资（千元/月）" extra="最终薪资由部门负责人审批时确定">
            <Input placeholder="如 20-25（可选）" />
          </Form.Item>
          <Form.Item name="joinDate" label="入职时间">
            <Input placeholder="如 2025-03-01（可选）" />
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input.TextArea rows={3} placeholder="Offer 补充说明（可选）" maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
