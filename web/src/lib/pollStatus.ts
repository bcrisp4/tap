import { writable, type Readable } from 'svelte/store';

type Status = { active: number | null };

const internal = writable<Status>({ active: null });

let timer: ReturnType<typeof setTimeout> | undefined;
let running = false;

async function tick() {
  try {
    const r = await fetch('/healthz');
    if (r.ok) {
      const body = await r.json() as { polls_active?: number };
      internal.set({ active: body.polls_active ?? 0 });
    }
  } catch { /* leave value alone */ }
  if (running) {
    timer = setTimeout(tick, 30000);
  }
}

export function startPollStatus() {
  if (running) return;
  running = true;
  void tick();
}

export function stopPollStatus() {
  running = false;
  if (timer) clearTimeout(timer);
  timer = undefined;
}

export const pollStatus: Readable<Status> = { subscribe: internal.subscribe };
