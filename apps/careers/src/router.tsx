import { createBrowserRouter } from 'react-router-dom';
import { HomePage } from './pages/HomePage';

/**
 * 路由骨架：页面实现见 pages/ 目录。
 * 外部端页面清单：
 *  - /                    岗位列表（搜索/筛选）
 *  - /jobs/:id            岗位详情 + 投递
 *  - /my/applications     我的投递（邮箱验证码登录）
 */
export const router = createBrowserRouter([
  { path: '/', element: <HomePage /> },
]);
