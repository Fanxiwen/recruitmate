import { Navigate, createBrowserRouter } from 'react-router-dom';
import { AppLayout } from './pages/AppLayout';
import { ApprovalsPage } from './pages/ApprovalsPage';
import { JobDetailPage } from './pages/JobDetailPage';
import { JobFormPage } from './pages/JobFormPage';
import { JobsPage } from './pages/JobsPage';
import { LoginPage } from './pages/LoginPage';
import { RouteErrorPage } from './pages/RouteErrorPage';

/**
 * 内部端路由：
 *  - /login            登录（公开）
 *  - /jobs             岗位管理（列表 + 状态筛选）
 *  - /jobs/new         发布岗位
 *  - /jobs/:id         岗位详情（候选人 / 概览）
 *  - /jobs/:id/edit    编辑岗位
 * 其余路径由 AppLayout 做认证守卫（未登录重定向 /login）。
 * 渲染异常统一由 errorElement 兜底。
 *
 * 部署在子路径（如 https://域名/hr/）时，构建期设置 VITE_BASE_PATH=/hr，
 * 此处 basename 与 vite base 保持一致。
 */
export const router = createBrowserRouter(
  [
    { path: '/login', element: <LoginPage /> },
    {
      path: '/',
      element: <AppLayout />,
      errorElement: <RouteErrorPage />,
      children: [
        { index: true, element: <Navigate to="/jobs" replace /> },
        { path: 'jobs', element: <JobsPage /> },
        { path: 'jobs/new', element: <JobFormPage /> },
        { path: 'jobs/:id', element: <JobDetailPage /> },
        { path: 'jobs/:id/edit', element: <JobFormPage /> },
        { path: 'approvals', element: <ApprovalsPage /> },
      ],
    },
  ],
  { basename: import.meta.env.VITE_BASE_PATH ?? '/' },
);
