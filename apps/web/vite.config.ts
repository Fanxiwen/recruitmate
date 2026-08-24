import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  // 部署路径可配置：默认根路径；部署到 https://域名/hr/ 时用 VITE_BASE_PATH=/hr 构建
  base: process.env.VITE_BASE_PATH ?? '/',
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom', 'react-router-dom'],
          antd: ['antd', '@ant-design/icons'],
          vendor: ['@tanstack/react-query', 'zustand', 'dayjs'],
        },
      },
    },
  },
});
