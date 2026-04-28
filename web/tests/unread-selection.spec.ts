import { test, expect, request } from '@playwright/test';

// Plan 22 / T2: the unread river paints no row selected on first paint.
// Selection only exists after the user types j/k (or any keyboard nav
// shortcut). The first such press picks the FIRST visible entry; the
// second press is what advances by one. Before this plan, `selectedId`
// fell through to `visible[0]` so the first row painted highlighted as
// soon as input mode flipped to keyboard, and the first j moved to row
// 1 — skipping the user's apparent target.

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

test('unread page has no selected row until j/k is pressed', async ({ page }) => {
	await page.goto('/');
	await expect(page.locator('.entry').first()).toBeVisible({ timeout: 20_000 });
	// On first paint nothing is selected — the keyboard-highlight gate is
	// off (no key pressed yet) AND `selectedId` no longer falls through
	// to `visible[0]`.
	await expect(page.locator('.entry.is-selected')).toHaveCount(0);

	// First j picks the first entry.
	await page.keyboard.press('j');
	await expect(page.locator('.entry').first()).toHaveClass(/is-selected/);

	// Subsequent j moves to the second entry.
	await page.keyboard.press('j');
	await expect(page.locator('.entry').nth(1)).toHaveClass(/is-selected/);

	// k moves back to the first.
	await page.keyboard.press('k');
	await expect(page.locator('.entry').first()).toHaveClass(/is-selected/);
});
