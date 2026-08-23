import { PlusOutlined } from '@ant-design/icons';
import type { JobPosting, JobStatus } from '@recruitmate/shared-types';
import { JOB_STATUS_LABELS, JOB_TYPE_LABELS } from '@recruitmate/shared-types';
import { Button, Card, Segmented, Space, Table, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { JobActions } from '../components/JobActions';
import { JobStatusTag } from '../components/JobStatusTag';
import { useJobs } from '../hooks/useApi';
import { formatDateTime, formatSalary } from '../lib/format';

const STATUS_VALUES: JobStatus[] = ['draft', 'pending', 'open', 'closed'];

/** 列表行数据：候选人总数依赖后端在列表接口返回 applicationCount（共享类型未声明，可选字段） */
type JobRow = JobPosting & { applicationCount?: number };

/**
 * 岗位管理页：状态 Segmented 筛选 + 分页表格 + 行内操作（编辑/提交审批/审批/关闭/重新开启）。
 * 状态筛选与 URL 参数联动（支持从 Sider「待审批」菜单带 ?status=pending 进入）。
 */
export function JobsPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // 从 URL 读取状态筛选（?status=pending）
  const rawStatus = searchParams.get('status') ?? '';
  const status = (STATUS_VALUES as string[]).includes(rawStatus) ? (rawStatus as JobStatus) : null;

  useEffect(() => {
    setPage(1);
  }, [status]);

  const { data, isLoading, isFetching } = useJobs({
    status: status ?? undefined,
    page,
    pageSize,
  });

  const setStatus = (value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set('status', value);
    else next.delete('status');
    setSearchParams(next, { replace: true });
  };

  const segmentedOptions = useMemo(
    () => [
      { label: '全部', value: '' },
      ...(Object.entries(JOB_STATUS_LABELS) as [JobStatus, string][]).map(([value, label]) => ({
        label,
        value,
      })),
    ],
    [],
  );

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
      title: '需求数',
      dataIndex: 'headcount',
      width: 80,
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
          options={segmentedOptions}
          value={status ?? ''}
          onChange={(v) => setStatus(String(v))}
        />
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
      </Space>
    </Card>
  );
}
