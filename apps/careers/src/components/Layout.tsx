import { Outlet, ScrollRestoration } from 'react-router-dom';
import { Footer } from './Footer';
import { Header } from './Header';

/** 整体布局：顶栏 + 页面内容 + 页脚（路由切换时恢复滚动位置） */
export function Layout() {
  return (
    <div className="flex min-h-screen flex-col">
      <ScrollRestoration />
      <Header />
      <main className="flex-1">
        <Outlet />
      </main>
      <Footer />
    </div>
  );
}
