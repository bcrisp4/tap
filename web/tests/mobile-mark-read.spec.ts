import { test, expect, type APIRequestContext } from '@playwright/test';

// Plan 18 / T3 — mobile mark-read flicker.
//
// Pre-fix bug: tapping a row's mark-read dot at the mobile viewport
// either does nothing or sometimes opens the entry instead. The dot
// was hidden on mobile, and even after un-hiding it the touch+click
// pair plus a stretched <a> sibling caused the navigation handler to
// steal the event from the toggle button.
//
// This test asserts the cure: at the mobile viewport the read-dot is
// tappable, a single tap flips the row's read state, and the route
// stays on `/` (the click does NOT also open the reader).

interface Entry {
	id: number;
	read: boolean;
}

async function ensureUnread(request: APIRequestContext): Promise<number> {
	const subscribe = await request.post('/api/v1/feeds', {
		data: { feed_url: 'https://jvns.ca/atom.xml', title: 'jvns.ca (e2e)' }
	});
	if (subscribe.status() !== 201 && subscribe.status() !== 409) {
		throw new Error(
			'feed subscribe failed: ' + subscribe.status() + ' ' + (await subscribe.text())
		);
	}
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		const res = await request.get('/api/v1/entries?status=unread&limit=1');
		if (res.ok()) {
			const body = (await res.json()) as { data: Entry[] };
			if (body.data && body.data.length > 0) {
				// Force the entry back to unread so the test starts from a
				// known state regardless of prior runs.
				await request.put(`/api/v1/entries/${body.data[0].id}`, {
					data: { read: false }
				});
				return body.data[0].id;
			}
		}
		await new Promise((r) => setTimeout(r, 1_000));
	}
	throw new Error('no entries available within timeout');
}

test('mobile: tapping a row read-dot toggles read without opening the reader', async ({
	page,
	request
}) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await ensureUnread(request);

	await page.goto('/');
	const dot = page.getByTestId('row-read-toggle').first();
	await expect(dot).toBeVisible({ timeout: 20_000 });

	// Initially unread: aria-pressed='true' (the button represents
	// "is unread", per the EntryRow contract).
	await expect(dot).toHaveAttribute('aria-pressed', 'true');

	await dot.tap();

	// Single tap flips the state to read. Within 500 ms the row either
	// drops off the unread filter (post-invalidate) or its dot reflects
	// aria-pressed='false'. Either is the win condition — we accept the
	// row vanishing since the entries query refetches on settle.
	await expect
		.poll(
			async () => {
				const count = await dot.count();
				if (count === 0) return 'gone';
				return await dot.getAttribute('aria-pressed');
			},
			{ timeout: 2_000 }
		)
		.toMatch(/^(false|gone)$/);

	// The route must NOT have advanced to /entry/<id>. The tap-to-open
	// stretched anchor must yield to the read-dot button.
	expect(new URL(page.url()).pathname).toBe('/');
});
