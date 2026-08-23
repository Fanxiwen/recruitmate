import { defineConfig } from '@playwright/test';

/**
 * E2E 前置条件：
 *  - Go API 已在 http://localhost:8080 运行（make infra-up && make api-seed && make api-dev）
 *  - 前端 dev server 由本配置自动拉起（reuseExistingServer）
 */
export default defineConfig({
  testDir: './tests',
  timeout: 60_000,
  retries: 1,
  workers: 1, // 串行避免共享 seed 数据相互干扰
  reporter: [['list']],
  use: {
    locale: 'zh-CN',
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: 'pnpm dev:web',
      cwd: '..',
      url: 'http://localhost:5173',
      reuseExistingServer: true,
      timeout: 60_000,
    },
    {
      command: 'pnpm dev:careers',
      cwd: '..',
      url: 'http://localhost:5174',
      reuseExistingServer: true,
      timeout: 60_000,
    },
  ],
});
