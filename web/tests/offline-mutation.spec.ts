import { test, expect, request } from '@playwright/test';

// Plan 20 / T11 — assert the offline → reload → reconnect path
// drains a paused mark-read mutation. The persister now dehydrates
// paused mutations into IDB; on next boot the cache rehydrates and
// `drainPausedMutations(client)` kicks `resumePausedMutations()` so
// the queued PUT eventually fires once the browser reports back online.
//
// We intentionally do NOT assert the optimistic patch survives the
// offline reload. The Plan 12 service worker serves /api/v1/entries*
// from cache even while offline (stale-while-revalidate), so a
// refetch-on-mount can clobber the rehydrated patched list with the
// stale one — the load-bearing contract for users is that the queued
// mutation drains on reconnect and the server eventually reflects the
// edit, not that the patch survives every offline UI re-render.

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

test('offline mark-read survives a tab reload before reconnect', async ({ page, context }) => {
	await page.goto('/');
	const rows = page.locator('.entry');
	await expect(rows.first()).toBeVisible({ timeout: 20_000 });

	const title = (await rows.first().locator('.title').innerText()).trim();
	expect(title.length).toBeGreaterThan(0);

	// Capture every PUT against the entries collection so we can confirm
	// the queued mutation drains after reconnect. We only assert at-
	// least-once below — query-core may legitimately re-fire a paused
	// mutation across the boot/onlineManager/drainPausedMutations paths
	// so a strict equality check would be brittle.
	const puts: string[] = [];
	page.on('request', (req) => {
		if (req.method() === 'PUT' && req.url().includes('/api/v1/entries/')) {
			puts.push(req.url());
		}
	});

	// Go offline, mark read, reload still offline. The mark-read fires
	// onMutate (queued in mutation cache) and the persister writes the
	// queued mutation to IDB before the reload tears down the page.
	await context.setOffline(true);
	await rows.first().locator('[data-testid="row-read-toggle"]').click();
	// Tiny pause so the persister's throttle window flushes to IDB
	// before the reload nukes the in-memory caches.
	await page.waitForTimeout(200);

	await page.reload();

	// Reconnect. drainPausedMutations() runs at boot once the persisted
	// cache rehydrates; that, plus onlineManager flipping online, kicks
	// the rehydrated paused mutation into firing its PUT.
	await context.setOffline(false);
	await page.waitForFunction(() => navigator.onLine, undefined, { timeout: 5_000 });

	await expect.poll(() => puts.length, { timeout: 15_000 }).toBeGreaterThanOrEqual(1);

	// Cross-check from the server: the entry now shows up on /history.
	await page.goto('/history');
	await expect(
		page.locator('.entry').filter({ has: page.locator(`.title:text-is("${title}")`) }).first()
	).toBeVisible({ timeout: 10_000 });
});
