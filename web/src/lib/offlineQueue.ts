type QueuedMutation = {
  id: string;
  method: string;
  path: string;
  body: unknown;
  csrfToken: string;
  enqueuedAt: number;
};

type EnqueueInput = Omit<QueuedMutation, 'id' | 'enqueuedAt'>;

const BASE = '/api/v1';

function storageKey(userId: number) { return `tap:queue:${userId}`; }
function pendingKey(userId: number) { return `tap:queue-pending:${userId}`; }

function read(userId: number): QueuedMutation[] {
  try {
    return JSON.parse(localStorage.getItem(storageKey(userId)) ?? '[]');
  } catch {
    return [];
  }
}

function write(userId: number, items: QueuedMutation[]) {
  localStorage.setItem(storageKey(userId), JSON.stringify(items));
}

let draining = false;

async function fetchCSRF(): Promise<string | null> {
  try {
    const res = await fetch(BASE + '/sessions/current', { headers: { 'Content-Type': 'application/json' } });
    if (!res.ok) return null;
    const body = await res.json();
    return body.csrf_token ?? null;
  } catch {
    return null;
  }
}

async function sendMutation(m: QueuedMutation): Promise<Response> {
  return fetch(BASE + m.path, {
    method: m.method,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': m.csrfToken,
    },
    body: JSON.stringify(m.body),
  });
}

export const offlineQueue = {
  enqueue(userId: number, input: EnqueueInput): void {
    const items = read(userId);
    items.push({ ...input, id: crypto.randomUUID(), enqueuedAt: Date.now() });
    write(userId, items);
  },

  async drain(userId: number): Promise<void> {
    if (draining) return;
    draining = true;
    try {
      while (true) {
        const items = read(userId);
        if (items.length === 0) break;
        const [head, ...rest] = items;
        let res: Response;
        try {
          res = await sendMutation(head);
        } catch {
          break;
        }

        if (res.ok) {
          write(userId, rest);
          continue;
        }

        let code: string | undefined;
        try { code = (await res.json())?.error?.code; } catch { /* swallow */ }

        if (res.status === 401) {
          localStorage.setItem(pendingKey(userId), '1');
          break;
        }

        if (res.status === 403 && code === 'csrf_invalid') {
          const freshToken = await fetchCSRF();
          if (freshToken) {
            head.csrfToken = freshToken;
            let retry: Response;
            try {
              retry = await sendMutation(head);
            } catch {
              break;
            }
            if (retry.ok) {
              write(userId, rest);
              continue;
            }
          }
          // Double-403 or no fresh token — discard.
          console.warn('offlineQueue: discarding unrecoverable mutation', head.path);
          write(userId, rest);
          continue;
        }

        // Other 4xx — unrecoverable, discard.
        console.warn('offlineQueue: discarding unrecoverable mutation', res.status, head.path);
        write(userId, rest);
      }
    } finally {
      if (read(userId).length === 0) {
        localStorage.removeItem(pendingKey(userId));
      }
      draining = false;
    }
  },

  clearForUser(userId: number): void {
    localStorage.removeItem(storageKey(userId));
    localStorage.removeItem(pendingKey(userId));
  },
};
