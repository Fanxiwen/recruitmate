import { Fragment, useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import { Link } from 'react-router-dom';
import type { ApplicationPublic, CandidateApplicationStatus } from '@recruitmate/shared-types';
import {
  AlertIcon,
  CheckIcon,
  InboxIcon,
  LogoutIcon,
  MailIcon,
  XIcon,
} from '../components/Icons';
import {
  clearAuthToken,
  getAuthToken,
  getErrorMessage,
  isUnauthorized,
  setAuthToken,
  useMyApplications,
  useSendCode,
  useVerifyCode,
} from '../hooks/useApi';
import { formatDateTime, isValidEmail } from '../utils/format';

/** 状态步骤定义（已投递 → 处理中/面试中 → Offer/结果） */
const STATUS_STEPS = ['已投递', '处理中/面试中', 'Offer/结果'] as const;

/** 求职者视角状态 → 展示标签（shared-types 未提供该映射，此处定义） */
const STATUS_LABELS: Record<CandidateApplicationStatus, string> = {
  processing: '处理中',
  interviewing: '面试中',
  offered: '已发Offer',
  hired: '已入职',
  rejected: '未通过',
};

/** 状态 → 醒目提示文案 */
const STATUS_HINTS: Record<CandidateApplicationStatus, string> = {
  processing: '简历已投递，HR 正在处理中，请耐心等待',
  interviewing: '已进入面试环节，请留意邮件通知',
  offered: '已收到 Offer，请留意后续沟通',
  hired: '恭喜！你已成功入职，期待与你共事',
  rejected: '很遗憾，本次投递未通过筛选，感谢你的关注',
};

const VERIFY_CODE_LENGTH = 6;

/** 我的投递页：未登录时邮箱验证码登录，登录后展示投递状态 Steps */
export function MyApplicationsPage() {
  const [token, setToken] = useState<string | null>(() => getAuthToken());
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [codeSent, setCodeSent] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const [notice, setNotice] = useState('');
  const [loginError, setLoginError] = useState('');

  const sendCode = useSendCode();
  const verifyCode = useVerifyCode();
  const appsQuery = useMyApplications(token);

  useEffect(() => {
    document.title = '我的投递 · Recruitmate';
  }, []);

  // 验证码重发倒计时（60s）
  useEffect(() => {
    if (countdown <= 0) return;
    const timer = setInterval(() => setCountdown((c) => c - 1), 1000);
    return () => clearInterval(timer);
  }, [countdown]);

  // token 失效（401/403）时自动登出并提示重新验证
  useEffect(() => {
    if (appsQuery.error && isUnauthorized(appsQuery.error)) {
      clearAuthToken();
      setToken(null);
      setLoginError('登录已过期，请重新验证邮箱');
    }
  }, [appsQuery.error]);

  function handleSendCode(e: FormEvent) {
    e.preventDefault();
    setLoginError('');
    if (!email.trim()) {
      setLoginError('请输入邮箱');
      return;
    }
    if (!isValidEmail(email)) {
      setLoginError('邮箱格式不正确');
      return;
    }
    sendCode.mutate(
      { email: email.trim() },
      {
        onSuccess: () => {
          setCodeSent(true);
          setCountdown(60);
          setNotice(`验证码已发送至 ${email.trim()}，请查收邮箱（开发环境请查看后端日志）`);
        },
        onError: (err) => setLoginError(getErrorMessage(err)),
      },
    );
  }

  function handleVerify(e: FormEvent) {
    e.preventDefault();
    setLoginError('');
    if (!code.trim()) {
      setLoginError('请输入验证码');
      return;
    }
    verifyCode.mutate(
      { email: email.trim(), code: code.trim() },
      {
        onSuccess: (res) => {
          setAuthToken(res.token);
          setToken(res.token);
          setNotice('');
        },
        onError: (err) => setLoginError(getErrorMessage(err)),
      },
    );
  }

  function logout() {
    clearAuthToken();
    setToken(null);
    setEmail('');
    setCode('');
    setCodeSent(false);
    setCountdown(0);
    setNotice('');
    setLoginError('');
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-8 sm:py-12">
      <h1 className="text-2xl font-bold tracking-tight text-slate-900">我的投递</h1>
      <p className="mt-1.5 text-sm text-slate-500">查看你所有岗位的投递状态与结果反馈</p>

      {!token ? (
        /* ============ 未登录：邮箱验证码 ============ */
        <section className="card mt-8 p-6 sm:p-8">
          <h2 className="text-base font-semibold text-slate-900">验证邮箱，查看投递进度</h2>
          <p className="mt-1.5 text-sm text-slate-500">输入投递时填写的邮箱，获取验证码后即可查看</p>

          <form onSubmit={handleSendCode} className="mt-6 flex flex-col gap-3 sm:flex-row">
            <div className="relative flex-1">
              <MailIcon className="absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                inputMode="email"
                className="input pl-10"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="请输入投递时填写的邮箱"
              />
            </div>
            <button
              type="submit"
              disabled={sendCode.isPending || countdown > 0}
              className="btn-primary shrink-0"
            >
              {countdown > 0
                ? `重新发送（${countdown}s）`
                : sendCode.isPending
                  ? '发送中…'
                  : '发送验证码'}
            </button>
          </form>

          {codeSent && (
            <form onSubmit={handleVerify} className="mt-4 flex flex-col gap-3 sm:flex-row">
              <input
                type="text"
                inputMode="numeric"
                maxLength={VERIFY_CODE_LENGTH}
                className="input flex-1 text-center text-lg tracking-[0.5em]"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
                placeholder="6 位验证码"
                aria-label="验证码"
              />
              <button type="submit" disabled={verifyCode.isPending} className="btn-primary shrink-0">
                {verifyCode.isPending ? '验证中…' : '验证并查看'}
              </button>
            </form>
          )}

          {loginError && (
            <div className="mt-4 flex items-start gap-2 rounded-xl bg-red-50 px-3.5 py-3 text-sm text-red-600">
              <AlertIcon className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{loginError}</span>
            </div>
          )}
          {notice && (
            <div className="mt-4 flex items-start gap-2 rounded-xl bg-indigo-50 px-3.5 py-3 text-sm text-indigo-700">
              <AlertIcon className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{notice}</span>
            </div>
          )}

          <p className="mt-5 rounded-xl bg-slate-50 px-3.5 py-3 text-xs leading-5 text-slate-400">
            开发提示：验证码会打印在后端日志中；如未收到邮件，可联系管理员获取验证码。
          </p>
        </section>
      ) : (
        /* ============ 已登录：投递列表 ============ */
        <section className="mt-8">
          <div className="flex items-center justify-between">
            <p className="text-sm text-slate-500">
              {appsQuery.data ? `已通过邮箱验证，共 ${appsQuery.data.length} 条投递记录` : '正在加载投递记录…'}
            </p>
            <button
              type="button"
              onClick={logout}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:border-red-200 hover:bg-red-50 hover:text-red-600"
            >
              <LogoutIcon className="h-3.5 w-3.5" />
              退出登录
            </button>
          </div>

          {appsQuery.isPending ? (
            <ApplicationListSkeleton />
          ) : appsQuery.isError ? (
            <div className="card mt-5 flex flex-col items-center px-6 py-12 text-center">
              <span className="flex h-12 w-12 items-center justify-center rounded-full bg-red-50 text-red-400">
                <AlertIcon className="h-6 w-6" />
              </span>
              <p className="mt-3 text-sm text-slate-500">{getErrorMessage(appsQuery.error)}</p>
              <button className="btn-secondary mt-4" onClick={() => appsQuery.refetch()}>
                重新加载
              </button>
            </div>
          ) : appsQuery.data.length === 0 ? (
            <div className="card mt-5 flex flex-col items-center px-6 py-14 text-center">
              <span className="flex h-14 w-14 items-center justify-center rounded-full bg-slate-100 text-slate-400">
                <InboxIcon className="h-7 w-7" />
              </span>
              <h3 className="mt-4 text-base font-semibold text-slate-700">还没有投递记录</h3>
              <p className="mt-1.5 text-sm text-slate-400">去逛逛热招岗位，找到心仪的机会吧</p>
              <Link to="/" className="btn-primary mt-5">
                浏览热招岗位
              </Link>
            </div>
          ) : (
            <ul className="mt-5 space-y-4">
              {appsQuery.data.map((app) => (
                <ApplicationCard key={app.id} app={app} />
              ))}
            </ul>
          )}
        </section>
      )}
    </div>
  );
}

/** 单条投递卡片：岗位名 + 投递时间 + 状态 Steps + 状态文案 */
function ApplicationCard({ app }: { app: ApplicationPublic }) {
  const { status } = app;
  return (
    <li className="card p-5 sm:p-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <Link
            to={`/jobs/${app.jobId}`}
            className="text-base font-semibold text-slate-900 transition hover:text-indigo-600"
          >
            {app.jobTitle}
          </Link>
          <p className="mt-1 text-xs text-slate-400">投递于 {formatDateTime(app.submittedAt)}</p>
        </div>
        <span className={`badge ${statusBadgeClass(status)}`}>{STATUS_LABELS[status]}</span>
      </div>

      <StatusSteps status={status} />

      <p className={`mt-4 rounded-xl px-3.5 py-2.5 text-sm ${statusHintClass(status)}`}>
        {STATUS_HINTS[status]}
      </p>
    </li>
  );
}

/** 状态 Steps：processing→第一步，interviewing→第二步，offered→第三步；hired 成功终点，rejected 红色终点 */
function StatusSteps({ status }: { status: CandidateApplicationStatus }) {
  // 当前到达的步骤下标（0 起步）
  const activeIndex = status === 'processing' ? 0 : status === 'interviewing' ? 1 : 2;
  const isSuccess = status === 'hired';
  const isDanger = status === 'rejected';

  return (
    <ol className="mt-5 flex items-center">
      {STATUS_STEPS.map((label, i) => {
        const done = i < activeIndex;
        const current = i === activeIndex;
        // 圆圈样式
        const circleClass = isSuccess
          ? 'bg-emerald-500 text-white'
          : isDanger && current
            ? 'bg-red-500 text-white'
            : done
              ? 'bg-indigo-600 text-white'
              : current
                ? 'bg-indigo-600 text-white ring-4 ring-indigo-100'
                : 'bg-slate-100 text-slate-400';
        return (
          <Fragment key={label}>
            <li className="flex w-16 flex-col items-center sm:w-20">
              <span
                className={`flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold ${circleClass}`}
              >
                {done || isSuccess ? (
                  <CheckIcon className="h-3.5 w-3.5" />
                ) : current && isDanger ? (
                  <XIcon className="h-3.5 w-3.5" />
                ) : (
                  i + 1
                )}
              </span>
              <span
                className={`mt-1.5 whitespace-nowrap text-center text-[11px] leading-tight sm:text-xs ${
                  current ? 'font-medium text-slate-900' : 'text-slate-400'
                }`}
              >
                {label}
              </span>
            </li>
            {i < STATUS_STEPS.length - 1 && (
              <span
                className={`h-0.5 flex-1 rounded-full ${
                  isSuccess
                    ? 'bg-emerald-400'
                    : isDanger && i === activeIndex
                      ? 'bg-red-400'
                      : i < activeIndex
                        ? 'bg-indigo-400'
                        : 'bg-slate-200'
                }`}
                aria-hidden="true"
              />
            )}
          </Fragment>
        );
      })}
    </ol>
  );
}

/** 状态徽章配色 */
function statusBadgeClass(status: CandidateApplicationStatus): string {
  switch (status) {
    case 'hired':
      return 'bg-emerald-50 text-emerald-600';
    case 'rejected':
      return 'bg-red-50 text-red-500';
    case 'offered':
      return 'bg-indigo-50 text-indigo-600';
    case 'interviewing':
      return 'bg-blue-50 text-blue-600';
    case 'processing':
      return 'bg-slate-100 text-slate-500';
  }
}

/** 状态提示文案配色 */
function statusHintClass(status: CandidateApplicationStatus): string {
  switch (status) {
    case 'hired':
      return 'bg-emerald-50 text-emerald-700';
    case 'rejected':
      return 'bg-red-50 text-red-600';
    case 'offered':
      return 'bg-indigo-50 text-indigo-700';
    case 'interviewing':
      return 'bg-blue-50 text-blue-700';
    case 'processing':
      return 'bg-slate-50 text-slate-500';
  }
}

/** 列表加载骨架屏 */
function ApplicationListSkeleton() {
  return (
    <div className="mt-5 space-y-4">
      {Array.from({ length: 2 }).map((_, i) => (
        <div key={i} className="card p-5 sm:p-6">
          <div className="flex items-start justify-between gap-3">
            <div className="h-5 w-1/3 animate-pulse rounded bg-slate-100" />
            <div className="h-5 w-16 animate-pulse rounded-full bg-slate-100" />
          </div>
          <div className="mt-4 h-7 animate-pulse rounded-full bg-slate-100" />
          <div className="mt-4 h-9 animate-pulse rounded-xl bg-slate-100" />
        </div>
      ))}
    </div>
  );
}
