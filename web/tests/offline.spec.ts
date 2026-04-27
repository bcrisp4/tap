import { test, expect, request } from '@playwright/test';

// Plan 12 e2e — service worker registration + offline rendering.
//
// The service worker is only emitted in a production build (Vite dev
// strips it), so this spec is meant to be run against the embedded Go
// binary serving web/build/. The default `npm run e2e` against
// `npm run dev` will skip the SW assertions; tests that depend on it
// no-op-out unless `TAP_E2E_BASE` is set, matching the rest of the
// e2e suite's convention.

declare const process: { env: Record<string, string | undefined> };

const PROD = !!process.env.TAP_E2E_BASE;
const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

test.beforeEach(async () => {
	const ctx = await request.newContext({ baseURL: BASE });
	try {
		// Same idempotent seed pattern as unread.spec.ts.
		const res = await ctx.post('/api/v1/feeds', {
			data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
		});
		expect(res.ok() || res.status() === 409).toBeTruthy();
	} finally {
		await ctx.dispose();
	}
});

test('manifest exposes icons + iOS-friendly metadata', async ({ page }) => {
	await page.goto('/');
	const res = await page.request.get('/manifest.webmanifest');
	expect(res.ok()).toBeTruthy();
	const manifest = (await res.json()) as {
		name?: string;
		short_name?: string;
		display?: string;
		theme_color?: string;
		background_color?: string;
		icons?: Array<{ src: string; sizes: string; type: string; purpose?: string }>;
	};
	expect(manifest.name).toBe('Tap');
	expect(manifest.short_name).toBe('Tap');
	expect(manifest.display).toBe('standalone');
	expect(manifest.theme_color).toBe('#002FA7');
	expect(manifest.background_color).toBe('#fafaf7');
	expect(manifest.icons).toEqual(
		expect.arrayContaining([
			expect.objectContaining({ src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' }),
			expect.objectContaining({ src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' })
		])
	);
});

test('app.html wires apple-touch-icon + apple-mobile-web-app-capable', async ({ page }) => {
	await page.goto('/');
	const apple = await page
		.locator('link[rel="apple-touch-icon"]')
		.getAttribute('href');
	expect(apple).toMatch(/icon-192\.png$/);
	const capable = await page
		.locator('meta[name="apple-mobile-web-app-capable"]')
		.getAttribute('content');
	expect(capable).toBe('yes');
});

test.describe('service worker (production build only)', () => {
	test.skip(!PROD, 'service worker is only emitted in the production build');

	test('service worker registers + activates', async ({ page }) => {
		await page.goto('/');
		await page.waitForFunction(
			async () => {
				const r = await navigator.serviceWorker.ready;
				return !!r.active;
			},
			undefined,
			{ timeout: 15_000 }
		);
	});

	test('shell + entries render after a reload while offline', async ({ page, context }) => {
		await page.goto('/');
		await expect(page.getByTestId('river')).toBeVisible({ timeout: 20_000 });

		// Wait for SW to take control + cache the shell + first list.
		await page.waitForFunction(async () => {
			const r = await navigator.serviceWorker.ready;
			return !!r.active && navigator.serviceWorker.controller !== null;
		});
		await page.waitForTimeout(2_000);

		await context.setOffline(true);
		await page.reload();
		await expect(page.getByTestId('river')).toBeVisible({ timeout: 10_000 });
	});

	test('cached entry remains readable offline', async ({ page, context }) => {
		await page.goto('/');
		const rows = page.locator('.entry');
		await expect(rows.first()).toBeVisible({ timeout: 20_000 });

		// Click into an entry while online so its detail JSON + any
		// proxy media land in the SW caches.
		await rows.first().click();
		await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });
		await page.waitForTimeout(1_500);

		await context.setOffline(true);
		await page.reload();
		await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 10_000 });
	});
});
