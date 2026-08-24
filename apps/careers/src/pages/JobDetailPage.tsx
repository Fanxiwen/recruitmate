import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { EDUCATION_LEVEL_LABELS, JOB_TYPE_LABELS } from '@recruitmate/shared-types';
import { ApplyModal } from '../components/ApplyModal';
import {
  AlertIcon,
  BuildingIcon,
  ChevronLeftIcon,
  MapPinIcon,
  MoneyIcon,
} from '../components/Icons';
import { getErrorMessage, useJob } from '../hooks/useApi';
import { relativeTime, salaryLabel, splitParagraphs } from '../utils/format';

/** 岗位详情页：面包屑 + 头部信息 + 分区渲染（职责/要求/职位信息）+ 投递弹窗 */
export function JobDetailPage() {
  const { id } = useParams<{ id: string }>();
  const jobQuery = useJob(id);
  const [applyOpen, setApplyOpen] = useState(false);
  const closeApply = useCallback(() => setApplyOpen(false), []);

  const job = jobQuery.data;

  useEffect(() => {
    document.title = job ? `${job.title} · 中葡经贸中心` : '岗位详情 · 中葡经贸中心';
  }, [job]);

  if (jobQuery.isPending) return <DetailSkeleton />;
  if (jobQuery.isError || !job) {
    return <ErrorView error={jobQuery.error} onRetry={() => jobQuery.refetch()} />;
  }

  const isOpen = job.status === 'open';
  const eduLabel =
    job.requirements.minEducation === 'any'
      ? '学历不限'
      : `${EDUCATION_LEVEL_LABELS[job.requirements.minEducation]}及以上`;
  const yearLabel =
    job.requirements.minYears <= 0 ? '经验不限' : `${job.requirements.minYears} 年以上经验`;

  return (
    <div className="mx-auto max-w-5xl px-4 py-6 sm:py-8">
      {/* ============ 面包屑 ============ */}
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-sm text-slate-500 transition hover:text-brand-600"
      >
        <ChevronLeftIcon className="h-4 w-4" />
        返回岗位列表
      </Link>

      {/* ============ 岗位头部 ============ */}
      <section className="card mt-4 p-6 sm:p-8">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2.5">
              <h1 className="text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl">
                {job.title}
              </h1>
              <span className="badge bg-brand-50 text-brand-600">
                {JOB_TYPE_LABELS[job.jobType]}
              </span>
            </div>
            <div className="mt-3.5 flex flex-wrap items-center gap-x-5 gap-y-2 text-sm text-slate-500">
              <span className="inline-flex items-center gap-1.5">
                <BuildingIcon className="h-4 w-4 text-slate-400" />
                {job.departmentName}
              </span>
              <span className="inline-flex items-center gap-1.5">
                <MapPinIcon className="h-4 w-4 text-slate-400" />
                {job.location}
              </span>
              <span className="inline-flex items-center gap-1.5 font-medium text-slate-700">
                <MoneyIcon className="h-4 w-4 text-emerald-500" />
                {salaryLabel(job.salaryMin, job.salaryMax)}
              </span>
            </div>
          </div>
          <button
            type="button"
            className="btn-primary hidden px-6 md:inline-flex"
            disabled={!isOpen}
            onClick={() => setApplyOpen(true)}
          >
            {isOpen ? '立即投递' : '暂停招聘'}
          </button>
        </div>

        {!isOpen && (
          <div className="mt-5 flex items-start gap-2 rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-700">
            <AlertIcon className="mt-0.5 h-4 w-4 shrink-0" />
            <span>该岗位当前已停止招聘，可返回列表浏览其他热招岗位。</span>
          </div>
        )}
      </section>

      {/* ============ 正文分区 ============ */}
      <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="space-y-6">
          {/* 岗位职责 */}
          <section className="card p-6 sm:p-7">
            <h2 className="text-base font-semibold text-slate-900">岗位职责</h2>
            <div className="mt-4 space-y-3 text-sm leading-7 text-slate-600">
              {splitParagraphs(job.description).map((paragraph, i) => (
                <p key={i}>{paragraph}</p>
              ))}
            </div>
          </section>

          {/* 任职要求 */}
          <section className="card p-6 sm:p-7">
            <h2 className="text-base font-semibold text-slate-900">任职要求</h2>
            <div className="mt-4 space-y-5">
              <div>
                <p className="text-sm font-medium text-slate-700">必备技能</p>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {job.requirements.mustSkills.map((skill) => (
                    <span key={skill} className="tag bg-blue-50 text-blue-700">
                      {skill}
                    </span>
                  ))}
                </div>
              </div>
              <div>
                <p className="text-sm font-medium text-slate-700">加分技能</p>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {job.requirements.niceSkills.length > 0 ? (
                    job.requirements.niceSkills.map((skill) => (
                      <span key={skill} className="tag bg-slate-100 text-slate-600">
                        {skill}
                      </span>
                    ))
                  ) : (
                    <span className="text-sm text-slate-400">无</span>
                  )}
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <span className="badge border border-slate-200 bg-slate-50 text-slate-600">
                  {eduLabel}
                </span>
                <span className="badge border border-slate-200 bg-slate-50 text-slate-600">
                  {yearLabel}
                </span>
              </div>
            </div>
          </section>
        </div>

        {/* 右侧信息栏 */}
        <aside className="space-y-6">
          <section className="card p-6">
            <h2 className="text-base font-semibold text-slate-900">职位信息</h2>
            <dl className="mt-4 space-y-3.5 text-sm">
              <InfoRow label="所属部门" value={job.departmentName} />
              <InfoRow label="职位类型" value={JOB_TYPE_LABELS[job.jobType]} />
              <InfoRow label="招聘人数" value={`${job.headcount} 人`} />
              <InfoRow label="发布时间" value={relativeTime(job.publishedAt ?? job.createdAt)} />
            </dl>
          </section>

          <section className="rounded-2xl border border-brand-100 bg-brand-50/60 p-6">
            <h3 className="text-sm font-semibold text-brand-900">投递提示</h3>
            <p className="mt-2 text-xs leading-5 text-brand-700/80">
              投递后 1-3 个工作日内我们会通过邮件与你联系。可随时在「我的投递」中查看简历处理进度与结果反馈。
            </p>
          </section>
        </aside>
      </div>

      {/* ============ 移动端底部投递栏（sticky） ============ */}
      <div className="sticky bottom-0 z-30 mt-8 -mx-4 border-t border-slate-200 bg-white/95 px-4 py-3 backdrop-blur md:hidden">
        <button
          type="button"
          className="btn-primary w-full py-3"
          disabled={!isOpen}
          onClick={() => setApplyOpen(true)}
        >
          {isOpen ? '立即投递' : '暂停招聘'}
        </button>
      </div>

      <ApplyModal job={job} open={applyOpen} onClose={closeApply} />
    </div>
  );
}

