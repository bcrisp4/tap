import { defineConfig } from '@playwright/test';

// Two run modes:
//   - default: spin up `npm run dev` and hit it (fast iteration).
//   - TAP_E2E_BASE set: hit an already-running base URL — typically
//     the Go binary serving the embedded production SPA at :8080.
export default defineConfig({
	testDir: 'tests',
	fullyParallel: false,
	use: {
		baseURL: process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173',
		trace: 'on-first-retry'
	},
	webServer: process.env.TAP_E2E_BASE
		? undefined
		: { command: 'npm run dev', port: 5173, reuseExistingServer: true }
});
