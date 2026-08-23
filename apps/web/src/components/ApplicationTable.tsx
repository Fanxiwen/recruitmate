import type { ApplicationInternal, ApplicationStage } from '@recruitmate/shared-types';
import { STAGE_LABELS } from '@recruitmate/shared-types';
import { Button, Popconfirm, Progress, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useMemo, useRef, useState } from 'react';
import type { Key } from 'react';
import { formatDateTime } from '../lib/format';

/** 阶段下拉选项（来自 shared-types 的 STAGE_LABELS） */
const STAGE_OPTIONS = (Object.entries(STAGE_LABELS) as [ApplicationStage, string][]).map(
  ([value, label]) => ({ value, label }),
);

interface ApplicationTableProps {
  data: ApplicationInternal[] | undefined;
  loading: boolean;
  total: number;
  page: number;
  pageSize: number;
  selectedKeys: Key[];
  onSelectionChange: (keys: Key[]) => void;
  onPageChange: (page: number, pageSize: number) => void;
  /** 行内阶段流转 */
  onStageChange: (app: ApplicationInternal, stage: ApplicationStage) => void;
  /** 打开详情 Drawer */
  onOpenDetail: (app: ApplicationInternal) => void;
  /** 淘汰单个候选人（键盘 x 触发，带 Popconfirm 确认） */
  onReject: (app: ApplicationInternal) => void;
}

/**
 * 候选人表格（岗位详情页候选人 Tab 复用组件）。
 * 特性：
 * - 匹配度圆环 + 硬性条件 Tag；评分中显示「评分中…」；
 * - 行内阶段 Select 直接流转；
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
  onReject,
}: ApplicationTableProps) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [highlight, setHighlight] = useState(0);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectTarget, setRejectTarget] = useState<ApplicationInternal | null>(null);

  const rows = useMemo(() => data ?? [], [data]);

  // 数据变化时把高亮行收敛到有效范围内
  const safeHighlight = Math.min(highlight, Math.max(rows.length - 1, 0));
  const highlighted = rows[safeHighlight] ?? null;

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
        const skills = app.parsedResume.skills ?? [];
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
          options={STAGE_OPTIONS}
          onClick={(e) => e.stopPropagation()}
          onChange={(v) => onStageChange(app, v)}
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
      width: 80,
      fixed: 'right',
      render: (_, app) => (
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
      if (highlighted) {
        setRejectTarget(highlighted);
        setRejectOpen(true);
      }
    }
  };

  return (
    <Popconfirm
      title={`确认淘汰候选人「${rejectTarget?.candidateName ?? ''}」？`}
      description="淘汰后可在阶段下拉中恢复为其他阶段。"
      okText="淘汰"
      cancelText="取消"
      okButtonProps={{ danger: true }}
      open={rejectOpen}
      trigger={[]}
      onConfirm={() => {
        if (rejectTarget) onReject(rejectTarget);
        setRejectOpen(false);
      }}
      onCancel={() => setRejectOpen(false)}
    >
      <div
        ref={wrapRef}
        tabIndex={0}
        className="application-table"
        onKeyDown={handleKeyDown}
        onMouseDown={() => wrapRef.current?.focus()}
      >
        <Table<ApplicationInternal>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          columns={columns}
          scroll={{ x: 980 }}
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
    </Popconfirm>
  );
}
