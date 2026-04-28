import { test, expect } from '@playwright/test';

// Plan 22 / T8: search placeholder reads "Search posts…" — the prior
// "search the river…" leaned hard on metaphor over function.

test('search input placeholder reads "Search posts…"', async ({ page }) => {
	await page.goto('/search');
	const input = page.locator('input[type="search"], input[placeholder]').first();
	await expect(input).toHaveAttribute('placeholder', 'Search posts…');
});
