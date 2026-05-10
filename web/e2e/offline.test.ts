import { test, expect } from '@playwright/test';

// These tests require the Go server running on port 8084 with a test user.
// The Vite dev server proxies /api to port 8080 (default GO_BACKEND); adjust
// playwright.config.ts webServer or run against the built binary directly.

const USERNAME = process.env.TAP_E2E_USER ?? 'ben';
const PASSWORD = process.env.TAP_E2E_PASS ?? 'test12345';

async function login(page: import('@playwright/test').Page) {
  await page.goto('/');
  await page.waitForSelector('input[name="username"], input[type="text"]', { timeout: 10000 });
  await page.fill('input[name="username"], input[type="text"]', USERNAME);
  await page.fill('input[name="password"], input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  // Wait for login to complete (either unread view or any post-login element).
  await page.waitForFunction(
    () => document.querySelector('.unread-view, [data-testid="unread"], .entry-row, [data-testid="entry-row"]') !== null,
    { timeout: 15000 },
  );
}

test.describe('Offline reading (Scenario 1)', () => {
  test('app shell and entry load from SW cache when offline', async ({ page, context }) => {
    await login(page);

    // Wait for warm-cache to run (2s defer + some fetch time).
    await page.waitForTimeout(5000);

    // Open an entry while online so it's in the SW cache.
    const firstEntry = page.locator('.entry-row, [data-testid="entry-row"]').first();
    await firstEntry.click();
    await page.waitForSelector('.reader-body, [data-testid="reader-body"], .reader, article', { timeout: 10000 });

    // Save the URL for navigating back to.
    const entryUrl = page.url();

    // Go offline.
    await context.setOffline(true);

    // Reload the page.
    await page.reload({ waitUntil: 'domcontentloaded', timeout: 10000 }).catch(() => {});

    // App shell must load (from precache).
    await page.waitForSelector('body', { timeout: 8000 });

    // Navigate back to the entry.
    await page.goto(entryUrl).catch(() => {});

    // Entry body must render (from stale-while-revalidate cache).
    await expect(
      page.locator('.reader-body, [data-testid="reader-body"], .reader, article').first()
    ).toBeVisible({ timeout: 8000 });

    await context.setOffline(false);
  });
});

test.describe('Offline mutation replay (Scenario 2)', () => {
  test('mutations queued offline are replayed on reconnect', async ({ page, context }) => {
    await login(page);
    await page.waitForTimeout(1000);

    await context.setOffline(true);

    // Open an entry — auto-mark-read fires and is queued.
    const firstEntry = page.locator('.entry-row, [data-testid="entry-row"]').first();
    await firstEntry.click();
    await page.waitForTimeout(500);

    // Verify mutation is in the localStorage queue.
    const queueJson = await page.evaluate(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.length > 0 ? localStorage.getItem(keys[0]) : '[]';
    });
    const queue = JSON.parse(queueJson ?? '[]') as unknown[];
    expect(queue.length).toBeGreaterThan(0);

    // Go back online.
    await context.setOffline(false);

    // Wait for drain to complete (queue emptied).
    await page.waitForFunction(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.every(k => JSON.parse(localStorage.getItem(k) ?? '[]').length === 0);
    }, { timeout: 10000 });
  });
});

test.describe('Queue survives reload (Scenario 3)', () => {
  test('queued mutations persist across full page reload and drain on reconnect', async ({ page, context }) => {
    await login(page);

    await context.setOffline(true);

    // Mark an entry read while offline.
    const firstEntry = page.locator('.entry-row, [data-testid="entry-row"]').first();
    await firstEntry.click();
    await page.waitForTimeout(500);

    // Verify queued.
    const hasQueue = await page.evaluate(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.some(k => JSON.parse(localStorage.getItem(k) ?? '[]').length > 0);
    });
    expect(hasQueue).toBe(true);

    // Reload while still offline.
    await page.reload({ waitUntil: 'domcontentloaded', timeout: 10000 }).catch(() => {});

    // Still queued after reload.
    const stillQueued = await page.evaluate(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.some(k => JSON.parse(localStorage.getItem(k) ?? '[]').length > 0);
    });
    expect(stillQueued).toBe(true);

    // Go back online.
    await context.setOffline(false);

    // Wait for drain.
    await page.waitForFunction(() => {
      const keys = Object.keys(localStorage).filter(k => k.startsWith('tap:queue:'));
      return keys.every(k => JSON.parse(localStorage.getItem(k) ?? '[]').length === 0);
    }, { timeout: 10000 });
  });
});
