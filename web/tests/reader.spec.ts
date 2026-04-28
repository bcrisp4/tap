import { test, expect, type APIRequestContext } from '@playwright/test';

// The reader e2e seeds a real feed via the API, waits for the poller
// to ingest at least one entry, then drives the reader directly via
// `/entry/<id>`. We intentionally don't go through the unread-list row
// click here — that flow lands with Plan 10's EntryRow component; this
// spec stays focused on Plan 11's deliverable.
//
// The playwright `request` fixture inherits `baseURL` from
// playwright.config.ts so we don't need to re-read TAP_E2E_BASE here.

interface Entry {
	id: number;
	title: string;
	read: boolean;
	saved: boolean;
}

async function seedAndFetchEntryID(request: APIRequestContext): Promise<number> {
	// First run returns 201; subsequent runs hit 409 because the feed
	// already exists. Either is fine. Anything else is a real error and
	// we'd rather see it than degrade into the 30-s polling timeout.
	const subscribe = await request.post('/api/v1/feeds', {
		data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
	});
	if (subscribe.status() !== 201 && subscribe.status() !== 409) {
		throw new Error(
			`feed subscribe failed: ${subscribe.status()} ${await subscribe.text()}`
		);
	}

	// Poll the entries list for up to ~30s; the poller has to fetch and
	// extract the article body before /entries/{id} returns content.
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		const res = await request.get('/api/v1/entries?limit=1');
		if (res.ok()) {
			const body = (await res.json()) as { data: Entry[] };
			if (body.data && body.data.length > 0) return body.data[0].id;
		}
		await new Promise((r) => setTimeout(r, 1_000));
	}
	throw new Error('no entries available within timeout — is the poller running?');
}

test('reader opens an entry and shows body + mono header', async ({ page, request }) => {
	const id = await seedAndFetchEntryID(request);
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

test('auto-marks read on reader mount (Plan 16 invariant)', async ({ page, request }) => {
	// Plan 16 deliberately INVERTS Plan 11's "no auto-mark-read" rule.
	// Opening the reader is now treated as the user's signal that they
	// have engaged with the entry, so the read flag flips on mount.
	const id = await seedAndFetchEntryID(request);

	// Reset to unread; the prior test may have toggled it.
	await request.put(`/api/v1/entries/${id}`, { data: { read: false } });
	const before = (await request.get(`/api/v1/entries/${id}`).then((r) => r.json())) as Entry;
	expect(before.read).toBe(false);

	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	// Within ~250 ms of mount the reader's $effect should have fired
	// the toggleRead mutation. Poll briefly so we don't race the
	// optimistic update / network round-trip.
	await expect
		.poll(
			async () => (await request.get(`/api/v1/entries/${id}`).then((r) => r.json())).read,
			{ timeout: 3_000, intervals: [100, 150, 200, 300] }
		)
		.toBe(true);
});

test('Esc returns to the unread list', async ({ page, request }) => {
	const id = await seedAndFetchEntryID(request);
	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	await page.keyboard.press('Escape');
	await expect(page).toHaveURL(/\/$/);
});

test('Saved button in reader header turns Klein blue', async ({ page, request }) => {
	const id = await seedAndFetchEntryID(request);

	// Reset saved=false so the click is observable as a flip, not a no-op.
	await request.put(`/api/v1/entries/${id}`, { data: { saved: false } });

	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	const save = page.locator('.reader-action').filter({ hasText: /SAVE|SAVED/ });
	await save.click();

	// After click the label flips to SAVED and the colour matches the
	// theme's accent (Klein in light/sepia, desaturated Klein in dark).
	await expect(save).toHaveText(/SAVED/);
	const color = await save.evaluate((el) => getComputedStyle(el).color);
	expect(color === 'rgb(0, 47, 167)' || color === 'rgb(90, 127, 220)').toBeTruthy();
});

test('m keyboard shortcut toggles read state', async ({ page, request }) => {
	const id = await seedAndFetchEntryID(request);

	// Reset to unread first so the auto-mark-read effect has a real
	// transition to perform on mount.
	await request.put(`/api/v1/entries/${id}`, { data: { read: false } });

	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	// Auto-mark-read fires on mount (Plan 16). Wait for read=true to
	// land before pressing `m`, otherwise `m` could race the auto-mark
	// and the resulting state is non-deterministic.
	await expect
		.poll(async () => (await request.get(`/api/v1/entries/${id}`).then((r) => r.json())).read)
		.toBe(true);

	// `m` toggles back to unread.
	await page.keyboard.press('m');

	await expect
		.poll(async () => (await request.get(`/api/v1/entries/${id}`).then((r) => r.json())).read)
		.toBe(false);
});
