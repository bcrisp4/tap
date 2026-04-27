import { test, expect, request } from '@playwright/test';

// These tests exercise the river end-to-end against a running Tap
// binary. They expect the API to live at the same origin (production
// embed) or via the Vite proxy when TAP_E2E_BASE is unset.
//
// `setExtraHTTPHeaders` would only affect Playwright's own page —
// the seed feed is added directly via the API instead.

const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

test.beforeEach(async () => {
	const ctx = await request.newContext({ baseURL: BASE });
	// Best-effort subscribe of a small public feed so the poller has
	// something to fetch. The endpoint is idempotent (409 on repeat),
	// which we deliberately swallow.
	await ctx
		.post('/api/v1/feeds', {
			data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
		})
		.catch(() => undefined);
});

test('unread view renders the river container and wordmark', async ({ page }) => {
	await page.goto('/');
	await expect(page.locator('.wordmark')).toBeVisible();
	await expect(page.getByTestId('river')).toBeVisible();
});

test('unread renders entries after the poller runs', async ({ page }) => {
	await page.goto('/');
	const rows = page.locator('.entry');
	await expect(rows.first()).toBeVisible({ timeout: 20_000 });
});

test('j/k advance the selected entry', async ({ page }) => {
	await page.goto('/');
	const rows = page.locator('.entry');
	await expect(rows.first()).toBeVisible({ timeout: 20_000 });

	// First row defaults to selected once data has loaded.
	await expect(rows.first()).toHaveClass(/is-selected/);

	await page.keyboard.press('j');
	await expect(rows.nth(1)).toHaveClass(/is-selected/);
	await page.keyboard.press('k');
	await expect(rows.first()).toHaveClass(/is-selected/);
});

test('m toggles read on the selected entry', async ({ page }) => {
	await page.goto('/');
	const rows = page.locator('.entry');
	await expect(rows.first()).toBeVisible({ timeout: 20_000 });
	await page.keyboard.press('m');
	await expect(rows.first()).toHaveClass(/is-read/);
});
