import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { VitePWA } from 'vite-plugin-pwa';

const GO_BACKEND = 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [
    svelte(),
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src/sw',
      filename: 'sw.ts',
      registerType: 'prompt',
      injectManifest: {
        injectionPoint: 'self.__WB_MANIFEST',
        minify: true,
        sourcemap: false,
        rollupFormat: 'es',
        buildPlugins: { vite: [], rollup: [] },
      },
      manifest: {
        name: 'Tap',
        short_name: 'Tap',
        description: 'Self-hosted feed reader',
        display: 'standalone',
        start_url: '/',
        scope: '/',
        background_color: '#fafaf7',
        theme_color: '#002FA7',
        icons: [
          {
            src: '/icons/icon-192.png',
            sizes: '192x192',
            type: 'image/png',
            purpose: 'maskable',
          },
          {
            src: '/icons/icon-512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
    }),
  ],
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
