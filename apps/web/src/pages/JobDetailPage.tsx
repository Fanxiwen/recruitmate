import { ArrowLeftOutlined, QuestionCircleOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ApplicationInternal, ApplicationStage } from '@recruitmate/shared-types';
import {
  EDUCATION_LEVEL_LABELS,
  JOB_TYPE_LABELS,
  STAGE_LABELS,
} from '@recruitmate/shared-types';
import {
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Input,
  Popover,
  Result,
  Row,
  Segmented,
  Select,
  Skeleton,
  Space,
  Statistic,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { useEffect, useState } from 'react';
import type { Key } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ApplicationTable } from '../components/ApplicationTable';
import { JobActions } from '../components/JobActions';
import { JobStatusTag } from '../components/JobStatusTag';
import { MatchDrawer } from '../components/MatchDrawer';
import {
  errorMessage,
  useApplications,
  useBatchAction,
  useJob,
  useJobStats,
  useSetStage,
} from '../hooks/useApi';
import { formatDateTime, formatSalary } from '../lib/format';

type SortValue = 'score_desc' | 'score_asc' | 'newest';
type HardPassFilter = 'only' | 'exclude';

const SORT_OPTIONS: { value: SortValue; label: string }[] = [
  { value: 'score_desc', label: '匹配度降序（默认）' },
  { value: 'score_asc', label: '匹配度升序' },
  { value: 'newest', label: '最新投递' },
];

const HARD_PASS_OPTIONS = [
  { label: '全部', value: '' },
  { label: '隐藏不满足', value: 'exclude' },
  { label: '只看不满足', value: 'only' },
];

const STAGE_OPTIONS = (Object.entries(STAGE_LABELS) as [ApplicationStage, string][]).map(
  ([value, label]) => ({ value, label }),
);

/** 技能标签渲染（空数组显示占位符） */
function renderSkillTags(skills: string[] | undefined) {
  if (!skills || skills.length === 0) return <Typography.Text type="secondary">—</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {skills.map((s) => (
        <Tag key={s}>{s}</Tag>
      ))}
    </Space>
  );
}

/** 键盘快捷键说明 */
const shortcutContent = (
  <div style={{ lineHeight: 1.9 }}>
    <div>
      <kbd>j</kbd> / <kbd>k</kbd> 上下移动高亮行
    </div>
    <div>
      <kbd>Enter</kbd> 打开当前行详情
    </div>
    <div>
      <kbd>x</kbd> 淘汰当前行（需确认）
    </div>
    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
      点击候选人表格区域获得焦点后生效
    </Typography.Text>
  </div>
);

/**
 * 岗位详情页（核心页面）：
 * - Tab「候选人」（默认激活）：搜索 / 阶段 / 硬性条件过滤 / 排序 / 批量操作 / 刷新轮询，
 *   表格支持行内阶段流转与键盘快捷键（j/k、Enter、x）；
 * - Tab「概览」：岗位全部信息 Descriptions + 岗位统计卡片（/jobs/:id/stats）。
 * 顶栏动作按钮与列表页共用 JobActions 逻辑。
 */
