import { Link } from 'react-router-dom';
import { JOB_TYPE_LABELS } from '@recruitmate/shared-types';
import type { JobPosting } from '@recruitmate/shared-types';
import { relativeTime, salaryLabel, truncateText } from '../utils/format';
import {
  ArrowRightIcon,
  BuildingIcon,
  ClockIcon,
  MapPinIcon,
  MoneyIcon,
} from './Icons';

interface JobCardProps {
  job: JobPosting;
}

/** 岗位卡片：标题/部门/地点/类型/薪资/必备技能/职责摘要/发布时间，点击进入详情 */
export function JobCard({ job }: JobCardProps) {
  const skills = job.requirements.mustSkills;
  return (
    <Link
      to={`/jobs/${job.id}`}
      className="group card block p-5 transition duration-200 hover:-translate-y-0.5 hover:shadow-lg hover:shadow-slate-200/80 sm:p-6"
    >
      {/* 标题 + 类型 */}
      <div className="flex items-start justify-between gap-3">
        <h3 className="line-clamp-1 text-lg font-semibold text-slate-900 transition-colors group-hover:text-indigo-600">
          {job.title}
        </h3>
        <span className="badge shrink-0 bg-indigo-50 text-indigo-600">
          {JOB_TYPE_LABELS[job.jobType]}
        </span>
      </div>

      {/* 部门 / 地点 / 薪资 */}
      <div className="mt-2.5 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-sm text-slate-500">
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

      {/* 必备技能前 4 个 Tag */}
      {skills.length > 0 && (
        <div className="mt-3.5 flex flex-wrap gap-1.5">
          {skills.slice(0, 4).map((skill) => (
            <span key={skill} className="tag bg-slate-100 text-slate-600">
              {skill}
            </span>
          ))}
          {skills.length > 4 && (
            <span className="tag text-slate-400">+{skills.length - 4}</span>
          )}
        </div>
      )}

      {/* 职责摘要（纯文本截断 120 字） */}
      <p className="mt-3.5 line-clamp-2 text-sm leading-relaxed text-slate-500">
        {truncateText(job.description, 120)}
      </p>

      {/* 发布时间 + 查看详情 */}
      <div className="mt-4 flex items-center justify-between border-t border-slate-100 pt-3.5 text-xs text-slate-400">
        <span className="inline-flex items-center gap-1">
          <ClockIcon className="h-3.5 w-3.5" />
          {relativeTime(job.publishedAt ?? job.createdAt)}
        </span>
        <span className="inline-flex items-center gap-0.5 font-medium text-indigo-600">
          查看详情
          <ArrowRightIcon className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
        </span>
      </div>
    </Link>
  );
}
