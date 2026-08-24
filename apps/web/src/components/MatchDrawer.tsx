import type { ApplicationInternal, ApplicationStage } from '@recruitmate/shared-types';
import { STAGE_LABELS } from '@recruitmate/shared-types';
import {
  Alert,
  Button,
  Collapse,
  Descriptions,
  Divider,
  Drawer,
  Empty,
  List,
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
import { errorMessage, useApplicationDetail, type ApplicationDetail } from '../hooks/useApi';

const STAGE_OPTIONS = (Object.entries(STAGE_LABELS) as [ApplicationStage, string][]).map(
  ([value, label]) => ({ value, label }),
);

interface MatchDrawerProps {
  open: boolean;
  application: ApplicationInternal | null;
  onClose: () => void;
  /** 阶段流转（父级负责 mutation 与数据刷新） */
  onStageChange: (app: ApplicationInternal, stage: ApplicationStage) => void;
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
                options={STAGE_OPTIONS}
                onChange={(v) => onStageChange(app, v)}
              />
            </Space>
          </section>

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
    </Drawer>
  );
}
