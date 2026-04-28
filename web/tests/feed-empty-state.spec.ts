import { test, expect, request } from '@playwright/test';

// Plan 22 / T4: the empty-state copy on /feeds/[id] is now just
// "no entries" — the prior tagline ("the poller may still be catching
// up") read as an excuse, not as guidance.

declare const process: { env: Record<string, string | undefined> };
const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

let emptyFeedId: number;

test.beforeAll(async () => {
	const ctx = await request.newContext({ baseURL: BASE });
	try {
		// A bogus feed URL the poller can't fetch — guarantees zero
		// entries even after the dispatcher's first tick.
		const res = await ctx.post('/api/v1/feeds', {
			data: {
				feed_url: 'https://example.invalid/never-resolves.atom',
				title: 'plan-22-empty (e2e)'
			}
		});
		if (res.status() === 201) {
			const body = (await res.json()) as { id: number };
			emptyFeedId = body.id;
		} else {
			// Already exists — look it up.
			const list = await ctx.get('/api/v1/feeds');
			const feeds = ((await list.json()) as { data: Array<{ id: number; feed_url: string }> }).data;
			const found = feeds.find((f) => f.feed_url === 'https://example.invalid/never-resolves.atom');
			expect(found).toBeDefined();
			emptyFeedId = found!.id;
		}
	} finally {
		await ctx.dispose();
	}
});

test('empty feed renders concise empty-state copy', async ({ page }) => {
	await page.goto(`/feeds/${emptyFeedId}`);
	await expect(page.locator('.entries .empty.mono')).toHaveText('no entries');
});
