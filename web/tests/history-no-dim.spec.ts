import { test, expect, request } from '@playwright/test';

// Plan 22 / T7: read entries on /history render at full ink. The
// `is-read` dim that fades title + meta to `--ink-3` is suppressed
// when EntryRow's `dimRead` prop is false (history opts in; the
// unread river keeps the dim because users want a visual cue when
// they undo a mark-read).

declare const process: { env: Record<string, string | undefined> };
const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

test.beforeEach(async () => {
	const ctx = await request.newContext({ baseURL: BASE });
	try {
		const res = await ctx.post('/api/v1/feeds', {
			data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
		});
		expect(res.ok() || res.status() === 409).toBeTruthy();
		// Make sure at least one entry is read so /history has a row.
		const list = await ctx.get('/api/v1/entries?limit=1');
		const body = (await list.json()) as { data: Array<{ id: number }> };
		if (body.data && body.data.length > 0) {
			await ctx.put(`/api/v1/entries/${body.data[0].id}`, { data: { read: true } });
		}
	} finally {
		await ctx.dispose();
	}
});

test('history page renders read entries at full ink', async ({ page }) => {
	await page.goto('/history');
	await expect(page.locator('.entry').first()).toBeVisible({ timeout: 20_000 });

	// History entries are read by definition. The title color must
	// match the unread page's title color (full ink, var(--ink)).
	const historyColor = await page
		.locator('.entry .title')
		.first()
		.evaluate((el) => getComputedStyle(el).color);

	await page.goto('/');
	await expect(page.locator('.entry').first()).toBeVisible({ timeout: 20_000 });
	const unreadColor = await page
		.locator('.entry .title')
		.first()
		.evaluate((el) => getComputedStyle(el).color);

	expect(historyColor).toBe(unreadColor);
});
