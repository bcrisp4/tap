import { test, expect } from '@playwright/test';

// Lightweight checks for things every page must always have. The
// data-driven assertions live in unread.spec.ts.

test('home renders the wordmark', async ({ page }) => {
	await page.goto('/');
	const wordmark = page.locator('.wordmark');
	await expect(wordmark).toBeVisible();
	await expect(wordmark).toContainText('tap');
});

test('home renders the poll-status surface', async ({ page }) => {
	await page.goto('/');
	await expect(page.getByTestId('poll-status')).toBeVisible();
});
