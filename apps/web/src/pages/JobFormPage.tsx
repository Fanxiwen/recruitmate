import { ArrowLeftOutlined, SaveOutlined, SendOutlined } from '@ant-design/icons';
import { ApiError } from '@recruitmate/api-client';
import type { EducationLevel, JobPostingInput, JobType } from '@recruitmate/shared-types';
import { EDUCATION_LEVEL_LABELS, JOB_TYPE_LABELS } from '@recruitmate/shared-types';
import { Button, Card, Form, Input, InputNumber, Select, Skeleton, Space, message } from 'antd';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useCreateJob, useDepartments, useJob, useJobAction, useUpdateJob } from '../hooks/useApi';
import { useAuthStore } from '../stores/auth';

const { TextArea } = Input;

/** 学历下拉选项（来自 shared-types 的 EDUCATION_LEVEL_LABELS） */
const EDUCATION_OPTIONS = (Object.entries(EDUCATION_LEVEL_LABELS) as [EducationLevel, string][]).map(
  ([value, label]) => ({ value, label }),
);

const JOB_TYPE_OPTIONS = (Object.entries(JOB_TYPE_LABELS) as [JobType, string][]).map(
  ([value, label]) => ({ value, label }),
);

/**
 * 岗位表单（/jobs/new 与 /jobs/:id/edit 共用）。
 * 结构化要求编辑器：必备技能/加分技能 tags、最低学历、最低年限。
 * 校验：标题/部门/职责/至少一项必备技能必填；薪资 max ≥ min。
 * hiring_manager 的部门下拉锁定为本部门。
 */
