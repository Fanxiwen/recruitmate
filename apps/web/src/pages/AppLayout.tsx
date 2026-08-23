import {
  AppstoreOutlined,
  AuditOutlined,
  BarChartOutlined,
  LogoutOutlined,
  RocketFilled,
} from '@ant-design/icons';
import { Badge, Button, Layout, Menu, Popconfirm, Space, Tag, message } from 'antd';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { usePendingCount } from '../hooks/useApi';
import { ROLE_LABELS } from '../lib/format';
import { useAuthStore } from '../stores/auth';

const { Sider, Header, Content } = Layout;

/** 角色 Tag 颜色 */
const ROLE_COLORS: Record<string, string> = {
  admin: 'red',
  hr: 'blue',
  hiring_manager: 'geekblue',
};

/**
 * 内部端主布局：左侧 Sider（logo + 菜单，待审批带徽标数字），
 * 顶部 Header（当前用户 + 角色 + 退出）。
 * 未登录时重定向到 /login。
 */
export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const token = useAuthStore((s) => s.token);
  const user = useAuthStore((s) => s.user);
  const { data: pending } = usePendingCount();

  // 认证守卫：无 token 或用户信息时回登录页
  if (!token || !user) return <Navigate to="/login" replace />;

  const pendingCount = pending?.total ?? 0;

  const handleLogout = () => {
    useAuthStore.getState().logout();
    navigate('/login', { replace: true });
  };

  // 根据路径高亮菜单（/jobs 与 /jobs?status=pending 都归属「岗位管理」）
  const selectedKeys = location.pathname.startsWith('/jobs') ? ['/jobs'] : [];

  const menuItems = [
    {
      key: '/jobs',
      icon: <AppstoreOutlined />,
      label: '岗位管理',
      onClick: () => navigate('/jobs'),
    },
    {
      key: 'pending',
      icon: <AuditOutlined />,
      label: (
        <Space size={6}>
          待审批
          {pendingCount > 0 && <Badge count={pendingCount} size="small" />}
        </Space>
      ),
      onClick: () => navigate('/jobs?status=pending'),
    },
    {
      key: 'stats',
      icon: <BarChartOutlined />,
      label: '统计',
      onClick: () => message.info('统计功能开发中，敬请期待'),
    },
  ];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider width={216} theme="dark" breakpoint="lg" collapsedWidth={64}>
        <div
          style={{
            height: 56,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 8,
            cursor: 'pointer',
          }}
          onClick={() => navigate('/jobs')}
        >
          <RocketFilled style={{ color: '#1677ff', fontSize: 22 }} />
          <span style={{ color: '#fff', fontSize: 17, fontWeight: 600, whiteSpace: 'nowrap' }}>
            Recruitmate
          </span>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={selectedKeys}
          items={menuItems}
          style={{ borderInlineEnd: 'none' }}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: '#fff',
            padding: '0 24px',
            display: 'flex',
            justifyContent: 'flex-end',
            alignItems: 'center',
            boxShadow: '0 1px 4px rgba(0,0,0,0.06)',
            zIndex: 10,
          }}
        >
          <Space size="middle">
            <Tag color={ROLE_COLORS[user.role] ?? 'default'} style={{ marginInlineEnd: 0 }}>
              {ROLE_LABELS[user.role]}
            </Tag>
            <span>{user.name}</span>
            {user.departmentName && (
              <span style={{ color: 'rgba(0,0,0,0.45)' }}>（{user.departmentName}）</span>
            )}
            <Popconfirm
              title="确认退出登录？"
              okText="退出"
              cancelText="取消"
              onConfirm={handleLogout}
            >
              <Button icon={<LogoutOutlined />} size="small">
                退出
              </Button>
            </Popconfirm>
          </Space>
        </Header>
        <Content style={{ padding: 24, overflow: 'auto' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
