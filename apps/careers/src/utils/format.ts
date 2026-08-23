/**
 * 通用格式化与校验工具函数。
 * 类型一律从 @recruitmate/shared-types 导入，禁止重复定义。
 */

// ============ 时间 ============

/** 友好相对时间：刚刚 / x 分钟前 / x 小时前 / x 天前 / x 个月前 / x 年前 */
export function relativeTime(iso: string): string {
  const time = new Date(iso).getTime();
  if (Number.isNaN(time)) return '';
  const diff = Date.now() - time;
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diff < minute) return '刚刚';
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`;
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`;
  if (diff < 30 * day) return `${Math.floor(diff / day)} 天前`;
  if (diff < 365 * day) return `${Math.floor(diff / (30 * day))} 个月前`;
  return `${Math.floor(diff / (365 * day))} 年前`;
}

/** 绝对时间：YYYY-MM-DD HH:mm */
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// ============ 薪资 ============

/** 月薪（千元）区间 → 展示文案；两者皆空时为「薪资面议」 */
export function salaryLabel(salaryMin?: number, salaryMax?: number): string {
  if (salaryMin == null && salaryMax == null) return '薪资面议';
  if (salaryMin != null && salaryMax != null) return `${salaryMin}K-${salaryMax}K`;
  if (salaryMin != null) return `${salaryMin}K 起`;
  return `最高 ${salaryMax}K`;
}

// ============ 文本 ============

/** 纯文本截断（压缩空白后按字数截断，超出加省略号） */
export function truncateText(text: string, max: number): string {
  const t = text.trim().replace(/\s+/g, ' ');
  return t.length > max ? `${t.slice(0, max)}…` : t;
}

/** 按空行将文本拆分为段落（用于岗位职责渲染） */
export function splitParagraphs(text: string): string[] {
  return text
    .split(/\r?\n+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

/** 文件大小友好显示：B / KB / MB */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)}MB`;
}

// ============ 表单校验 ============

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
/** 邮箱格式校验 */
export function isValidEmail(value: string): boolean {
  return EMAIL_RE.test(value.trim());
}

const PHONE_RE = /^\d{11}$/;
/** 手机号校验：11 位数字 */
export function isValidPhone(value: string): boolean {
  return PHONE_RE.test(value.trim());
}

// ============ 分页 ============

/** 页码窗口：1 … 4 5 6 … 20（超出 7 页时折叠为省略号） */
export function paginationItems(current: number, total: number): (number | '…')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const pages = new Set<number>([1, total, current - 1, current, current + 1]);
  const sorted = [...pages].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b);
  const result: (number | '…')[] = [];
  let prev = 0;
  for (const p of sorted) {
    if (p - prev > 1) result.push('…');
    result.push(p);
    prev = p;
  }
  return result;
}
