import { ApiError } from '@recruitmate/api-client';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Form, Input, Typography } from 'antd';
import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { useAuthStore } from '../stores/auth';

interface LoginFormValues {
  email: string;
  password: string;
}

/**
 * 登录页：居中卡片，邮箱 + 密码。
 * 成功后写入认证 store 并跳转 /jobs；失败展示 ApiError.message。
 */
export function LoginPage() {
  const navigate = useNavigate();
  const token = useAuthStore((s) => s.token);
  const user = useAuthStore((s) => s.user);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // 已登录则直接进入岗位管理
  if (token && user) return <Navigate to="/jobs" replace />;

  const onFinish = async (values: LoginFormValues) => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.login(values);
      useAuthStore.getState().setAuth(res.token, res.user);
      navigate('/jobs', { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '登录失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg, #e6f4ff 0%, #f0f5ff 100%)',
      }}
    >
      <Card style={{ width: 400, boxShadow: '0 4px 24px rgba(0,0,0,0.08)' }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <Typography.Title level={3} style={{ marginBottom: 4 }}>
            Recruitmate
          </Typography.Title>
          <Typography.Text type="secondary">内部招聘管理系统</Typography.Text>
        </div>
        <Form<LoginFormValues>
          layout="vertical"
          onFinish={onFinish}
          initialValues={{ email: '', password: '' }}
        >
          <Form.Item
            name="email"
            label="邮箱"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '邮箱格式不正确' },
            ]}
          >
            <Input prefix={<UserOutlined />} placeholder="you@company.com" autoComplete="email" />
          </Form.Item>
          <Form.Item
            name="password"
            label="密码"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="请输入密码"
              autoComplete="current-password"
            />
          </Form.Item>
          {error && (
            <Form.Item>
              <Alert type="error" showIcon message={error} />
            </Form.Item>
          )}
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" block loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
