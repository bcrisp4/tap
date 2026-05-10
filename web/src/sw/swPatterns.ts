export const PROXY_PATTERN = /^\/api\/v1\/proxy\//;
export const ENTRIES_PATTERN = /^\/api\/v1\/(entries|subscriptions)/;
export const EXCLUDED_PATTERNS = [
  /^\/api\/v1\/search/,
  /^\/api\/v1\/opml/,
  /^\/api\/v1\/sessions/,
];

export function matchesProxy(pathname: string) { return PROXY_PATTERN.test(pathname); }
export function matchesEntries(pathname: string) { return ENTRIES_PATTERN.test(pathname); }
export function matchesExcluded(pathname: string) { return EXCLUDED_PATTERNS.some(p => p.test(pathname)); }
export function apiCacheName(prefix: string, userId: number) { return `${prefix}-${userId}`; }
