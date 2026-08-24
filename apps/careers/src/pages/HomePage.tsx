import { useEffect, useMemo, useRef, useState } from 'react';
import { JOB_TYPE_LABELS } from '@recruitmate/shared-types';
import type { JobType } from '@recruitmate/shared-types';
import { JobCard } from '../components/JobCard';
import { Reveal } from '../components/Reveal';
import {
  AlertIcon,
  ArrowRightIcon,
  BriefcaseIcon,
  BuildingIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  InboxIcon,
  SearchIcon,
  SparklesIcon,
} from '../components/Icons';
import { getErrorMessage, useDepartments, useJobs } from '../hooks/useApi';
import { paginationItems } from '../utils/format';

const PAGE_SIZE = 10;

/** 岗位列表页：Hero 搜索 + 筛选（部门/类型）+ 卡片列表 + 分页 */
export function HomePage() {
  const [searchInput, setSearchInput] = useState('');
  // 防抖后的搜索关键词（300ms）
  const [q, setQ] = useState('');
  const [departmentId, setDepartmentId] = useState('');
  const [jobType, setJobType] = useState<'' | JobType>('');
  const [page, setPage] = useState(1);
  // 搜索结果区锚点（搜索反馈与自动滚动）
  const resultsRef = useRef<HTMLElement>(null);

  // 搜索防抖：停止输入 300ms 后再发请求
  useEffect(() => {
    const timer = setTimeout(() => setQ(searchInput.trim()), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // 筛选条件变化时回到第一页
  useEffect(() => {
    setPage(1);
  }, [q, departmentId, jobType]);

  useEffect(() => {
    document.title = '中葡经贸中心 · 人才招聘';
  }, []);

  // 稳定的查询参数（避免 queryKey 每次渲染都变化）
  const query = useMemo(
    () => ({
      q: q || undefined,
      departmentId: departmentId || undefined,
      jobType: jobType || undefined,
      page,
      pageSize: PAGE_SIZE,
    }),
    [q, departmentId, jobType, page],
  );

  const jobsQuery = useJobs(query);
  const deptQuery = useDepartments();

  const total = jobsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const hasFilter = Boolean(q || departmentId || jobType);

  // 用户开始搜索/筛选时，滚动到结果区（等布局稳定后手动计算，避免与品牌区收起竞争）
  const prevHasFilter = useRef(false);
  useEffect(() => {
    if (hasFilter && !prevHasFilter.current) {
      const t = setTimeout(scrollToResults, 80);
      return () => clearTimeout(t);
    }
    prevHasFilter.current = hasFilter;
    return undefined;
  }, [hasFilter]);

  function scrollToResults() {
    const el = resultsRef.current;
    if (!el) return;
    // 目标 = 结果区文档位置 - 顶部留白（sticky 导航约 64px + 呼吸空间）
    const top = el.getBoundingClientRect().top + window.scrollY - 96;
    window.scrollTo({ top: Math.max(top, 0), behavior: 'smooth' });
  }

  function clearFilters() {
    setSearchInput('');
    setDepartmentId('');
    setJobType('');
    setPage(1);
  }

  return (
    <div>
      {/* ============ Hero：机构主张 + 大搜索框 ============ */}
      <section className="relative overflow-hidden bg-linear-to-b from-brand-800 via-brand-700 to-brand-600">
        {/* 纹样覆盖层独立于背景渐变，避免 background-image 相互覆盖 */}
        <div className="hero-texture pointer-events-none absolute inset-0" aria-hidden="true" />
        {/* 悬浮光晕（缓慢漂移，增加空间感） */}
        <div
          className="animate-float-slow pointer-events-none absolute -top-10 right-[12%] h-56 w-56 rounded-full bg-gold-400/10 blur-3xl"
          aria-hidden="true"
        />
        <div
          className="animate-float-slow pointer-events-none absolute bottom-[-40px] left-[8%] h-64 w-64 rounded-full bg-white/5 blur-3xl"
          style={{ animationDelay: '-4.5s' }}
          aria-hidden="true"
        />
        <div className="relative mx-auto max-w-4xl px-4 pb-14 pt-14 text-center sm:pb-18 sm:pt-20">
          <span className="animate-rise badge border border-gold-300/60 bg-gold-400/25 text-gold-100">
            中国-葡语（西语）国家经济贸易服务中心
          </span>
          <h1
            className="animate-rise mt-5 text-3xl font-bold leading-snug tracking-tight text-white sm:text-5xl"
            style={{ animationDelay: '90ms' }}
          >
            连接中国与世界，
            <br className="sm:hidden" />
            共筑经贸桥梁
          </h1>
          <p
            className="animate-rise mx-auto mt-4 max-w-xl text-base leading-relaxed text-white/95 sm:text-lg"
            style={{ animationDelay: '180ms' }}
          >
            依托「澳门＋横琴」战略新定位，搭建连接中国与葡语（西语）国家的「一站式」综合服务平台，深化双方经贸合作。
          </p>
          <p
            className="animate-rise mt-3 text-sm font-medium tracking-[0.35em] text-gold-200"
            style={{ animationDelay: '270ms' }}
          >
            CONECTAR · COOPERAR · CRESCER
          </p>
          <div
            className="animate-rise search-glow relative mx-auto mt-8 max-w-xl"
            style={{ animationDelay: '360ms' }}
          >
            <SearchIcon className="absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" />
            <input
              className="h-14 w-full rounded-2xl border border-white/20 bg-white pl-12 pr-4 text-base text-slate-900 shadow-xl shadow-brand-900/30 outline-none transition placeholder:text-slate-400 focus:ring-2 focus:ring-gold-400/70"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="搜索岗位、关键词或技能，如「葡语」「Go」「经贸」"
              aria-label="搜索岗位"
            />
          </div>
          <p
            className="animate-rise mt-3 text-xs font-medium text-brand-100"
            style={{ animationDelay: '450ms' }}
          >
            支持按关键词、部门、职位类型筛选岗位
          </p>
          {/* 搜索反馈：有筛选条件时即时展示结果数量，点击直达结果区 */}
          {hasFilter && (
            <button
              type="button"
              onClick={scrollToResults}
              className="animate-rise mx-auto mt-3 inline-flex items-center gap-2 rounded-full border border-gold-300/40 bg-white/10 px-4 py-1.5 text-sm font-medium text-white backdrop-blur transition hover:bg-white/20"
              style={{ animationDelay: '500ms' }}
              aria-live="polite"
            >
              {jobsQuery.isFetching ? (
                <span className="inline-flex items-center gap-2">
                  <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-gold-300/40 border-t-gold-300" />
                  正在搜索…
                </span>
              ) : (
                <>
                  找到 <span className="font-bold text-gold-300">{total}</span> 个相关岗位
                  <ArrowRightIcon className="h-4 w-4 rotate-90" />
                </>
              )}
            </button>
          )}
        </div>
        <div className="azulejo-band animate-azulejo" />
      </section>

      {/* ============ 品牌区（仅浏览态展示；搜索/筛选时收起，结果直达） ============ */}
      {!hasFilter && (
        <>
          {/* ============ 关于我们：官方机构简介 ============ */}
      <section className="border-b border-slate-100 bg-white">
        <div className="mx-auto max-w-4xl px-4 py-12 sm:py-14">
          <Reveal>
            <div className="card border-l-4 border-l-gold-500 p-6 sm:p-8">
              <h2 className="flex items-center gap-2 text-lg font-bold text-slate-900 sm:text-xl">
                关于中葡经贸中心
                <span className="h-px flex-1 bg-linear-to-r from-gold-400/60 to-transparent" />
              </h2>
              <p className="mt-4 text-[15px] leading-relaxed text-slate-700">
                中国-葡语（西语）国家经济贸易服务中心（中葡经贸中心）是在国家有关部委及广东省指导支持下，由澳门特别行政区政府和横琴粤澳深度合作区执行委员会共同发起成立的。
              </p>
              <p className="mt-3 text-[15px] leading-relaxed text-slate-700">
                中葡经贸中心旨在落实“澳门＋横琴”战略新定位，发挥澳门作为对葡语（西语）国家“精准联系人”作用，依托“澳门＋横琴”政策优势、开放优势、创新优势和区位优势等，广泛汇聚中国、葡语（西语）国家的政府、产业企业、专业服务机构等多方资源，搭建连接中国与葡语（西语）国家的“一站式”综合服务平台，为有关市场主体提供精准、高效的服务，深化中国与葡语（西语）国家的经贸合作关系。
              </p>
              <div className="mt-5 flex flex-wrap gap-2">
                {['澳门＋横琴 战略定位', '对葡语国家精准联系人', '一站式综合服务平台'].map((t) => (
                  <span key={t} className="tag bg-brand-50 text-brand-700">
                    {t}
                  </span>
                ))}
              </div>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ============ 品牌价值主张 ============ */}
      <section className="border-b border-slate-100 bg-white">
        <div className="mx-auto grid max-w-4xl grid-cols-1 gap-4 px-4 py-8 sm:grid-cols-3">
          {[
            {
              icon: <BuildingIcon className="h-6 w-6 text-brand-600" />,
              title: '跨文化平台',
              desc: '服务中国与葡语（西语）国家合作，工作场景横跨多元市场。',
            },
            {
              icon: <BriefcaseIcon className="h-6 w-6 text-brand-600" />,
              title: '经贸一线',
              desc: '深度参与经贸促进、投资服务与产业对接的前沿实践。',
            },
            {
              icon: <SparklesIcon className="h-6 w-6 text-brand-600" />,
              title: '广阔成长',
              desc: '语言能力与专业能力并重，与机构共同成长。',
            },
          ].map((item, i) => (
            <Reveal key={item.title} delay={i * 90}>
              <div className="flex items-start gap-3">
                <span className="mt-0.5 flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-brand-50 transition-transform duration-300 hover:-translate-y-0.5 hover:rotate-3">
                  {item.icon}
                </span>
                <div>
                  <h3 className="text-sm font-semibold text-slate-900">{item.title}</h3>
                  <p className="mt-1 text-xs leading-relaxed text-slate-600">{item.desc}</p>
                </div>
              </div>
            </Reveal>
          ))}
        </div>
      </section>
        </>
      )}

      <main ref={resultsRef} className="mx-auto max-w-4xl scroll-mt-20 px-4 pb-16">
        {/* ============ 筛选栏 ============ */}
        <div className="card mt-6 flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:gap-4">
          <label className="flex items-center gap-2 sm:w-52">
            <BuildingIcon className="h-4 w-4 shrink-0 text-slate-400" />
            <select
              className="select"
              value={departmentId}
              onChange={(e) => setDepartmentId(e.target.value)}
              aria-label="按部门筛选"
            >
              <option value="">全部部门</option>
              {(deptQuery.data ?? []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
          </label>

          <label className="flex items-center gap-2 sm:w-44">
            <BriefcaseIcon className="h-4 w-4 shrink-0 text-slate-400" />
            <select
              className="select"
              value={jobType}
              onChange={(e) => setJobType(e.target.value as '' | JobType)}
              aria-label="按职位类型筛选"
            >
              <option value="">全部类型</option>
              {Object.entries(JOB_TYPE_LABELS).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>

          <button
            type="button"
            className="shrink-0 text-sm font-medium text-brand-600 transition hover:text-brand-700 disabled:cursor-not-allowed disabled:text-slate-300 sm:ml-auto"
            disabled={!hasFilter}
            onClick={clearFilters}
          >
            清空筛选
          </button>
        </div>

        {/* ============ 结果标题 ============ */}
        <div className="mt-8 flex items-baseline justify-between">
          <h2 className="text-lg font-semibold text-slate-900">
            {hasFilter ? '筛选结果' : '热招岗位'}
          </h2>
          {jobsQuery.data && <span className="text-sm text-slate-400">共 {total} 个岗位</span>}
        </div>

        {/* ============ 列表 / 加载 / 错误 / 空态 ============ */}
        {jobsQuery.isPending ? (
          <JobListSkeleton />
        ) : jobsQuery.isError ? (
          <div className="card mt-4 flex flex-col items-center px-6 py-12 text-center">
            <span className="flex h-12 w-12 items-center justify-center rounded-full bg-red-50 text-red-400">
              <AlertIcon className="h-6 w-6" />
            </span>
            <p className="mt-3 text-sm text-slate-500">{getErrorMessage(jobsQuery.error)}</p>
            <button className="btn-secondary mt-4" onClick={() => jobsQuery.refetch()}>
              重新加载
            </button>
          </div>
        ) : (jobsQuery.data.items ?? []).length === 0 ? (
          <div className="card mt-4 flex flex-col items-center px-6 py-14 text-center">
            <span className="flex h-14 w-14 items-center justify-center rounded-full bg-slate-100 text-slate-400">
              <InboxIcon className="h-7 w-7" />
            </span>
            <h3 className="mt-4 text-base font-semibold text-slate-700">没有找到符合条件的岗位</h3>
            <p className="mt-1.5 text-sm text-slate-400">试试更换关键词，或清空筛选条件后再试</p>
            {hasFilter && (
              <button className="btn-secondary mt-5" onClick={clearFilters}>
                清空筛选
              </button>
            )}
          </div>
        ) : (
          <div className="mt-4 space-y-4">
            {(jobsQuery.data.items ?? []).map((job, i) => (
              <Reveal key={job.id} delay={Math.min(i, 5) * 70}>
                <JobCard job={job} />
              </Reveal>
            ))}
          </div>
        )}

        {/* ============ 分页 ============ */}
        {jobsQuery.data && total > 0 && (
          <Pagination page={page} totalPages={totalPages} onChange={setPage} />
        )}
      </main>
    </div>
  );
}

/** 分页器：上一页/下一页 + 页码窗口 + 省略号 */
function Pagination({
  page,
  totalPages,
  onChange,
}: {
  page: number;
  totalPages: number;
  onChange: (page: number) => void;
}) {
  if (totalPages <= 1) return null;
  const items = paginationItems(page, totalPages);
  const base =
    'inline-flex h-9 min-w-9 items-center justify-center rounded-lg border text-sm font-medium transition';
  return (
    <nav className="mt-8 flex items-center justify-center gap-1.5" aria-label="分页">
      <button
        type="button"
        className={`${base} border-slate-200 bg-white text-slate-600 hover:border-brand-200 hover:text-brand-600 disabled:cursor-not-allowed disabled:opacity-40`}
        disabled={page <= 1}
        onClick={() => onChange(page - 1)}
        aria-label="上一页"
      >
        <ChevronLeftIcon className="h-4 w-4" />
      </button>
      {items.map((item, i) =>
        item === '…' ? (
          <span key={`ellipsis-${i}`} className="px-1 text-sm text-slate-400">
            …
          </span>
        ) : (
          <button
            key={item}
            type="button"
            className={`${base} ${
              item === page
                ? 'border-brand-600 bg-brand-600 text-white shadow-sm'
                : 'border-slate-200 bg-white text-slate-600 hover:border-brand-200 hover:text-brand-600'
            }`}
            onClick={() => onChange(item)}
            aria-current={item === page ? 'page' : undefined}
          >
            {item}
          </button>
        ),
      )}
      <button
        type="button"
        className={`${base} border-slate-200 bg-white text-slate-600 hover:border-brand-200 hover:text-brand-600 disabled:cursor-not-allowed disabled:opacity-40`}
        disabled={page >= totalPages}
        onClick={() => onChange(page + 1)}
        aria-label="下一页"
      >
        <ChevronRightIcon className="h-4 w-4" />
      </button>
    </nav>
  );
}

/** 列表加载骨架屏 */
function JobListSkeleton() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="card p-5 sm:p-6">
          <div className="flex items-start justify-between gap-3">
            <div className="h-5 w-1/2 animate-pulse rounded bg-slate-100" />
            <div className="h-5 w-14 animate-pulse rounded-full bg-slate-100" />
          </div>
          <div className="mt-3 h-4 w-2/3 animate-pulse rounded bg-slate-100" />
          <div className="mt-3 flex gap-2">
            <div className="h-6 w-14 animate-pulse rounded-lg bg-slate-100" />
            <div className="h-6 w-14 animate-pulse rounded-lg bg-slate-100" />
            <div className="h-6 w-14 animate-pulse rounded-lg bg-slate-100" />
          </div>
          <div className="mt-4 h-4 w-full animate-pulse rounded bg-slate-100" />
          <div className="mt-2 h-4 w-4/5 animate-pulse rounded bg-slate-100" />
        </div>
      ))}
    </div>
  );
}
