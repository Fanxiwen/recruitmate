import type { User } from '@recruitmate/shared-types';
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface AuthState {
  token: string | null;
  user: User | null;
  /** 登录成功后写入 token 与用户信息 */
  setAuth: (token: string, user: User) => void;
  /** 退出/401 时清空认证状态 */
  logout: () => void;
}

/**
 * 认证状态：zustand + persist。
 * localStorage 持久化 {token, user}，刷新页面后仍保持登录。
 * ApiClient 的 getToken 从这里读取（见 src/lib/api.ts）。
 */
export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      setAuth: (token, user) => set({ token, user }),
      logout: () => set({ token: null, user: null }),
    }),
    { name: 'recruitmate-auth' },
  ),
);
