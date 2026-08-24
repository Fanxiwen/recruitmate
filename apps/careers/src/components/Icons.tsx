/**
 * 轻量内联 SVG 图标集（heroicons / lucide 风格，stroke 线条）。
 * 避免为少量图标引入额外依赖，默认 24x24 viewBox、当前色。
 */
import type { SVGProps } from 'react';

type IconProps = SVGProps<SVGSVGElement>;

/** 描边图标基座：统一线条参数 */
function StrokeIcon({ className = 'h-5 w-5', ...props }: IconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
      {...props}
    />
  );
}

/** 品牌闪电（实心，用于 Logo） */
export function BoltIcon({ className = 'h-5 w-5', ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" className={className} {...props}>
      <path d="M13.5 2 4.8 13.6h5.9L9.6 22l8.7-11.6h-5.9z" />
    </svg>
  );
}

/** 地球仪（品牌 Logo：中国—葡语国家桥梁意象） */
export function GlobeIcon({ className = 'h-5 w-5', ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true" className={className} {...props}>
      <circle cx="12" cy="12" r="9" strokeWidth={1.8} />
      <path d="M3.6 9h16.8M3.6 15h16.8" strokeWidth={1.8} strokeLinecap="round" />
      <ellipse cx="12" cy="12" rx="4" ry="9" strokeWidth={1.8} />
      <circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function SearchIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.6-3.6" />
    </StrokeIcon>
  );
}

export function MapPinIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M12 21s-7-5.1-7-11a7 7 0 1 1 14 0c0 5.9-7 11-7 11z" />
      <circle cx="12" cy="10" r="2.6" />
    </StrokeIcon>
  );
}

export function MoneyIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <rect x="3" y="6" width="18" height="12" rx="2" />
      <circle cx="12" cy="12" r="2.5" />
      <path d="M7 9.5h.01M17 14.5h.01" />
    </StrokeIcon>
  );
}

export function BriefcaseIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <rect x="3" y="7.5" width="18" height="13" rx="2" />
      <path d="M8 7.5V6a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v1.5" />
      <path d="M3 12.5h18" />
    </StrokeIcon>
  );
}

export function ClockIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3.2 2" />
    </StrokeIcon>
  );
}

export function CalendarIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <rect x="3.5" y="5" width="17" height="16" rx="2" />
      <path d="M8 3v4M16 3v4" />
      <path d="M3.5 10h17" />
    </StrokeIcon>
  );
}

export function UserIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <circle cx="12" cy="8" r="4" />
      <path d="M4.5 21c.8-3.8 3.9-5.5 7.5-5.5s6.7 1.7 7.5 5.5" />
    </StrokeIcon>
  );
}

export function CheckIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="m4.5 12.5 5 5L19.5 7" />
    </StrokeIcon>
  );
}

export function XIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M6 6l12 12M18 6 6 18" />
    </StrokeIcon>
  );
}

export function UploadIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M12 16V4.5" />
      <path d="m7.5 8.5 4.5-4.5 4.5 4.5" />
      <path d="M4 15.5V19a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3.5" />
    </StrokeIcon>
  );
}

export function DocumentIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5" />
    </StrokeIcon>
  );
}

export function ArrowRightIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M4.5 12h15" />
      <path d="m13 6 6 6-6 6" />
    </StrokeIcon>
  );
}

export function ChevronLeftIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="m15 6-6 6 6 6" />
    </StrokeIcon>
  );
}

export function ChevronRightIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="m9 6 6 6-6 6" />
    </StrokeIcon>
  );
}

export function InboxIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M22 12h-6l-2 3h-4l-2-3H2" />
      <path d="M5.5 5h13l3.5 7v6a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-6z" />
    </StrokeIcon>
  );
}

export function AlertIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 8v5" />
      <path d="M12 16.5h.01" />
    </StrokeIcon>
  );
}

export function BuildingIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <rect x="4" y="3" width="16" height="18" rx="2" />
      <path d="M8 7h.01M12 7h.01M16 7h.01M8 11h.01M12 11h.01M16 11h.01M8 15h.01M12 15h.01M16 15h.01" />
      <path d="M10 21v-3h4v3" />
    </StrokeIcon>
  );
}

export function LogoutIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M9 21H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3" />
      <path d="m15.5 16.5 4.5-4.5-4.5-4.5" />
      <path d="M20 12H9.5" />
    </StrokeIcon>
  );
}

export function MailIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <rect x="3" y="5.5" width="18" height="13" rx="2" />
      <path d="m3.5 7.5 8.5 6 8.5-6" />
    </StrokeIcon>
  );
}

export function ListBulletIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="M9 6h11M9 12h11M9 18h11" />
      <path d="M4 6h.01M4 12h.01M4 18h.01" />
    </StrokeIcon>
  );
}

export function SparklesIcon(props: IconProps) {
  return (
    <StrokeIcon {...props}>
      <path d="m12 3.5 1.7 4.6 4.6 1.7-4.6 1.7L12 16.1l-1.7-4.6-4.6-1.7 4.6-1.7z" />
      <path d="m18.5 14.5.8 2.2 2.2.8-2.2.8-.8 2.2-.8-2.2-2.2-.8 2.2-.8z" />
      <path d="m5.5 3.5.6 1.7 1.7.6-1.7.6-.6 1.7-.6-1.7-1.7-.6 1.7-.6z" />
    </StrokeIcon>
  );
}
