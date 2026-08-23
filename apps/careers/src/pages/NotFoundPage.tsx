import { Link } from 'react-router-dom';

/** 404 兜底页 */
export function NotFoundPage() {
  return (
    <div className="flex flex-col items-center px-4 py-24 text-center">
      <p className="text-5xl font-bold text-indigo-600">404</p>
      <p className="mt-3 text-slate-500">页面不存在或已被移除</p>
      <Link to="/" className="btn-primary mt-6">
        返回首页
      </Link>
    </div>
  );
}
