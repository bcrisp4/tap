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
		// Poll until the dispatcher has ingested at least one entry,
		// then explicitly mark it read so /history has a row to render.
		// On a fresh DB the prior "if there's an entry, mark it read"
		// shortcut would silently no-op and leave /history empty.
		const deadline = Date.now() + 30_000;
		let entryId: number | undefined;
		while (Date.now() < deadline) {
			const list = await ctx.get('/api/v1/entries?limit=1');
			if (list.ok()) {
				const body = (await list.json()) as { data: Array<{ id: number }> };
				if (body.data && body.data.length > 0) {
					entryId = body.data[0].id;
					break;
				}
			}
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(entryId, 'no entries available within timeout').toBeDefined();
		const markRead = await ctx.put(`/api/v1/entries/${entryId}`, { data: { read: true } });
		expect(markRead.ok()).toBeTruthy();
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