export function JobDetailPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();

  // ===== 岗位与统计 =====
  const { data: job, isLoading: jobLoading, isError: jobError, error } = useJob(id);
  const { data: stats } = useJobStats(id);

  // ===== 候选人筛选状态 =====
  const [searchInput, setSearchInput] = useState('');
  const [q, setQ] = useState('');
  const [stage, setStage] = useState<ApplicationStage | undefined>(undefined);
  const [hardPass, setHardPass] = useState<HardPassFilter | undefined>(undefined);
  const [sort, setSort] = useState<SortValue>('score_desc');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [selectedKeys, setSelectedKeys] = useState<Key[]>([]);

  // 搜索防抖 400ms
  useEffect(() => {
    const timer = setTimeout(() => setQ(searchInput.trim()), 400);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // 筛选条件变化时回到第一页
  useEffect(() => {
    setPage(1);
  }, [q, stage, hardPass, sort]);

  // AI 评分异步就绪：30s 自动轮询（refetchInterval）
  const applications = useApplications(
    id,
    { stage, hardPass, q: q || undefined, sort, page, pageSize },
    30_000,
  );

  // ===== 候选人详情 Drawer =====
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detailApp, setDetailApp] = useState<ApplicationInternal | null>(null);

  const openDetail = (app: ApplicationInternal) => {
    setDetailApp(app);
    setDrawerOpen(true);
  };

  // ===== 变更操作 =====
  const setStageMutation = useSetStage();
  const batch = useBatchAction(id);

  const handleStageChange = (app: ApplicationInternal, nextStage: ApplicationStage) => {
    setStageMutation.mutate(
      { id: app.id, stage: nextStage },
      {
        onError: (e) => message.error(errorMessage(e, '更新阶段失败')),
      },
    );
  };

  const handleRejectOne = (app: ApplicationInternal) => {
    batch.mutate(
      { ids: [app.id], action: 'reject' },
      {
        onSuccess: (res) => message.success(`已更新 ${res.updated} 份简历`),
        onError: (e) => message.error(errorMessage(e, '淘汰失败')),
      },
    );
  };

  const batchToScreening = () => {
    if (selectedKeys.length === 0) return;
    batch.mutate(
      { ids: selectedKeys as string[], action: 'stage', stage: 'screening' },
      {
        onSuccess: (res) => {
          message.success(`已更新 ${res.updated} 份简历`);
          setSelectedKeys([]);
        },
        onError: (e) => message.error(errorMessage(e, '批量操作失败')),
      },
    );
  };

  const batchReject = () => {
    if (selectedKeys.length === 0) return;
    batch.mutate(
      { ids: selectedKeys as string[], action: 'reject' },
      {
        onSuccess: (res) => {
          message.success(`已更新 ${res.updated} 份简历`);
          setSelectedKeys([]);
        },
        onError: (e) => message.error(errorMessage(e, '批量操作失败')),
      },
    );
  };

  if (jobLoading) {
    return (
      <Card>
        <Skeleton active paragraph={{ rows: 10 }} />
      </Card>
    );
  }

  if (jobError || !job) {
    return (
      <Card>
        <Result
          status="warning"
          title="岗位加载失败"
          subTitle={errorMessage(error, '岗位不存在或已被删除')}
          extra={
            <Button type="primary" onClick={() => navigate('/jobs')}>
              返回岗位列表
            </Button>
          }
        />
      </Card>
    );
  }

  const overviewTab = (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card title="岗位统计">
        <Row gutter={[16, 16]}>
          <Col xs={12} md={6}>
            <Statistic title="总投递" value={stats?.total ?? 0} />
          </Col>
          <Col xs={12} md={6}>
            <Statistic title="平均匹配分" value={stats?.avgScore ?? 0} suffix={stats?.avgScore != null ? '分' : ''} />
          </Col>
          <Col xs={12} md={6}>
            <Statistic title="不满足硬性条件" value={stats?.hardPassCount ?? 0} valueStyle={{ color: '#fa541c' }} />
          </Col>
          <Col xs={12} md={6}>
            <Statistic title="需求人数" value={job.headcount} />
          </Col>
        </Row>
        <DividerWithGap />
        <Typography.Text strong>各阶段计数</Typography.Text>
        <Row gutter={[16, 16]} style={{ marginTop: 12 }}>
          {STAGE_OPTIONS.map(({ value, label }) => (
            <Col xs={8} md={4} key={value}>
              <Statistic title={label} value={stats?.byStage[value] ?? 0} />
            </Col>
          ))}
        </Row>
      </Card>

      <Card title="岗位信息">
        <Descriptions bordered column={2} size="middle">
          <Descriptions.Item label="岗位标题">{job.title}</Descriptions.Item>
          <Descriptions.Item label="所属部门">{job.departmentName}</Descriptions.Item>
          <Descriptions.Item label="岗位类型">{JOB_TYPE_LABELS[job.jobType]}</Descriptions.Item>
          <Descriptions.Item label="工作地点">{job.location || '—'}</Descriptions.Item>
          <Descriptions.Item label="需求人数">{job.headcount} 人</Descriptions.Item>
          <Descriptions.Item label="薪资（千元/月）">{formatSalary(job.salaryMin, job.salaryMax)}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <JobStatusTag status={job.status} />
          </Descriptions.Item>
          <Descriptions.Item label="发布人">{job.ownerName}</Descriptions.Item>
          <Descriptions.Item label="发布时间">{formatDateTime(job.publishedAt)}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{formatDateTime(job.updatedAt)}</Descriptions.Item>
          <Descriptions.Item label="岗位职责" span={2}>
            <Typography.Paragraph style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>
              {job.description}
            </Typography.Paragraph>
          </Descriptions.Item>
          <Descriptions.Item label="必备技能" span={2}>
            {renderSkillTags(job.requirements.mustSkills)}
          </Descriptions.Item>
          <Descriptions.Item label="加分技能" span={2}>
            {renderSkillTags(job.requirements.niceSkills)}
          </Descriptions.Item>
          <Descriptions.Item label="最低学历">
            {EDUCATION_LEVEL_LABELS[job.requirements.minEducation]}
          </Descriptions.Item>
          <Descriptions.Item label="最低工作年限">{job.requirements.minYears} 年</Descriptions.Item>
        </Descriptions>
      </Card>
    </Space>
  );

  const candidatesTab = (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      {/* 工具栏：搜索 / 阶段 / 硬性条件 / 排序 / 批量操作 / 刷新 */}
      <Space wrap size={8}>
        <Input
          allowClear
          placeholder="搜索姓名 / 邮箱 / 技能关键词"
          style={{ width: 240 }}
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
        />
        <Select
          allowClear
          placeholder="阶段（留空=全部）"
          style={{ width: 150 }}
          value={stage}
          options={STAGE_OPTIONS}
          onChange={(v) => setStage(v || undefined)}
        />
        <Segmented
          options={HARD_PASS_OPTIONS}
          value={hardPass ?? ''}
          onChange={(v) => setHardPass((v as HardPassFilter | '') || undefined)}
        />
        <Select
          style={{ width: 170 }}
          value={sort}
          options={SORT_OPTIONS}
          onChange={(v) => setSort(v)}
        />
        {selectedKeys.length > 0 && (
          <>
            <Button
              type="primary"
              loading={batch.isPending}
              onClick={batchToScreening}
            >
              批量通过到初筛（{selectedKeys.length}）
            </Button>
            <Button danger loading={batch.isPending} onClick={batchReject}>
              批量淘汰
            </Button>
            <Button onClick={() => setSelectedKeys([])}>取消选择</Button>
          </>
        )}
        <Button
          icon={<ReloadOutlined />}
          loading={applications.isFetching}
          onClick={() => applications.refetch()}
        >
          刷新
        </Button>
      </Space>

      {applications.data && applications.data.items.length === 0 ? (
        <Empty description="暂无候选人，稍后将自动刷新" style={{ padding: '48px 0' }} />
      ) : (
        <ApplicationTable
          data={applications.data?.items}
          loading={applications.isLoading}
          total={applications.data?.total ?? 0}
          page={page}
          pageSize={pageSize}
          selectedKeys={selectedKeys}
          onSelectionChange={setSelectedKeys}
          onPageChange={(p, ps) => {
            setPage(p);
            setPageSize(ps);
          }}
          onStageChange={handleStageChange}
          onOpenDetail={openDetail}
          onReject={handleRejectOne}
        />
      )}
    </Space>
  );

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      {/* 顶栏：返回 + 标题 + 状态 + 快捷键提示 + 岗位操作 */}
      <Card>
        <Space style={{ width: '100%', justifyContent: 'space-between' }} wrap>
          <Space size={12}>
            <Button
              icon={<ArrowLeftOutlined />}
              onClick={() => navigate('/jobs')}
              aria-label="返回岗位列表"
            />
            <Typography.Title level={4} style={{ margin: 0 }}>
              {job.title}
            </Typography.Title>
            <JobStatusTag status={job.status} />
          </Space>
          <Space size={8}>
            <Popover content={shortcutContent} title="键盘快捷键" trigger="click">
              <Button icon={<QuestionCircleOutlined />}>快捷键</Button>
            </Popover>
            <JobActions job={job} />
          </Space>
        </Space>
      </Card>

      <Tabs
        defaultActiveKey="candidates"
        items={[
          { key: 'candidates', label: '候选人', children: candidatesTab },
          { key: 'overview', label: '概览', children: overviewTab },
        ]}
      />

      <MatchDrawer
        open={drawerOpen}
        application={detailApp}
        onClose={() => setDrawerOpen(false)}
        onStageChange={handleStageChange}
      />
    </Space>
  );
}

/** 统计卡与各阶段计数之间的小分隔 */
function DividerWithGap() {
  return <div style={{ height: 1, background: 'rgba(5,5,5,0.06)', margin: '16px 0' }} />;
}
