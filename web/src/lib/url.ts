// feed_url is always an absolute http(s) URL — do not prepend "https://".
export function originOf(absUrl: string): string {
  try { return new URL(absUrl).origin; } catch { return absUrl; }
}

// Strip scheme for display (jvns.ca/atom.xml, not https://jvns.ca/atom.xml).
export function displayUrl(absUrl: string): string {
  try {
    const u = new URL(absUrl);
    return u.host + u.pathname.replace(/\/$/, '');
  } catch {
    return absUrl;
  }
}

export function formatAgo(seconds: number): string {
  if (!Number.isFinite(seconds)) return '—';
  if (seconds < 60)    return `${Math.floor(seconds)}s`;
  if (seconds < 3600)  return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
  return `${Math.floor(seconds / 86400)}d`;
}
