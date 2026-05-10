const PROXY_URL_REGEX = /\/api\/v1\/proxy\/[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g;
const PER_ENTRY_CAP = 20;
const TOTAL_CAP = 200;
const CONCURRENCY = 4;
// Number of entry details to fetch for proxy URL extraction.
const DETAIL_FETCH_LIMIT = 20;

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
    // The list endpoint returns items without content; fetch list to get IDs,
    // then fetch detail endpoints (which include content) to extract proxy URLs.
    const listRes = await fetch('/api/v1/entries?limit=50&unread=1');
    if (!listRes.ok) return;
    const listBody = await listRes.json();
    const ids: number[] = (listBody.data ?? []).slice(0, DETAIL_FETCH_LIMIT).map((e: { id: number }) => e.id);
    if (ids.length === 0) return;

    const details = await Promise.all(
      ids.map(async (id) => {
        try {
          const r = await fetch(`/api/v1/entries/${id}`);
          if (!r.ok) return null;
          return r.json() as Promise<{ content?: string }>;
        } catch {
          return null;
        }
      })
    );

    const seen = new Set<string>();
    const proxyUrls: string[] = [];

    for (const detail of details) {
      if (!detail || proxyUrls.length >= TOTAL_CAP) break;
      const matches = (detail.content ?? '').matchAll(PROXY_URL_REGEX);
      let count = 0;
      for (const m of matches) {
        if (count >= PER_ENTRY_CAP) break;
        if (proxyUrls.length >= TOTAL_CAP) break;
        const url = m[0];
        if (!seen.has(url)) {
          seen.add(url);
          proxyUrls.push(url);
          count++;
        }
      }
    }

    await runWithConcurrency(proxyUrls, CONCURRENCY);
  } catch {
    // Swallow all errors — warm-cache must never surface to the user.
  }
}
