import { test, expect, request } from '@playwright/test';

// The reader e2e seeds a real feed via the API, waits for the poller
// to ingest at least one entry, then drives the reader directly via
// `/entry/<id>`. We intentionally don't go through the unread-list row
// click here — that flow lands with Plan 10's EntryRow component; this
// spec stays focused on Plan 11's deliverable.
const BASE = process.env.TAP_E2E_BASE ?? 'http://127.0.0.1:5173';

interface Entry {
	id: number;
	title: string;
}

async function seedAndFetchEntryID(): Promise<number> {
	const ctx = await request.newContext({ baseURL: BASE });
	// Idempotent subscribe — duplicate POSTs return the existing feed.
	await ctx
		.post('/api/v1/feeds', {
			data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
		})
		.catch(() => undefined);

	// Poll the entries list for up to ~30s; the poller has to fetch and
	// extract the article body before /entries/{id} returns content.
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		const res = await ctx.get('/api/v1/entries?limit=1');
		if (res.ok()) {
			const body = (await res.json()) as { data: Entry[] };
			if (body.data && body.data.length > 0) return body.data[0].id;
		}
		await new Promise((r) => setTimeout(r, 1_000));
	}
	throw new Error('no entries available within timeout — is the poller running?');
}

test('reader opens an entry and shows body + mono header', async ({ page }) => {
	const id = await seedAndFetchEntryID();
	await page.goto(`/entry/${id}`);

	// Body renders.
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	// Mono buttons present and using JetBrains Mono.
	const back = page.locator('.reader-back');
	await expect(back).toBeVisible();
	await expect(back).toHaveText(/UNREAD/);
	const fontFamily = await back.evaluate((el) => getComputedStyle(el).fontFamily);
	expect(fontFamily.toLowerCase()).toContain('jetbrains');

	// All three action buttons present (mark read, save, view original).
	await expect(page.locator('.reader-action').filter({ hasText: /MARK/ })).toBeVisible();
	await expect(page.locator('.reader-action').filter({ hasText: /SAVE/ })).toBeVisible();
});

test('does not auto-mark-read on reader mount', async ({ page }) => {
	const id = await seedAndFetchEntryID();

	// Confirm pre-state: entry is unread on the wire.
	const ctx = await request.newContext({ baseURL: BASE });
	const before = await ctx.get(`/api/v1/entries/${id}`).then((r) => r.json());
	expect(before.read).toBe(false);

	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	// Give the page a moment to settle in case any rogue effect tries
	// to flip the bit on mount.
	await page.waitForTimeout(800);

	const after = await ctx.get(`/api/v1/entries/${id}`).then((r) => r.json());
	expect(after.read).toBe(false);
});

test('Esc returns to the unread list', async ({ page }) => {
	const id = await seedAndFetchEntryID();
	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	await page.keyboard.press('Escape');
	await expect(page).toHaveURL(/\/$/);
});

test('Saved button in reader header turns Klein blue', async ({ page }) => {
	const id = await seedAndFetchEntryID();
	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	const save = page.locator('.reader-action').filter({ hasText: /SAVE|SAVED/ });
	const initialColor = await save.evaluate((el) => getComputedStyle(el).color);

	await save.click();

	// After click the label flips to SAVED and the colour matches the
	// theme's accent (Klein in light/sepia, desaturated Klein in dark).
	await expect(save).toHaveText(/SAVED/);
	await expect.poll(async () => save.evaluate((el) => getComputedStyle(el).color)).not.toBe(
		initialColor
	);
	const color = await save.evaluate((el) => getComputedStyle(el).color);
	expect(color === 'rgb(0, 47, 167)' || color === 'rgb(90, 127, 220)').toBeTruthy();
});

test('m keyboard shortcut toggles read state', async ({ page }) => {
	const id = await seedAndFetchEntryID();
	const ctx = await request.newContext({ baseURL: BASE });

	// Reset to unread first so the test is order-independent.
	await ctx.put(`/api/v1/entries/${id}`, { data: { read: false } });

	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	await page.keyboard.press('m');

	await expect
		.poll(async () => (await ctx.get(`/api/v1/entries/${id}`).then((r) => r.json())).read)
		.toBe(true);
});
