import { isRouteErrorResponse, useRouteError } from 'react-router-dom';

/**
 * 路由级错误兜底：渲染异常时给出友好提示与返回入口，
 * 避免用户看到空白页或开发态错误横幅。
 */
export function RouteErrorPage() {
  const error = useRouteError();
  const status = isRouteErrorResponse(error) ? error.status : undefined;

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-slate-50 px-6 text-center">
      <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-red-50 text-3xl">
        ⚠️
      </div>
      <h1 className="mt-5 text-xl font-semibold text-slate-900">
        {status === 404 ? '页面不存在' : '页面出错了'}
      </h1>
      <p className="mt-2 max-w-sm text-sm text-slate-500">
        {status === 404
          ? '你访问的页面不存在或已被移除。'
          : '抱歉，页面渲染时发生异常，请返回首页重试。'}
      </p>
      <div className="mt-6 flex gap-3">
        <a href="/" className="btn-primary">
          返回首页
        </a>
        <button type="button" className="btn-secondary" onClick={() => window.location.reload()}>
          刷新页面
        </button>
      </div>
    </div>
  );
}
