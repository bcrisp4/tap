import { test, expect, type APIRequestContext } from '@playwright/test';

// Plan 22 / T6: the reader body content widens to 820px (was 680px).

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

test('reader body max-width is 820px', async ({ page, request }) => {
	const id = await seedAndFetchEntryID(request);
	await page.setViewportSize({ width: 1600, height: 900 });
	await page.goto(`/entry/${id}`);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	const maxWidth = await page
		.getByTestId('reader-body')
		.evaluate((el) => getComputedStyle(el).maxWidth);
	expect(maxWidth).toBe('820px');
});
