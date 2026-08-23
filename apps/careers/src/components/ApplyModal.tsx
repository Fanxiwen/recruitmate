import { useEffect, useRef, useState } from 'react';
import type { ChangeEvent, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import type { JobPosting } from '@recruitmate/shared-types';
import { getErrorMessage, useApplyJob } from '../hooks/useApi';
import { formatFileSize, isValidEmail, isValidPhone } from '../utils/format';
import { AlertIcon, CheckIcon, DocumentIcon, UploadIcon, XIcon } from './Icons';

/** 来源渠道选项（选填） */
const SOURCE_OPTIONS = ['官网', '内推', '猎聘', '其他'];

/** 简历文件白名单与大小上限 */
const ALLOWED_EXTS = ['pdf', 'docx', 'txt'];
const MAX_FILE_SIZE = 5 * 1024 * 1024;

interface ApplyModalProps {
  job: JobPosting;
  open: boolean;
  onClose: () => void;
}

type Phase = 'form' | 'success';

/**
 * 投递弹窗：基本信息 + 简历（上传文件或粘贴文本，二选一但至少其一）。
 * 提交成功后在 Modal 内展示成功态，并提供「查看我的投递」入口。
 */
export function ApplyModal({ job, open, onClose }: ApplyModalProps) {
  const navigate = useNavigate();
  const apply = useApplyJob(job.id);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [phase, setPhase] = useState<Phase>('form');
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [source, setSource] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [resumeText, setResumeText] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState('');

  // 每次打开时重置表单
  useEffect(() => {
    if (!open) return;
    setPhase('form');
    setName('');
    setEmail('');
    setPhone('');
    setSource('');
    setFile(null);
    setResumeText('');
    setFieldErrors({});
    setSubmitError('');
  }, [open]);

  // 打开时锁定背景滚动，并支持 Esc 关闭
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [open, onClose]);

  if (!open) return null;

  /** 写入/清除单个字段错误 */
  function setFieldError(key: string, message?: string) {
    setFieldErrors((prev) => {
      const next = { ...prev };
      if (message === undefined) delete next[key];
      else next[key] = message;
      return next;
    });
  }

  /** 校验表单，返回错误字典（无错误时为空对象） */
  function validate(): Record<string, string> {
    const errors: Record<string, string> = {};
    if (!name.trim()) errors.name = '请输入姓名';
    if (!email.trim()) errors.email = '请输入邮箱';
    else if (!isValidEmail(email)) errors.email = '邮箱格式不正确';
    if (!phone.trim()) errors.phone = '请输入手机号';
    else if (!isValidPhone(phone)) errors.phone = '手机号需为 11 位数字';
    if (!file && !resumeText.trim()) errors.resume = '请上传简历文件，或粘贴简历内容（至少提供一种）';
    return errors;
  }

  function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    // 清空 input 值，允许重复选择同一文件
    e.target.value = '';
    if (!f) return;
    const ext = f.name.split('.').pop()?.toLowerCase() ?? '';
    if (!ALLOWED_EXTS.includes(ext)) {
      setFieldError('resume', '仅支持 PDF / Word / TXT 格式的简历');
      return;
    }
    if (f.size > MAX_FILE_SIZE) {
      setFieldError('resume', '简历文件不能超过 5MB');
      return;
    }
    setFieldError('resume');
    setFile(f);
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitError('');
    const errors = validate();
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) return;
    apply.mutate(
      {
        data: {
          name: name.trim(),
          email: email.trim(),
          phone: phone.trim(),
          source: source.trim() || undefined,
          resumeText: resumeText.trim() || undefined,
        },
        file: file ?? undefined,
      },
      {
        onSuccess: () => setPhase('success'),
        onError: (err) => setSubmitError(getErrorMessage(err)),
      },
    );
  }

  function goToApplications() {
    onClose();
    navigate('/my/applications');
  }

  return (
    <div
      className="fixed inset-0 z-[60] flex items-end justify-center sm:items-center sm:p-4"
      role="dialog"
      aria-modal="true"
      aria-label="投递简历"
    >
      {/* 遮罩 */}
      <div className="absolute inset-0 bg-slate-900/50 backdrop-blur-sm" onClick={onClose} />

      {/* 弹窗主体：移动端底部弹出，桌面居中 */}
      <div className="relative flex max-h-[92dvh] w-full flex-col overflow-hidden rounded-t-2xl bg-white shadow-2xl sm:max-w-lg sm:rounded-2xl">
        <div className="flex items-start justify-between gap-3 border-b border-slate-100 px-5 py-4">
          <div className="min-w-0">
            <h3 className="text-base font-semibold text-slate-900">投递简历</h3>
            <p className="mt-0.5 truncate text-xs text-slate-400">{job.title}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-600"
            aria-label="关闭"
          >
            <XIcon className="h-5 w-5" />
          </button>
        </div>

        <div className="overflow-y-auto px-5 py-5">
          {phase === 'success' ? (
            /* ============ 成功态 ============ */
            <div className="flex flex-col items-center py-8 text-center">
              <span className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-100 text-emerald-600">
                <CheckIcon className="h-7 w-7" />
              </span>
              <h3 className="mt-4 text-lg font-semibold text-slate-900">投递成功</h3>
              <p className="mt-2 max-w-xs text-sm leading-6 text-slate-500">
                你的简历已送达{job.departmentName}，我们会在 1-3 个工作日内通过邮件与你联系。
              </p>
              <div className="mt-7 flex w-full flex-col gap-2.5 sm:w-auto sm:flex-row">
                <button type="button" className="btn-primary sm:px-6" onClick={goToApplications}>
                  查看我的投递
                </button>
                <button type="button" className="btn-secondary sm:px-6" onClick={onClose}>
                  继续浏览岗位
                </button>
              </div>
            </div>
          ) : (
            /* ============ 表单态 ============ */
            <form onSubmit={handleSubmit} noValidate className="space-y-4">
              <div>
                <label htmlFor="apply-name" className="mb-1.5 block text-sm font-medium text-slate-700">
                  姓名 <span className="text-red-500">*</span>
                </label>
                <input
                  id="apply-name"
                  className={`input ${fieldErrors.name ? 'border-red-400' : ''}`}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="请输入你的姓名"
                />
                {fieldErrors.name && <p className="mt-1 text-xs text-red-500">{fieldErrors.name}</p>}
              </div>

              <div>
                <label htmlFor="apply-email" className="mb-1.5 block text-sm font-medium text-slate-700">
                  邮箱 <span className="text-red-500">*</span>
                </label>
                <input
                  id="apply-email"
                  type="email"
                  className={`input ${fieldErrors.email ? 'border-red-400' : ''}`}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="用于接收面试通知与进度反馈"
                />
                {fieldErrors.email && <p className="mt-1 text-xs text-red-500">{fieldErrors.email}</p>}
              </div>

              <div>
                <label htmlFor="apply-phone" className="mb-1.5 block text-sm font-medium text-slate-700">
                  手机号 <span className="text-red-500">*</span>
                </label>
                <input
                  id="apply-phone"
                  inputMode="numeric"
                  maxLength={11}
                  className={`input ${fieldErrors.phone ? 'border-red-400' : ''}`}
                  value={phone}
                  onChange={(e) => setPhone(e.target.value.replace(/\D/g, ''))}
                  placeholder="请输入 11 位手机号"
                />
                {fieldErrors.phone && <p className="mt-1 text-xs text-red-500">{fieldErrors.phone}</p>}
              </div>

              <div>
                <label htmlFor="apply-source" className="mb-1.5 block text-sm font-medium text-slate-700">
                  来源（选填）
                </label>
                <select
                  id="apply-source"
                  className="select"
                  value={source}
                  onChange={(e) => setSource(e.target.value)}
                >
                  <option value="">请选择（选填）</option>
                  {SOURCE_OPTIONS.map((opt) => (
                    <option key={opt} value={opt}>
                      {opt}
                    </option>
                  ))}
                </select>
              </div>

              {/* 简历：文件或文本，至少一种 */}
              <div>
                <label htmlFor="apply-resume-text" className="mb-1.5 block text-sm font-medium text-slate-700">
                  简历 <span className="text-red-500">*</span>
                  <span className="ml-1 font-normal text-slate-400">（上传文件或粘贴内容，至少一种）</span>
                </label>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".pdf,.docx,.txt"
                  className="hidden"
                  onChange={handleFileChange}
                />
                {file ? (
                  <div className="flex items-center justify-between gap-2 rounded-xl border border-indigo-200 bg-indigo-50/60 px-3.5 py-3">
                    <span className="flex min-w-0 items-center gap-2 text-sm text-slate-700">
                      <DocumentIcon className="h-5 w-5 shrink-0 text-indigo-500" />
                      <span className="truncate">{file.name}</span>
                      <span className="shrink-0 text-xs text-slate-400">{formatFileSize(file.size)}</span>
                    </span>
                    <button
                      type="button"
                      onClick={() => setFile(null)}
                      className="shrink-0 rounded-lg p-1 text-slate-400 transition hover:bg-slate-200/60 hover:text-slate-600"
                      aria-label="移除文件"
                    >
                      <XIcon className="h-4 w-4" />
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => fileInputRef.current?.click()}
                    className="flex w-full flex-col items-center justify-center gap-1.5 rounded-xl border-2 border-dashed border-slate-200 bg-slate-50/60 px-4 py-6 text-slate-500 transition hover:border-indigo-300 hover:bg-indigo-50/40 hover:text-indigo-600"
                  >
                    <UploadIcon className="h-6 w-6" />
                    <span className="text-sm font-medium">点击上传简历文件</span>
                    <span className="text-xs text-slate-400">支持 PDF / Word / TXT，不超过 5MB</span>
                  </button>
                )}

                <div className="my-3 flex items-center gap-3 text-xs text-slate-400">
                  <span className="h-px flex-1 bg-slate-100" />
                  或
                  <span className="h-px flex-1 bg-slate-100" />
                </div>

                <textarea
                  id="apply-resume-text"
                  className={`input min-h-[110px] resize-y ${fieldErrors.resume ? 'border-red-400' : ''}`}
                  value={resumeText}
                  onChange={(e) => setResumeText(e.target.value)}
                  placeholder="无简历文件时可粘贴简历内容（支持纯文本）"
                  rows={4}
                />
                {fieldErrors.resume && <p className="mt-1 text-xs text-red-500">{fieldErrors.resume}</p>}
              </div>

              {/* 后端错误（如 duplicate：你已投递过该岗位） */}
              {submitError && (
                <div className="flex items-start gap-2 rounded-xl bg-red-50 px-3.5 py-3 text-sm text-red-600">
                  <AlertIcon className="mt-0.5 h-4 w-4 shrink-0" />
                  <span>{submitError}</span>
                </div>
              )}

              <button type="submit" disabled={apply.isPending} className="btn-primary w-full py-3">
                {apply.isPending ? '提交中…' : '提交投递'}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
