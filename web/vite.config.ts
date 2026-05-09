import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const GO_BACKEND = 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api':     { target: GO_BACKEND, changeOrigin: false },
      '/healthz': { target: GO_BACKEND, changeOrigin: false },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
