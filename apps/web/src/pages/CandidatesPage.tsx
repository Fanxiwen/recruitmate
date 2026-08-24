import type { ApplicationInternal, ApplicationStage, CandidateListQuery, InterviewRound } from '@recruitmate/shared-types';
import { STAGE_LABELS } from '@recruitmate/shared-types';
import { Button, Card, Input, Segmented, Select, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useEffect, useMemo, useState } from 'react';
import { InterviewStatusTag } from '../components/InterviewStatusTag';
import { MatchDrawer } from '../components/MatchDrawer';
import { errorMessage, useCandidates, useDepartments, useJobs, useSetStage } from '../hooks/useApi';
import { formatDateTime, STAGE_COLORS } from '../lib/format';
import { STAGE_OPTIONS } from '../lib/stages';

type CandidateSort = 'score_desc' | 'newest';

const SORT_OPTIONS = [
  { label: '匹配度降序', value: 'score_desc' },
  { label: '最新投递', value: 'newest' },
];

/** 单轮面试列：面试时间 + 状态 Tag（未安排显示占位符） */
function InterviewCell({ app, round }: { app: ApplicationInternal; round: InterviewRound }) {
  const iv = (app.interviews ?? []).find((i) => i.round === round);
  if (!iv) return <Typography.Text type="secondary">—</Typography.Text>;
  return (
    <Space direction="vertical" size={0}>
      <Typography.Text style={{ fontSize: 12 }} type="secondary">
        {formatDateTime(iv.scheduledAt)}
      </Typography.Text>
      <InterviewStatusTag interview={iv} />
    </Space>
  );
}

/**
 * 候选人中心：跨岗位汇总全部候选人。
 * 顶部筛选（阶段 / 部门 / 岗位 / 搜索防抖 300ms / 排序）+ 分页表格（30s 自动刷新），
 * 行点击 / 查看详情打开 MatchDrawer（复用详情抽屉，支持面试安排与阶段流转）。
 */
export function CandidatesPage() {
  const [stage, setStage] = useState<ApplicationStage | ''>('');
  const [departmentId, setDepartmentId] = useState<string | undefined>(undefined);
  const [jobId, setJobId] = useState<string | undefined>(undefined);
  const [searchInput, setSearchInput] = useState('');
  const [q, setQ] = useState('');
  const [sort, setSort] = useState<CandidateSort>('score_desc');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // 搜索防抖 300ms
  useEffect(() => {
    const timer = setTimeout(() => setQ(searchInput.trim()), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // 筛选条件变化时回到第一页
  useEffect(() => {
    setPage(1);
  }, [q, stage, departmentId, jobId, sort]);

  const departments = useDepartments();

  // 岗位筛选数据源：招聘中 + 已关闭（草稿/待审批岗位无候选人投递）
  const openJobs = useJobs({ status: 'open', page: 1, pageSize: 100 });
  const closedJobs = useJobs({ status: 'closed', page: 1, pageSize: 100 });
  const jobOptions = useMemo(
    () => [...(openJobs.data?.items ?? []), ...(closedJobs.data?.items ?? [])],
    [openJobs.data, closedJobs.data],
  );

  const query: CandidateListQuery = {
    stage,
    departmentId,
    jobId,
    q: q || undefined,
    sort,
    page,
    pageSize,
  };
  const candidates = useCandidates(query);

  // ===== 候选人详情 Drawer =====
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detailApp, setDetailApp] = useState<ApplicationInternal | null>(null);
  const setStageMutation = useSetStage();

  const openDetail = (app: ApplicationInternal) => {
    setDetailApp(app);
    setDrawerOpen(true);
  };

  const handleStageChange = (app: ApplicationInternal, nextStage: ApplicationStage, reason?: string) => {
    setStageMutation.mutate(
      { id: app.id, stage: nextStage, reason },
      {
        onError: (e) => message.error(errorMessage(e, '更新阶段失败')),
      },
    );
  };

  const columns: ColumnsType<ApplicationInternal> = [
    {
      title: '候选人',
      key: 'candidate',
      width: 200,
      render: (_, app) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{app.candidateName}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {app.email}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '岗位',
      dataIndex: 'jobTitle',
      width: 170,
      ellipsis: true,
    },
    {
      title: '阶段',
      dataIndex: 'stage',
      width: 110,
      render: (v: ApplicationStage) => <Tag color={STAGE_COLORS[v]}>{STAGE_LABELS[v]}</Tag>,
    },
    {
      title: 'HR 初面',
      key: 'hr',
      width: 150,
      render: (_, app) => <InterviewCell app={app} round="hr" />,
    },
    {
      title: '负责人面',
      key: 'manager',
      width: 150,
      render: (_, app) => <InterviewCell app={app} round="manager" />,
    },
    {
      title: '匹配度',
      key: 'score',
      width: 90,
      render: (_, app) =>
        app.matchScore != null ? (
          <Typography.Text strong>{app.matchScore}</Typography.Text>
        ) : (
          <Tag color="processing">评分中…</Tag>
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
      width: 100,
      fixed: 'right',
      render: (_, app) => (
        <Button
          type="link"
          size="small"
          onClick={(e) => {
            e.stopPropagation();
            openDetail(app);
          }}
        >
          查看详情
        </Button>
      ),
    },
  ];

  return (
    <Card title="候选人中心">
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {/* 顶部筛选 */}
        <Space wrap size={8}>
          <Select
            allowClear
            placeholder="全部阶段"
            style={{ width: 150 }}
            value={stage || undefined}
            options={STAGE_OPTIONS}
            onChange={(v) => setStage((v ?? '') as ApplicationStage | '')}
          />
          <Select
            allowClear
            placeholder="全部部门"
            style={{ width: 150 }}
            value={departmentId}
            options={(departments.data ?? []).map((d) => ({ value: d.id, label: d.name }))}
            loading={departments.isLoading}
            onChange={(v) => setDepartmentId(v)}
          />
          <Select
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder="全部岗位"
            style={{ width: 220 }}
            value={jobId}
            options={jobOptions.map((j) => ({ value: j.id, label: j.title }))}
            loading={openJobs.isLoading || closedJobs.isLoading}
            onChange={(v) => setJobId(v)}
          />
          <Input
            allowClear
            placeholder="搜索姓名 / 邮箱"
            style={{ width: 220 }}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
          <Segmented
            options={SORT_OPTIONS}
            value={sort}
            onChange={(v) => setSort(v as CandidateSort)}
          />
        </Space>

        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          候选人中心跨岗位汇总，30 秒自动刷新；点击行查看候选人详情与面试安排。
        </Typography.Text>

        <Table<ApplicationInternal>
          rowKey="id"
          size="middle"
          loading={candidates.isLoading || candidates.isFetching}
          columns={columns}
          dataSource={candidates.data?.items ?? []}
          scroll={{ x: 1170 }}
          onRow={(record) => ({
            onClick: (e) => {
              const target = e.target as HTMLElement;
              if (target.closest('button, a, .ant-select, .ant-checkbox, .ant-segmented')) return;
              openDetail(record);
            },
          })}
          pagination={{
            current: page,
            pageSize,
            total: candidates.data?.total ?? 0,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50, 100],
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps);
            },
          }}
        />
      </Space>

      <MatchDrawer
        open={drawerOpen}
        application={detailApp}
        onClose={() => setDrawerOpen(false)}
        onStageChange={handleStageChange}
      />
    </Card>
  );
}
