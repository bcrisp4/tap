export type StatusEvent = {
  time: string;
  level: 'warn' | 'error';
  event: string;
  attrs: Record<string, unknown>;
};

export type StatusResponse = {
  version: string;
  uptime_seconds: number;
  db: 'ok' | 'degraded';
  polls_active: number;
  polls_total: number;
  last_poll_at: number | null;
  recent_errors: StatusEvent[];
  feeds_total: number;
  feeds_ok: number;
  feeds_with_errors: number;
  offending_feeds: string[];
  entries_total: number;
  entries_24h: number;
};

export async function getStatus(): Promise<StatusResponse> {
  const resp = await fetch('/api/v1/status');
  if (!resp.ok) throw new Error(`status ${resp.status}`);
  return resp.json();
}
