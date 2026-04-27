import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		// Forward API requests to the locally-running Go binary so
		// `npm run dev` can talk to `/api/v1/...`.
		proxy: { '/api': 'http://127.0.0.1:8080' }
	},
	// Vitest config — keep unit tests under src/ and let Playwright own
	// the tests/ directory.
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});
