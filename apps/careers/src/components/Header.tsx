import { Link, NavLink } from 'react-router-dom';
import { BoltIcon, ListBulletIcon } from './Icons';

/** 顶部导航：品牌 Logo + 我的投递入口（移动端自动收窄） */
export function Header() {
  return (
    <header className="sticky top-0 z-40 border-b border-slate-200/70 bg-white/85 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4 sm:h-16">
        <Link to="/" className="flex min-w-0 items-center gap-2.5" aria-label="Recruitmate 首页">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-600 to-blue-500 text-white shadow-sm sm:h-9 sm:w-9">
            <BoltIcon className="h-4 w-4 sm:h-5 sm:w-5" />
          </span>
          <span className="truncate text-base font-bold tracking-tight text-slate-900 sm:text-lg">
            Recruitmate
          </span>
          <span className="hidden rounded-full bg-indigo-50 px-2.5 py-0.5 text-xs font-medium text-indigo-600 sm:inline-block">
            加入我们
          </span>
        </Link>

        <nav className="flex shrink-0 items-center gap-1">
          <NavLink
            to="/my/applications"
            className={({ isActive }) =>
              `inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition ${
                isActive
                  ? 'bg-indigo-50 text-indigo-600'
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
