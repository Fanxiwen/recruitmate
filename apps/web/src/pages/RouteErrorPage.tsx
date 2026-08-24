import { Button, Result } from 'antd';
import { Link, isRouteErrorResponse, useRouteError } from 'react-router-dom';

/**
 * 路由级错误兜底：渲染异常时显示友好提示，避免白屏与开发态错误横幅。
 * 注意：内部端可能部署在子路径（/hr/），「返回首页」用 Link 走路由
 * basename，不能写死 href="/"（会跳出到外部端）。
 */
export function RouteErrorPage() {
  const error = useRouteError();
  const status = isRouteErrorResponse(error) ? error.status : undefined;

  return (
    <div
      style={{
        height: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Result
        status={status === 404 ? '404' : 'error'}
        title={status === 404 ? '页面不存在' : '页面出错了'}
        subTitle={status === 404 ? '你访问的页面不存在或已被移除。' : '抱歉，页面渲染时发生异常，请返回重试。'}
        extra={[
          <Link key="home" to="/">
            <Button type="primary">返回首页</Button>
          </Link>,
          <Button key="reload" onClick={() => window.location.reload()}>
            刷新页面
          </Button>,
        ]}
      />
    </div>
  );
}