/** 职位信息行（label - value） */
function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt className="text-slate-400">{label}</dt>
      <dd className="font-medium text-slate-700">{value}</dd>
    </div>
  );
}

/** 详情加载骨架屏 */
function DetailSkeleton() {
  return (
    <div className="mx-auto max-w-5xl px-4 py-6 sm:py-8">
      <div className="h-4 w-24 animate-pulse rounded bg-slate-100" />
      <div className="card mt-4 p-6 sm:p-8">
        <div className="h-7 w-2/3 animate-pulse rounded bg-slate-100" />
        <div className="mt-4 h-4 w-1/2 animate-pulse rounded bg-slate-100" />
        <div className="mt-6 h-10 w-28 animate-pulse rounded-xl bg-slate-100" />
      </div>
      <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="space-y-6">
          <div className="card p-6 sm:p-7">
            <div className="h-5 w-20 animate-pulse rounded bg-slate-100" />
            <div className="mt-4 space-y-2.5">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-4 animate-pulse rounded bg-slate-100" />
              ))}
            </div>
          </div>
          <div className="card p-6 sm:p-7">
            <div className="h-5 w-20 animate-pulse rounded bg-slate-100" />
            <div className="mt-4 flex flex-wrap gap-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-6 w-16 animate-pulse rounded-lg bg-slate-100" />
              ))}
            </div>
          </div>
        </div>
        <div className="card p-6">
          <div className="h-5 w-20 animate-pulse rounded bg-slate-100" />
          <div className="mt-4 space-y-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-4 animate-pulse rounded bg-slate-100" />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

/** 加载失败视图 */
function ErrorView({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return (
    <div className="mx-auto max-w-lg px-4 py-20 text-center">
      <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-red-50 text-red-400">
        <AlertIcon className="h-7 w-7" />
      </span>
      <h1 className="mt-4 text-lg font-semibold text-slate-900">岗位加载失败</h1>
      <p className="mt-2 text-sm text-slate-500">{getErrorMessage(error)}</p>
      <div className="mt-6 flex items-center justify-center gap-3">
        <Link to="/" className="btn-secondary">
          返回岗位列表
        </Link>
        <button type="button" className="btn-primary" onClick={onRetry}>
          重新加载
        </button>
      </div>
    </div>
  );
}
