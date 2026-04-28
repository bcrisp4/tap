import { test, expect, request } from '@playwright/test';

// These tests exercise the river end-to-end against a running Tap
// binary. They expect the API to live at the same origin (production
// embed) or via the Vite proxy when TAP_E2E_BASE is unset.
//
// The seed feed is subscribed via the API directly so the test
// doesn't depend on the SPA's own forms (which land in a later plan).

// `@types/node` isn't a dependency (the rest of the SPA uses Vite
// shims) — declare the slice of Node's global we actually consume.
declare const process: { env: Record<string, string | undefined> };

const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

test.beforeEach(async () => {
	const ctx = await request.newContext({ baseURL: BASE });
	try {
		// Best-effort subscribe of a small public feed so the poller has
		// something to fetch. The endpoint is idempotent (409 on repeat),
		// which we deliberately accept alongside 2xx so re-runs against
		// the same DB don't fail this hook.
		const res = await ctx.post('/api/v1/feeds', {
			data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
		});
		expect(res.ok() || res.status() === 409).toBeTruthy();
	} finally {
		await ctx.dispose();
	}
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

	// Plan 18: the keyboard-selection highlight is gated on input mode
	// being 'keyboard'. Before the first keystroke we're still in
	// 'mouse' mode (the safe default) so no row paints highlighted.
	// Plan 22 / T2: `selectedId` no longer falls through to visible[0],
	// so even after switching to keyboard mode the first row only
	// highlights once the user has actually pressed j/k.
	await expect(rows.first()).not.toHaveClass(/is-selected/);

	// First j picks the first entry (Plan 22 / T2).
	await page.keyboard.press('j');
	await expect(rows.first()).toHaveClass(/is-selected/);
	// Second j advances to the second entry.
	await page.keyboard.press('j');
	await expect(rows.nth(1)).toHaveClass(/is-selected/);
	await page.keyboard.press('k');
	await expect(rows.first()).toHaveClass(/is-selected/);
});

test('m toggles read on the selected entry', async ({ page }) => {
	await page.goto('/');
	const rows = page.locator('.entry');
	await expect(rows.first()).toBeVisible({ timeout: 20_000 });

	// Capture an element handle to the originally-selected row. After
	// `m`, the entry leaves the unread filter and the locator at index 0
	// would silently start pointing at a different row, so we assert on
	// the captured handle's connectedness instead — that's the actual
	// behaviour we care about (the marked entry drops off the river).
	const firstRowHandle = await rows.first().elementHandle();
	expect(firstRowHandle).not.toBeNull();

	// Plan 22 / T2: pick the first row before pressing `m`. Selection is
	// no longer implicit on first paint, so a bare `m` would no-op.
	await page.keyboard.press('j');

	await page.keyboard.press('m');
	await expect
		.poll(async () => await firstRowHandle!.evaluate((el) => el.isConnected), {
			timeout: 5_000
		})
		.toBe(false);
});
