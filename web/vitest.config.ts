import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';

export default defineConfig({
  plugins: [svelte({ hot: !process.env.VITEST }), svelteTesting()],
  resolve: {
    alias: {
      // virtual:pwa-register/svelte is provided by vite-plugin-pwa at build time;
      // provide a minimal stub so vitest can resolve App.svelte in tests.
      'virtual:pwa-register/svelte': '/src/__mocks__/pwa-register-svelte.ts',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.ts'],
    setupFiles: ['src/test-setup.ts'],
  },
});
