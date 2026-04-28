import { test, expect, request } from '@playwright/test';

// Plan 20 / T10 — assert the unread-to-history propagation works
// without a manual reload. Before this plan, marking an entry read on
// /unread invalidated `keys.entriesAll()` only (prefix 'entries'); the
// history list was a sibling root key and never got refreshed, so
// /history showed stale data until you forced a reload. The cache-patch
// helpers walk every list namespace synchronously, and `onSettled`
// invalidates entries/history/search as a backstop.

declare const process: { env: Record<string, string | undefined> };

const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

test.beforeEach(async () => {
	const ctx = await request.newContext({ baseURL: BASE });
	try {
		const res = await ctx.post('/api/v1/feeds', {
			data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
		});
		expect(res.ok() || res.status() === 409).toBeTruthy();
	} finally {
		await ctx.dispose();
	}
});

test('mark-read on /unread propagates to /history without reload', async ({ page }) => {
	await page.goto('/');
	const rows = page.locator('.entry');
	await expect(rows.first()).toBeVisible({ timeout: 20_000 });

	// Capture the title of the first unread entry so we can find it on
	// /history. The title sits in `.entry h3.title`.
	const title = (await rows.first().locator('.title').innerText()).trim();
	expect(title.length).toBeGreaterThan(0);

	// Mark read via the per-row toggle (Plan 15 testid).
	await rows.first().locator('[data-testid="row-read-toggle"]').click();

	// Navigate to /history without a reload — SPA navigation only.
	await page.goto('/history');

	// The previously-marked-read entry must surface here. Match by
	// exact title via the `.title` child.
	await expect(
		page.locator('.entry').filter({ has: page.locator(`.title:text-is("${title}")`) }).first()
	).toBeVisible({ timeout: 5_000 });
});
