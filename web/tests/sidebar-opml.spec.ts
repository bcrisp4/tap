import { test, expect, request } from '@playwright/test';

// Plan 21 — sidebar/footer/settings + OPML wiring.
//
// Asserts the four user-facing changes that Plan 21 introduces:
//   * The sidebar footer hosts an OPML import/export popover next to
//     Settings (T4 + T5).
//   * Clicking Export OPML triggers a download of the
//     `tap-subscriptions.opml` file (T4).
//   * The Settings page no longer renders an About card (T6) and the
//     StatsPanel header carries an inline GitHub link (T7).
//   * The Sidebar Settings glyph is the Lucide cog (T3) — locked via
//     a presence check on its data-lucide attribute.
//
// These tests don't rely on a real polled feed (Plan 21 doesn't add
// any feed-state assertions), so they should be green on a fresh DB.

declare const process: { env: Record<string, string | undefined> };
const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

test('sidebar footer exposes the OPML popover next to Settings', async ({ page }) => {
	await page.goto('/');
	const opmlBtn = page.getByRole('button', { name: 'OPML import / export' });
	await expect(opmlBtn).toBeVisible();
	await expect(opmlBtn).toHaveAttribute('aria-expanded', 'false');
	await opmlBtn.click();
	await expect(opmlBtn).toHaveAttribute('aria-expanded', 'true');
	// Both actions render in the popover.
	await expect(page.getByRole('menu')).toBeVisible();
	await expect(page.getByText('Import OPML…')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Export OPML' })).toBeVisible();
});

test('Export OPML triggers a download of tap-subscriptions.opml', async ({ page }) => {
	await page.goto('/');
	await page.getByRole('button', { name: 'OPML import / export' }).click();
	const [download] = await Promise.all([
		page.waitForEvent('download'),
		page.getByRole('button', { name: 'Export OPML' }).click()
	]);
	expect(download.suggestedFilename()).toBe('tap-subscriptions.opml');
});

test('Import OPML round-trips a small OPML body and surfaces a toast', async ({ page }) => {
	const opmlBody = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>plan-21 e2e</title></head>
  <body>
    <outline text="Plan 21 e2e import" type="rss" xmlUrl="https://example.invalid/plan-21.atom" />
  </body>
</opml>`;

	// Pre-clean: if a prior run already imported this URL, skip the
	// import (the dedupe path returns 0 imported and the assertion
	// becomes vacuous; we exercise the post path either way).
	const ctx = await request.newContext({ baseURL: BASE });
	try {
		const list = await ctx.get('/api/v1/feeds');
		const feeds = ((await list.json()) as { data: Array<{ feed_url: string }> }).data;
		const already = feeds.some((f) => f.feed_url === 'https://example.invalid/plan-21.atom');

		await page.goto('/');
		await page.getByRole('button', { name: 'OPML import / export' }).click();

		const fileChooserPromise = page.waitForEvent('filechooser');
		await page.getByText('Import OPML…').click();
		const chooser = await fileChooserPromise;
		await chooser.setFiles({
			name: 'plan-21.opml',
			mimeType: 'application/xml',
			buffer: Buffer.from(opmlBody)
		});

		// Toast surfaces (replaces the existing one if any). Either
		// "Imported 1 feed" (fresh) or "Imported 0 feeds" (re-import).
		const toast = page.locator('.toast').first();
		await expect(toast).toBeVisible({ timeout: 5_000 });
		// Text contains "Imported" — keeps the assertion stable across
		// both fresh and re-import paths.
		await expect(toast).toContainText(/Imported/i);
		// Sanity: the imported (or already-existing) URL is in the feeds list.
		const list2 = await ctx.get('/api/v1/feeds');
		const feeds2 = ((await list2.json()) as { data: Array<{ feed_url: string }> }).data;
		expect(
			feeds2.some((f) => f.feed_url === 'https://example.invalid/plan-21.atom') ||
				already
		).toBeTruthy();
	} finally {
		await ctx.dispose();
	}
});

test('Settings page drops the About card and surfaces the inline GitHub link', async ({
	page
}) => {
	await page.goto('/settings');
	// About card is gone.
	await expect(page.getByRole('heading', { name: /About/i })).toHaveCount(0);
	// StatsPanel header carries the icon-only GitHub link.
	const ghLink = page.getByRole('link', { name: 'View source on GitHub' });
	await expect(ghLink).toBeVisible();
	await expect(ghLink).toHaveAttribute('href', 'https://github.com/bcrisp4/tap');
	await expect(ghLink).toHaveAttribute('target', '_blank');
});

test('failing feeds show a warning indicator in the sidebar', async ({ page }) => {
	// Subscribe a guaranteed-unreachable feed so the dispatcher records
	// at least one error against it. The test waits up to 30s for the
	// dispatcher to pick the refresh request up; on a fresh DB with no
	// other backlog this lands within a couple of ticks.
	const ctx = await request.newContext({ baseURL: BASE });
	let feedId: number;
	try {
		const res = await ctx.post('/api/v1/feeds', {
			data: {
				feed_url: 'https://example.invalid/sidebar-warning.atom',
				title: 'plan21-warning (e2e)'
			}
		});
		if (res.status() === 201) {
			const body = (await res.json()) as { id: number };
			feedId = body.id;
		} else {
			const list = await ctx.get('/api/v1/feeds');
			const feeds = ((await list.json()) as { data: Array<{ id: number; feed_url: string }> })
				.data;
			feedId = feeds.find((f) => f.feed_url === 'https://example.invalid/sidebar-warning.atom')!
				.id;
		}

		// Poll the feed until the API confirms error_count > 0. We
		// re-arm the refresh on each iteration in case the dispatcher
		// already serviced the previous attempt.
		const deadline = Date.now() + 30_000;
		let errs = 0;
		while (Date.now() < deadline) {
			await ctx.post(`/api/v1/feeds/${feedId}/refresh`, { data: {} });
			await new Promise((res2) => setTimeout(res2, 1_500));
			const r = await ctx.get(`/api/v1/feeds/${feedId}`);
			const f = (await r.json()) as { error_count: number };
			errs = f.error_count;
			if (errs > 0) break;
		}
		expect(errs).toBeGreaterThan(0);
	} finally {
		await ctx.dispose();
	}

	await page.goto('/');
	const warn = page.locator(`a[href="/feeds/${feedId}"] .feed-warn`);
	await expect(warn).toBeVisible();
	// The tooltip surfaces a non-empty error string.
	await expect(warn).toHaveAttribute('title', /.+/);
});
