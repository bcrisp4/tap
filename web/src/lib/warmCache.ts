const PROXY_URL_REGEX = /\/api\/v1\/proxy\/[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g;
const PER_ENTRY_CAP = 20;
const TOTAL_CAP = 200;
const CONCURRENCY = 4;

async function runWithConcurrency(urls: string[], concurrency: number): Promise<void> {
  const queue = [...urls];
  async function worker() {
    while (queue.length > 0) {
      const url = queue.shift()!;
      try {
        await fetch(url);
      } catch { /* opportunistic — errors are irrelevant */ }
    }
  }
  await Promise.all(Array.from({ length: concurrency }, worker));
}

// userId is accepted for interface consistency with callers (drain takes userId)
// and for future per-user cache-warming signals; the browser attaches the session
// cookie implicitly on same-origin GETs so no per-user auth is needed here.
export async function warmCache(_userId: number): Promise<void> {
  try {
    const res = await fetch('/api/v1/entries?limit=50&unread=1');
    if (!res.ok) return;
    const body = await res.json();
    const entries: Array<{ content: string }> = body.data ?? [];

    const seen = new Set<string>();
    const urls: string[] = [];

    for (const entry of entries) {
      if (urls.length >= TOTAL_CAP) break;
      const matches = (entry.content ?? '').matchAll(PROXY_URL_REGEX);
      let count = 0;
      for (const m of matches) {
        if (count >= PER_ENTRY_CAP) break;
        if (urls.length >= TOTAL_CAP) break;
        const url = m[0];
        if (!seen.has(url)) {
          seen.add(url);
          urls.push(url);
          count++;
        }
      }
    }

    await runWithConcurrency(urls, CONCURRENCY);
  } catch {
    // Swallow all errors — warm-cache must never surface to the user.
  }
}
