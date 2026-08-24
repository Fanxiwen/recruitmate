import { createBrowserRouter } from 'react-router-dom';
import { Layout } from './components/Layout';
import { HomePage } from './pages/HomePage';
import { JobDetailPage } from './pages/JobDetailPage';
import { MyApplicationsPage } from './pages/MyApplicationsPage';
import { NotFoundPage } from './pages/NotFoundPage';
import { RouteErrorPage } from './pages/RouteErrorPage';

/**
 * 外部端路由：
 *  - /                    岗位列表（搜索/筛选）
 *  - /jobs/:id            岗位详情 + 投递
 *  - /my/applications     我的投递（邮箱验证码登录）
 * 渲染异常统一由 errorElement 兜底。
 */
export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    errorElement: <RouteErrorPage />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'jobs/:id', element: <JobDetailPage /> },
      { path: 'my/applications', element: <MyApplicationsPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]);
