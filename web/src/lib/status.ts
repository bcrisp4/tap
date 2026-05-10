export type StatusResponse = {
  version: string;
  uptime_seconds: number;
  db: 'ok' | 'degraded';
  polls_active: number;
  polls_total: number;
  last_poll_at: number | null;
  recent_errors: Array<{
    time: string;
    level: 'warn' | 'error';
    event: string;
    attrs: Record<string, unknown>;
  }>;
};

export async function getStatus(): Promise<StatusResponse> {
  const resp = await fetch('/api/v1/status');
  if (!resp.ok) throw new Error(`status ${resp.status}`);
  return resp.json();
}
