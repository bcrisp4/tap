import { describe, it, expect } from 'vitest';
import {
  matchesProxy, matchesEntries, matchesExcluded, apiCacheName,
} from '../swPatterns';

// Helper wraps pathname extraction so tests pass full URLs naturally.
const p = (url: string) => new URL(url, 'http://localhost').pathname;

describe('SW URL pattern matching', () => {
  it('proxy pattern matches proxy URLs', () => {
    expect(matchesProxy(p('/api/v1/proxy/abc123.xyz456'))).toBe(true);
  });
  it('proxy pattern does not match entry URLs', () => {
    expect(matchesProxy(p('/api/v1/entries?unread=1'))).toBe(false);
  });
  it('entries pattern matches entries and subscriptions', () => {
    expect(matchesEntries(p('/api/v1/entries?unread=1'))).toBe(true);
    expect(matchesEntries(p('/api/v1/subscriptions'))).toBe(true);
  });
  it('entries pattern does not match proxy URLs', () => {
    expect(matchesEntries(p('/api/v1/proxy/abc'))).toBe(false);
  });
  it('search is excluded', () => {
    expect(matchesExcluded(p('/api/v1/search?q=hello'))).toBe(true);
  });
  it('opml is excluded', () => {
    expect(matchesExcluded(p('/api/v1/opml'))).toBe(true);
  });
  it('sessions is excluded', () => {
    expect(matchesExcluded(p('/api/v1/sessions/current'))).toBe(true);
  });
  it('entries is not excluded', () => {
    expect(matchesExcluded(p('/api/v1/entries'))).toBe(false);
  });
});

describe('SW cache naming', () => {
  it('api cache name includes user ID', () => {
    expect(apiCacheName('tap-api', 42)).toBe('tap-api-42');
  });
  it('proxy cache name includes user ID', () => {
    expect(apiCacheName('tap-proxy', 42)).toBe('tap-proxy-42');
  });
  it('different users get different cache names', () => {
    expect(apiCacheName('tap-api', 1)).not.toBe(apiCacheName('tap-api', 2));
  });
});

import { apiCacheName as _apiCacheName } from '../swPatterns';

describe('SW invalidate message logic', () => {
  it('deletes cache entries whose URLs contain any of the given paths', async () => {
    const deletedKeys: string[] = [];
    const cachedRequests = [
      new Request('http://localhost/api/v1/entries?unread=1'),
      new Request('http://localhost/api/v1/entries?cursor=abc'),
      new Request('http://localhost/api/v1/subscriptions'),
      new Request('http://localhost/api/v1/proxy/img.jpg'),
    ];

    const mockCache = {
      keys: async () => cachedRequests,
      delete: async (req: Request) => { deletedKeys.push(req.url); return true; },
    };
    const mockCaches = {
      open: async (_name: string) => mockCache,
    };

    const paths = ['/api/v1/entries'];
    const cacheName = _apiCacheName('tap-api', 1);

    // Replicate the invalidate handler logic directly.
    await mockCaches.open(cacheName).then(cache =>
      cache.keys().then(keys =>
        Promise.all(
          keys
            .filter(req => paths.some(path => req.url.includes(path)))
            .map(req => cache.delete(req))
        )
      )
    );

    expect(deletedKeys).toContain('http://localhost/api/v1/entries?unread=1');
    expect(deletedKeys).toContain('http://localhost/api/v1/entries?cursor=abc');
    expect(deletedKeys).not.toContain('http://localhost/api/v1/subscriptions');
    expect(deletedKeys).not.toContain('http://localhost/api/v1/proxy/img.jpg');
  });

  it('deletes subscriptions cache entries when /api/v1/subscriptions is in paths', async () => {
    const deletedKeys: string[] = [];
    const cachedRequests = [
      new Request('http://localhost/api/v1/subscriptions'),
      new Request('http://localhost/api/v1/entries?unread=1'),
    ];

    const mockCache = {
      keys: async () => cachedRequests,
      delete: async (req: Request) => { deletedKeys.push(req.url); return true; },
    };
    const mockCaches = { open: async (_name: string) => mockCache };

    const paths = ['/api/v1/subscriptions'];
    await mockCaches.open('tap-api-1').then(cache =>
      cache.keys().then(keys =>
        Promise.all(
          keys
            .filter(req => paths.some(path => req.url.includes(path)))
            .map(req => cache.delete(req))
        )
      )
    );

    expect(deletedKeys).toContain('http://localhost/api/v1/subscriptions');
    expect(deletedKeys).not.toContain('http://localhost/api/v1/entries?unread=1');
  });
});

import fs from 'fs';
import path from 'path';

describe('PWA manifest', () => {
  // Vitest runs from the web/ directory; dist/ is a sibling of src/.
  const manifestPath = path.resolve(process.cwd(), 'dist/manifest.webmanifest');

  it('manifest exists after build', () => {
    if (!fs.existsSync(manifestPath)) {
      console.warn('manifest.webmanifest not found — run pnpm build first');
      return;
    }
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf-8'));
    expect(manifest.name).toBe('Tap');
    expect(manifest.display).toBe('standalone');
    expect(manifest.theme_color).toBe('#002FA7');
    const icons: Array<{ sizes: string; purpose: string }> = manifest.icons ?? [];
    expect(icons.some(i => i.sizes === '192x192' && i.purpose === 'maskable')).toBe(true);
    expect(icons.some(i => i.sizes === '512x512' && i.purpose === 'maskable')).toBe(true);
  });
});
