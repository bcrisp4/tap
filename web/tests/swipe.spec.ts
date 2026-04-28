import { test, expect, type APIRequestContext } from '@playwright/test';

// Plan 18 / T5 + T6 — synthetic swipe gestures via dispatched
// TouchEvents. Playwright's `touchscreen.tap()` exists but there's no
// `swipe` primitive; we hand-roll the touch sequence by dispatching
// touchstart/touchmove/touchend on the target element.

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

// Dispatch synthetic touchstart → touchmove → touchend on `selector`
// from (sx, sy) to (ex, ey). We can't construct TouchEvent in a single
// `evaluate` arg in a typed way without the lib types; use loose any
// to keep the helper portable.
async function syntheticSwipe(
	page: import('@playwright/test').Page,
	selector: string,
	sx: number,
	sy: number,
	ex: number,
	ey: number
) {
	await page.evaluate(
		([sel, sxArg, syArg, exArg, eyArg]) => {
			const el = document.querySelector(sel as string) as HTMLElement | null;
			if (!el) throw new Error('element not found: ' + sel);
			const sxN = sxArg as number;
			const syN = syArg as number;
			const exN = exArg as number;
			const eyN = eyArg as number;
			const TouchCtor = (
				window as unknown as { Touch?: new (init: unknown) => unknown }
			).Touch;
			const make = (clientX: number, clientY: number) => {
				if (TouchCtor) {
					return new TouchCtor({
						identifier: 1,
						target: el,
						clientX,
						clientY,
						pageX: clientX,
						pageY: clientY,
						radiusX: 1,
						radiusY: 1,
						rotationAngle: 0,
						force: 1
					});
				}
				return { clientX, clientY, pageX: clientX, pageY: clientY, identifier: 1, target: el };
			};
			const dispatch = (type: string, x: number, y: number) => {
				const t = make(x, y);
				const ev: any = new Event(type, { bubbles: true, cancelable: true });
				ev.touches = type === 'touchend' ? [] : [t];
				ev.changedTouches = [t];
				ev.targetTouches = type === 'touchend' ? [] : [t];
				el.dispatchEvent(ev);
			};
			dispatch('touchstart', sxN, syN);
			// A few interpolated moves so the state machine gets fresh dx
			// readings before the end fires.
			const steps = 5;
			for (let i = 1; i <= steps; i++) {
				const x = sxN + ((exN - sxN) * i) / steps;
				const y = syN + ((eyN - syN) * i) / steps;
				dispatch('touchmove', x, y);
			}
			dispatch('touchend', exN, eyN);
		},
		[selector, sx, sy, ex, ey] as const
	);
}

test('mobile: right-swipe on a row marks it read', async ({ page, request }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await ensureUnread(request);

	await page.goto('/');
	const firstRow = page.locator('.entry').first();
	await expect(firstRow).toBeVisible({ timeout: 20_000 });

	// Capture the row's id from the embedded /entry/<id> href so we
	// can verify the API state after the gesture.
	const href = await firstRow.locator('a.hit').getAttribute('href');
	expect(href).toMatch(/^\/entry\/\d+$/);
	const id = Number(href!.split('/').pop());

	await syntheticSwipe(page, '.entry', 30, 100, 200, 100);

	// The mutation flips read=true. Within the optimistic update window
	// the entry leaves the unread filter — poll the API directly to
	// avoid relying on the SPA's re-render timing.
	await expect
		.poll(
			async () =>
				(await request.get(`/api/v1/entries/${id}`).then((r) => r.json())).read,
			{ timeout: 3_000 }
		)
		.toBe(true);
});

test('mobile: left-swipe in the reader navigates to the next sibling entry', async ({
	page,
	request
}) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await ensureUnread(request);

	// Pull two unread sibling ids straight from the API so we know what
	// the SPA's cached list will resolve to. The SPA's useEntries call
	// uses the same default ordering, so list[0] is the "current" and
	// list[1] is "next".
	const list = (await request
		.get('/api/v1/entries?status=unread&limit=10')
		.then((r) => r.json())) as { data: Array<{ id: number }> };
	if (list.data.length < 2) test.skip();
	const currentId = list.data[0].id;
	const nextId = list.data[1].id;

	await page.goto('/entry/' + currentId);
	await expect(page.getByTestId('reader-body')).toBeVisible({ timeout: 15_000 });

	await syntheticSwipe(page, '.reader-scroller', 320, 400, 60, 400);

	// goto fires synchronously inside the swipe handler; the SPA route
	// transitions to /entry/<nextId>. Wait for the URL to settle.
	await expect(page).toHaveURL(new RegExp('/entry/' + nextId + '$'));
});
