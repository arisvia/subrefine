/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'
import tailwindcss from '@tailwindcss/vite'
import { compression } from 'vite-plugin-compression2'
import { fileURLToPath, URL } from 'node:url'

const backend = () => `http://127.0.0.1:${process.env.SR_PORT ?? 3000}`

export default defineConfig(({ mode }) => ({
  plugins: [
    solid({
      hot: mode !== 'test',
      ssr: false,
    }),
    tailwindcss(),
    compression(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 20137,
    // 后端端口可注入：默认 3000，与 subrefine 默认监听端口一致
    // 用法：SR_PORT=3000 npm run dev
    proxy: {
      '/api': backend(),
      // Regex so /s/{id} proxies without swallowing /src/* module requests.
      '^/s/': backend(),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['src/test-setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    server: {
      deps: {
        inline: [/solid-js/, /@solidjs\/(router|testing-library)/],
      },
    },
  },
}))
