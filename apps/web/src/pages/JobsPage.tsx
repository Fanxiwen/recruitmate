import { PlusOutlined } from '@ant-design/icons';
import type { ApplicationInternal, JobPosting, JobStatus } from '@recruitmate/shared-types';
import { JOB_STATUS_LABELS, JOB_TYPE_LABELS } from '@recruitmate/shared-types';
import { Badge, Button, Card, Form, Input, Modal, Segmented, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { JobActions } from '../components/JobActions';
import { JobStatusTag } from '../components/JobStatusTag';
import { errorMessage, useJobs, usePendingApplications, useSetStage } from '../hooks/useApi';
import { formatDateTime, formatSalary } from '../lib/format';

const STATUS_VALUES: JobStatus[] = ['draft', 'open', 'closed'];

/** 列表行数据：候选人总数依赖后端在列表接口返回 applicationCount（共享类型未声明，可选字段） */
type JobRow = JobPosting & { applicationCount?: number };

/**
 * 岗位管理页：
 *  - 岗位视图：状态 Segmented（全部/草稿/招聘中/已关闭）+ 分页表格（岗位审批已移至审批中心）
 *  - 待处理视图：新投递简历（stage=new）集中处理，HR 在此初筛（通过/淘汰），30s 自动刷新
 */
export function JobsPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [view, setView] = useState<'jobs' | 'pending'>('jobs');

  // 从 URL 读取状态筛选（?status=open 等）
  const rawStatus = searchParams.get('status') ?? '';
  const status = (STATUS_VALUES as string[]).includes(rawStatus) ? (rawStatus as JobStatus) : null;

  useEffect(() => {
    setPage(1);
  }, [status, view]);

  const { data, isLoading, isFetching } = useJobs({
    status: status ?? undefined,
    page,
    pageSize,
  });
  const pending = usePendingApplications(page, pageSize);

  // ===== 待处理：淘汰原因弹窗 =====
  const setStageMutation = useSetStage();
  const [rejectTarget, setRejectTarget] = useState<ApplicationInternal | null>(null);
  const [rejectForm] = Form.useForm();

  const handlePassScreening = async (app: ApplicationInternal) => {
    try {
      await setStageMutation.mutateAsync({ id: app.id, stage: 'screening' });
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
      return;
    }
    try {
      await setStageMutation.mutateAsync({ id: rejectTarget.id, stage: 'rejected', reason });
      message.success('已淘汰');
      setRejectTarget(null);
    } catch (err) {
      message.error(errorMessage(err, '淘汰失败'));
    }
  };

  const setStatus = (value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set('status', value);
    else next.delete('status');
    setSearchParams(next, { replace: true });
  };

  const segmentedOptions = useMemo(
    () => [
      { label: '全部', value: '' },
      ...(Object.entries(JOB_STATUS_LABELS) as [JobStatus, string][])
        .filter(([v]) => v !== 'pending') // 岗位发布审批已收口到审批中心
        .map(([value, label]) => ({ label, value })),
    ],
    [],
  );

  const pendingCount = pending.data?.total ?? 0;

  const columns: ColumnsType<JobRow> = [
    {
      title: '岗位',
      dataIndex: 'title',
      width: 220,
      render: (title: string, record) => (
        <Typography.Link onClick={() => navigate(`/jobs/${record.id}`)}>{title}</Typography.Link>
      ),
    },
    {
      title: '部门',
      dataIndex: 'departmentName',
      width: 140,
    },
    {
      title: '类型 / 地点',
      key: 'typeLocation',
      width: 150,
      render: (_, record) => `${JOB_TYPE_LABELS[record.jobType]} · ${record.location || '—'}`,
    },
    {
      title: '薪资（千元/月）',
      key: 'salary',
      width: 140,
      render: (_, record) => formatSalary(record.salaryMin, record.salaryMax),
    },
    {
      title: '招聘进度（入职/需求）',
      dataIndex: 'headcount',
      width: 150,
      render: (headcount: number, record: JobPosting) => {
        const hired = record.hiredCount ?? 0;
        return (
          <span style={{ color: hired >= headcount ? '#389e0d' : undefined }}>
            {hired}/{headcount}
            {hired >= headcount && ' · 已满编'}
          </span>
        );
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (value: JobStatus) => <JobStatusTag status={value} />,
    },
    {
      title: '候选人总数',
      key: 'applicationCount',
      width: 110,
      render: (_, record) =>
        record.applicationCount != null ? (
          <Typography.Link onClick={() => navigate(`/jobs/${record.id}`)}>
            {record.applicationCount}
          </Typography.Link>
        ) : (
          <Typography.Link onClick={() => navigate(`/jobs/${record.id}`)}>查看</Typography.Link>
        ),
    },
    {
      title: '发布时间',
      dataIndex: 'publishedAt',
      width: 150,
      render: (v: string | undefined) => formatDateTime(v),
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: 300,
      render: (_, record) => <JobActions job={record} size="small" />,
    },
  ];

  const pendingColumns: ColumnsType<ApplicationInternal> = [
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
      title: '应聘岗位',
      dataIndex: 'jobTitle',
      width: 180,
    },
    {
      title: '匹配度',
      key: 'score',
      width: 90,
      render: (_, app) =>
        app.matchScore != null ? (
          <Typography.Text strong>{app.matchScore}</Typography.Text>
        ) : (
          <Tag>评分中…</Tag>
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
      width: 240,
      render: (_, app) => (
        <Space>
          <Button type="primary" size="small" onClick={() => handlePassScreening(app)}>
            通过初筛
          </Button>
          <Button
            danger
            size="small"
            onClick={() => {
              rejectForm.resetFields();
              setRejectTarget(app);
            }}
          >
            淘汰
          </Button>
          <Typography.Link onClick={() => navigate(`/jobs/${app.jobId}`)}>进入岗位</Typography.Link>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title="岗位管理"
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/jobs/new')}>
          发布岗位
        </Button>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Segmented
          options={[
            ...segmentedOptions,
            {
              label: (
                <Space size={4}>
                  待处理
                  {pendingCount > 0 && <Badge count={pendingCount} size="small" />}
                </Space>
              ),
              value: 'pending',
            },
          ]}
          value={view === 'pending' ? 'pending' : (status ?? '')}
          onChange={(v) => {
            const val = String(v);
            if (val === 'pending') {
              setView('pending');
            } else {
              setView('jobs');
              setStatus(val);
            }
          }}
        />

        {view === 'pending' ? (
          <>
            <Typography.Text type="secondary">
              新投递的简历集中在此，按投递时间先后处理（30 秒自动刷新）。通过初筛后进入岗位候选人列表继续流程。
            </Typography.Text>
            <Table<ApplicationInternal>
              rowKey="id"
              loading={pending.isLoading || pending.isFetching}
              columns={pendingColumns}
              dataSource={pending.data?.items ?? []}
              pagination={{
                current: page,
                pageSize,
                total: pendingCount,
                showSizeChanger: true,
                pageSizeOptions: [10, 20, 50, 100],
                showTotal: (t) => `共 ${t} 条`,
                onChange: (p, ps) => {
                  setPage(p);
                  setPageSize(ps);
                },
              }}
            />
          </>
        ) : (
          <Table<JobRow>
            rowKey="id"
            loading={isLoading || isFetching}
            columns={columns}
            dataSource={(data?.items ?? []) as JobRow[]}
            scroll={{ x: 1180 }}
            pagination={{
              current: page,
              pageSize,
              total: data?.total ?? 0,
              showSizeChanger: true,
              pageSizeOptions: [10, 20, 50, 100],
              showTotal: (t) => `共 ${t} 条`,
              onChange: (p, ps) => {
                setPage(p);
                setPageSize(ps);
              },
            }}
          />
        )}
      </Space>

      {/* 待处理：淘汰原因弹窗 */}
      <Modal
        title={`淘汰候选人「${rejectTarget?.candidateName ?? ''}」`}
        open={!!rejectTarget}
        onOk={confirmReject}
        onCancel={() => setRejectTarget(null)}
        okText="确认淘汰"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        confirmLoading={setStageMutation.isPending}
        destroyOnHidden
      >
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
    </Card>
  );
}
