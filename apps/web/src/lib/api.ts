import { ApiClient } from '@recruitmate/api-client';
import { useAuthStore } from '../stores/auth';

/**
 * 内部端 API 客户端单例。
 * - 复用 api-client 包的类型安全封装，不直接 fetch；
 * - 注入 token 提供者：统一从 zustand 认证 store 读取（persist 持久化）；
 * - baseUrl=/api/v1，由 Vite dev 代理转发到 Go 后端。
 */
function getToken(): string | null {
  return useAuthStore.getState().token;
}

export const api = new ApiClient('/api/v1', getToken);
