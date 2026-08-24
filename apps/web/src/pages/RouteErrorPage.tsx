import { Button, Result } from 'antd';
import { isRouteErrorResponse, useRouteError } from 'react-router-dom';

/**
 * 路由级错误兜底：渲染异常时显示友好提示，避免白屏与开发态错误横幅。
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
          <Button type="primary" key="home" href="/">
            返回首页
          </Button>,
          <Button key="reload" onClick={() => window.location.reload()}>
            刷新页面
          </Button>,
        ]}
      />
    </div>
  );
}
