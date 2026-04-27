import { test, expect } from '@playwright/test';

test('home renders the wordmark', async ({ page }) => {
	await page.goto('/');
	const wordmark = page.locator('.wordmark');
	await expect(wordmark).toBeVisible();
	await expect(wordmark).toContainText('tap');
});

test('status probe renders', async ({ page }) => {
	await page.goto('/');
	await expect(page.getByTestId('status')).toBeVisible();
});
