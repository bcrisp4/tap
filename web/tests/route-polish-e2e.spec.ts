import { test, expect, request, type APIRequestContext } from '@playwright/test';

// Plan 22 / T10: composite end-to-end walking every fix in one
// scenario. The narrower per-task specs already cover each behaviour
// in isolation; this one is the safety net catching cross-route
// interactions (e.g. an unread state mutation in T2 leaking into T7's
// history color check).

declare const process: { env: Record<string, string | undefined> };
const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

interface Entry {
	id: number;
}

let emptyFeedId: number;

async function ensureEmptyFeed(ctx: APIRequestContext): Promise<number> {
	const list = await ctx.get('/api/v1/feeds');
	const feeds = ((await list.json()) as { data: Array<{ id: number; feed_url: string }> }).data;
	const found = feeds.find(
		(f) => f.feed_url === 'https://example.invalid/never-resolves.atom'
	);
	if (found) return found.id;
	const created = await ctx.post('/api/v1/feeds', {
		data: {
			feed_url: 'https://example.invalid/never-resolves.atom',
			title: 'plan-22-empty (e2e)'
		}
	});
	expect(created.ok() || created.status() === 409).toBeTruthy();
	const body = (await created.json()) as { id: number };
	return body.id;
}

async function ensureUnreadEntry(ctx: APIRequestContext): Promise<number> {
	const sub = await ctx.post('/api/v1/feeds', {
		data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
	});
	expect(sub.ok() || sub.status() === 409).toBeTruthy();
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		const res = await ctx.get('/api/v1/entries?limit=1');
		if (res.ok()) {
			const body = (await res.json()) as { data: Entry[] };
			if (body.data && body.data.length > 0) {
				// Force at least one entry to be unread so /unread has rows.
				await ctx.put(`/api/v1/entries/${body.data[0].id}`, { data: { read: false } });
				return body.data[0].id;
			}
		}
		await new Promise((r) => setTimeout(r, 1_000));
	}
	throw new Error('no entries available within timeout');
}

test.beforeAll(async () => {
	const ctx = await request.newContext({ baseURL: BASE });
	try {
		emptyFeedId = await ensureEmptyFeed(ctx);
		await ensureUnreadEntry(ctx);
	} finally {
		await ctx.dispose();
	}
});

test('Plan 22: route polish end-to-end', async ({ page }) => {
	await page.setViewportSize({ width: 1600, height: 900 });

	// /unread: no implicit selection on load.
	await page.goto('/');
	await expect(page.locator('.entry').first()).toBeVisible({ timeout: 20_000 });
	await expect(page.locator('.entry.is-selected')).toHaveCount(0);

	// First j picks the first row.
	await page.keyboard.press('j');
	await expect(page.locator('.entry').first()).toHaveClass(/is-selected/);

	// TopBar: no duplicate count.
	await expect(page.locator('.tap-topbar .count')).toHaveCount(0);

	// /feeds/[id]: empty state copy.
	await page.goto(`/feeds/${emptyFeedId}`);
	await expect(page.locator('.entries .empty.mono')).toHaveText('no entries');

	// Reader: no collapsed rail; max-width 820px.
	await page.goto('/');
	await expect(page.locator('.entry').first()).toBeVisible({ timeout: 20_000 });
	await page.locator('.entry').first().click();
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });
	await expect(page.locator('aside.reader-rail-collapsed')).toHaveCount(0);
	const mw = await page
		.getByTestId('reader-body')
		.evaluate((el) => getComputedStyle(el).maxWidth);
	expect(mw).toBe('820px');

	// /history: read entries at full ink (parity with /unread title color).
	await page.goto('/history');
	await expect(page.locator('.entry').first()).toBeVisible({ timeout: 20_000 });
	const historyTitleColor = await page
		.locator('.entry .title')
		.first()
		.evaluate((el) => getComputedStyle(el).color);
	await page.goto('/');
	await expect(page.locator('.entry').first()).toBeVisible({ timeout: 20_000 });
	const unreadTitleColor = await page
		.locator('.entry .title')
		.first()
		.evaluate((el) => getComputedStyle(el).color);
	expect(historyTitleColor).toBe(unreadTitleColor);

	// /search: placeholder copy.
	await page.goto('/search');
	await expect(page.locator('input[type="search"]').first()).toHaveAttribute(
		'placeholder',
		'Search posts…'
	);
});
