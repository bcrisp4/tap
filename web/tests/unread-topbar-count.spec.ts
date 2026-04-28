import { test, expect, request } from '@playwright/test';

// Plan 22 / T3: the unread count appears in exactly one place — the
// sidebar's "Unread" nav badge. The desktop TopBar used to render a
// duplicate "<unread> of <total>" counter to the right of the title;
// that's gone now. The mobile MobileTopBar keeps its count badge
// (different chrome, different consumer).

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

test('TopBar on /unread does not duplicate the sidebar unread count', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/');
	// Sidebar shows the Unread nav badge.
	await expect(page.locator('aside.tap-sidebar .nav-item.active .badge')).toBeVisible();
	// TopBar (desktop chrome) does NOT render a count span.
	await expect(page.locator('.tap-topbar .count')).toHaveCount(0);
});
