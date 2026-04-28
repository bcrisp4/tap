import { test, expect, type APIRequestContext } from '@playwright/test';

// Plan 22 / T5: the desktop reader no longer mounts an empty
// `<aside.reader-rail-collapsed>` between the sidebar and the reader
// pane. The back-to-unread affordance lives in the ReaderHeader's
// "back" button (`.reader-back`); the rail-back button + sibling-count
// hook is gone.

interface Entry {
	id: number;
}

async function seedAndFetchEntryID(request: APIRequestContext): Promise<number> {
	const subscribe = await request.post('/api/v1/feeds', {
		data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
	});
	if (subscribe.status() !== 201 && subscribe.status() !== 409) {
		throw new Error(`feed subscribe failed: ${subscribe.status()}`);
	}
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		const res = await request.get('/api/v1/entries?limit=1');
		if (res.ok()) {
			const body = (await res.json()) as { data: Entry[] };
			if (body.data && body.data.length > 0) return body.data[0].id;
		}
		await new Promise((r) => setTimeout(r, 1_000));
	}
	throw new Error('no entries available within timeout');
}

test('desktop reader has no collapsed-rail column', async ({ page, request }) => {
	const id = await seedAndFetchEntryID(request);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	// The empty-grey rail strip is gone.
	await expect(page.locator('aside.reader-rail-collapsed')).toHaveCount(0);
	// The reader header's back button remains the back affordance.
	await expect(page.locator('.reader-header .reader-back')).toBeVisible();
});