export function JobFormPage() {
  const { id } = useParams();
  const isEdit = !!id;
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const [form] = Form.useForm<JobPostingInput>();

  const { data: departments, isLoading: deptLoading } = useDepartments();
  const { data: job, isLoading: jobLoading } = useJob(id ?? '');
  const createJob = useCreateJob();
  const updateJob = useUpdateJob(id ?? '');
  const submitAction = useJobAction('submit');

  // 编辑模式：回填岗位数据
  useEffect(() => {
    if (isEdit && job) {
      form.setFieldsValue({
        title: job.title,
        departmentId: job.departmentId,
        location: job.location,
        jobType: job.jobType,
        headcount: job.headcount,
        salaryMin: job.salaryMin,
        salaryMax: job.salaryMax,
        description: job.description,
        requirements: { ...job.requirements },
      });
    }
  }, [isEdit, job, form]);

  // hiring_manager 锁定本部门
  const lockedDepartmentId = user?.role === 'hiring_manager' ? user.departmentId : undefined;
  const departmentOptions = (departments ?? []).map((d) => ({ value: d.id, label: d.name }));

  const [submitting, setSubmitting] = useState(false);

  /** 校验并组装提交体 */
  const buildBody = async (): Promise<JobPostingInput> => {
    const values = await form.validateFields();
    return {
      title: values.title,
      departmentId: lockedDepartmentId ?? values.departmentId,
      location: values.location?.trim() ?? '',
      jobType: values.jobType,
      headcount: values.headcount,
      salaryMin: values.salaryMin,
      salaryMax: values.salaryMax,
      description: values.description,
      requirements: {
        mustSkills: values.requirements?.mustSkills ?? [],
        niceSkills: values.requirements?.niceSkills ?? [],
        minEducation: values.requirements?.minEducation ?? 'any',
        minYears: values.requirements?.minYears ?? 0,
      },
    };
  };

  /** 保存草稿（新建 POST / 编辑 PATCH），成功后回列表 */
  const handleSave = async () => {
    try {
      const body = await buildBody();
      if (isEdit) {
        await updateJob.mutateAsync(body);
        message.success('已保存修改');
      } else {
        await createJob.mutateAsync(body);
        message.success('已保存草稿');
      }
      navigate('/jobs');
    } catch (err) {
      // 校验失败时表单内已展示错误，这里只提示接口类错误
      if (err instanceof ApiError) message.error(err.message);
    }
  };

  /** 保存并提交审批（POST submit） */
  const handleSubmitApproval = async () => {
    setSubmitting(true);
    try {
      const body = await buildBody();
      let savedId: string;
      if (isEdit) {
        savedId = (await updateJob.mutateAsync(body)).id;
      } else {
        savedId = (await createJob.mutateAsync(body)).id;
      }
      await submitAction.mutateAsync(savedId);
      message.success('已提交审批');
      navigate('/jobs');
    } catch (err) {
      if (err instanceof ApiError) message.error(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  if (isEdit && jobLoading) {
    return (
      <Card title="编辑岗位">
        <Skeleton active paragraph={{ rows: 8 }} />
      </Card>
    );
  }

  // 编辑模式下仅草稿可重新提交审批
  const canSubmitApproval = !isEdit || job?.status === 'draft';

  return (
    <Card
      title={
        <Space>
          <Button
            type="text"
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate('/jobs')}
            aria-label="返回岗位列表"
          />
          {isEdit ? '编辑岗位' : '发布岗位'}
        </Space>
      }
    >
      <Form<JobPostingInput>
        form={form}
        layout="vertical"
        initialValues={{
          jobType: 'full_time',
          headcount: 1,
          requirements: { mustSkills: [], niceSkills: [], minEducation: 'any', minYears: 0 },
          ...(lockedDepartmentId ? { departmentId: lockedDepartmentId } : {}),
        }}
        style={{ maxWidth: 720 }}
      >
        <Card type="inner" title="岗位信息" style={{ marginBottom: 16 }}>
          <Form.Item
            name="title"
            label="岗位标题"
            rules={[{ required: true, message: '请输入岗位标题' }]}
          >
            <Input placeholder="例如：资深前端工程师" maxLength={100} />
          </Form.Item>
          <Form.Item
            name="departmentId"
            label="所属部门"
            rules={[{ required: true, message: '请选择所属部门' }]}
          >
            <Select
              placeholder="请选择部门"
              options={departmentOptions}
              loading={deptLoading}
              disabled={!!lockedDepartmentId}
            />
          </Form.Item>
          <Space size={16} wrap style={{ width: '100%' }} align="start">
            <Form.Item name="location" label="工作地点" style={{ minWidth: 220 }}>
              <Input placeholder="例如：北京 / 上海 / 远程" maxLength={50} />
            </Form.Item>
            <Form.Item
              name="jobType"
              label="岗位类型"
              rules={[{ required: true, message: '请选择岗位类型' }]}
            >
              <Select options={JOB_TYPE_OPTIONS} style={{ width: 160 }} />
            </Form.Item>
            <Form.Item
              name="headcount"
              label="需求人数"
              rules={[{ required: true, message: '请输入需求人数' }]}
            >
              <InputNumber min={1} max={999} style={{ width: 120 }} />
            </Form.Item>
          </Space>
          <Form.Item label="薪资（千元/月，可留空表示面议）" required>
            <Space size={8} align="baseline">
              <Form.Item name="salaryMin" noStyle>
                <InputNumber min={0} placeholder="最低" style={{ width: 120 }} />
              </Form.Item>
              <span>—</span>
              <Form.Item
                name="salaryMax"
                noStyle
                dependencies={['salaryMin']}
                rules={[
                  ({ getFieldValue }) => ({
                    validator: (_, value?: number) => {
                      const min = getFieldValue('salaryMin') as number | undefined;
                      if (min != null && value != null && value < min) {
                        return Promise.reject(new Error('最高薪资不能低于最低薪资'));
                      }
                      return Promise.resolve();
                    },
                  }),
                ]}
              >
                <InputNumber min={0} placeholder="最高" style={{ width: 120 }} />
              </Form.Item>
              <span>千元/月</span>
            </Space>
          </Form.Item>
          <Form.Item
            name="description"
            label="岗位职责"
            rules={[{ required: true, message: '请输入岗位职责' }]}
          >
            <TextArea rows={5} placeholder="描述岗位职责、工作内容等" maxLength={5000} showCount />
          </Form.Item>
        </Card>

        <Card type="inner" title="结构化要求" style={{ marginBottom: 24 }}>
          <Form.Item
            name={['requirements', 'mustSkills']}
            label="必备技能（硬性条件，至少一项）"
            rules={[
              {
                validator: (_, value: string[] | undefined) =>
                  value && value.length > 0
                    ? Promise.resolve()
                    : Promise.reject(new Error('请至少添加一项必备技能')),
              },
            ]}
          >
            <Select mode="tags" placeholder="输入后回车添加，例如：Go、React" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name={['requirements', 'niceSkills']} label="加分技能（可选）">
            <Select mode="tags" placeholder="输入后回车添加，例如：Kubernetes、AI 工程" tokenSeparators={[',']} />
          </Form.Item>
          <Space size={16} wrap align="start">
            <Form.Item name={['requirements', 'minEducation']} label="最低学历">
              <Select options={EDUCATION_OPTIONS} style={{ width: 160 }} />
            </Form.Item>
            <Form.Item name={['requirements', 'minYears']} label="最低工作年限（年）">
              <InputNumber min={0} max={50} style={{ width: 160 }} />
            </Form.Item>
          </Space>
        </Card>

        <Space>
          <Button icon={<SaveOutlined />} onClick={handleSave}>
            {isEdit ? '保存修改' : '保存草稿'}
          </Button>
          {canSubmitApproval && (
            <Button
              type="primary"
              icon={<SendOutlined />}
              loading={submitting}
              onClick={handleSubmitApproval}
            >
              提交审批
            </Button>
          )}
          <Button onClick={() => navigate('/jobs')}>取消</Button>
        </Space>
      </Form>
    </Card>
  );
}
