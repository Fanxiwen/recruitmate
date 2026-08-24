import { Link, NavLink } from 'react-router-dom';
import { GlobeIcon, ListBulletIcon } from './Icons';

/** 顶部导航：品牌 Logo + 我的投递入口（移动端自动收窄） */
export function Header() {
  return (
    <header className="sticky top-0 z-40 border-b border-slate-200/70 bg-white/85 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4 sm:h-16">
        <Link to="/" className="flex min-w-0 items-center gap-2.5" aria-label="中葡经贸中心 人才招聘首页">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-linear-to-br from-brand-700 to-brand-500 text-gold-300 shadow-sm transition-transform duration-300 hover:-rotate-6 hover:scale-105 sm:h-9 sm:w-9">
            <GlobeIcon className="h-4 w-4 sm:h-5 sm:w-5" />
          </span>
          <span className="truncate text-base font-bold tracking-tight text-slate-900 sm:text-lg">
            中葡经贸中心
          </span>
          <span className="hidden rounded-full bg-brand-50 px-2.5 py-0.5 text-xs font-medium text-brand-600 sm:inline-block">
            人才招聘
          </span>
        </Link>

        <nav className="flex shrink-0 items-center gap-1">
          <NavLink
            to="/my/applications"
            className={({ isActive }) =>
              `inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition ${
                isActive
                  ? 'bg-brand-50 text-brand-600'
                  : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
              }`
            }
          >
            <ListBulletIcon className="h-4 w-4" />
            <span>我的投递</span>
          </NavLink>
        </nav>
      </div>
    </header>
  );
}
