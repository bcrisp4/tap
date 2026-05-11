import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SysGrid from './SysGrid.svelte';
import type { StatusResponse } from '../lib/status';

const baseStatus: StatusResponse = {
  version: 'v1.0',
  uptime_seconds: 0,
  db: 'ok',
  polls_active: 0,
  polls_total: 0,
  last_poll_at: 1_700_000_000,
  recent_errors: [],
  metrics_ok: true,
  feeds_total: 24,
  feeds_ok: 22,
  feeds_with_errors: 2,
  offending_feeds: ['Phoronix', 'LWN'],
  entries_total: 14_820,
  entries_24h: 1_402,
};

describe('SysGrid', () => {
  it('renders four labelled cells', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: 1_700_000_060, now: 1_700_000_840 } });
    expect(screen.getByText('FEEDS')).toBeInTheDocument();
    expect(screen.getByText('ENTRIES')).toBeInTheDocument();
    expect(screen.getByText('ERRORS')).toBeInTheDocument();
    expect(screen.getByText('POLL')).toBeInTheDocument();
  });

  it('FEEDS cell shows total + "n ok" sub line', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-feeds-v')).toHaveTextContent('24');
    expect(screen.getByTestId('sys-feeds-sub')).toHaveTextContent('22 ok');
  });

  it('ENTRIES cell shows total + 24h sub line', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-entries-v')).toHaveTextContent('14,820');
    expect(screen.getByTestId('sys-entries-sub')).toHaveTextContent('1,402 24h');
  });

  it('ERRORS cell shows count + first offending feed', () => {
    render(SysGrid, { props: { status: baseStatus, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-errors-v')).toHaveTextContent('2');
    expect(screen.getByTestId('sys-errors-sub')).toHaveTextContent('Phoronix');
  });

  it('ERRORS cell renders "all clear" sub when no offending feeds', () => {
    render(SysGrid, { props: { status: { ...baseStatus, feeds_with_errors: 0, offending_feeds: [] }, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-errors-v')).toHaveTextContent('0');
    expect(screen.getByTestId('sys-errors-sub')).toHaveTextContent('all clear');
  });

  it('POLL cell shows relative last + next', () => {
    const now = 1_700_000_000 + 14 * 60;
    render(SysGrid, { props: { status: { ...baseStatus, last_poll_at: 1_700_000_000 }, nextPollAt: now + 60, now } });
    expect(screen.getByTestId('sys-poll-v')).toHaveTextContent('14m ago');
    expect(screen.getByTestId('sys-poll-sub')).toHaveTextContent('next 1m');
  });

  it('POLL cell handles missing timestamps', () => {
    render(SysGrid, { props: { status: { ...baseStatus, last_poll_at: null }, nextPollAt: null, now: 1_700_000_000 } });
    expect(screen.getByTestId('sys-poll-v')).toHaveTextContent('—');
    expect(screen.getByTestId('sys-poll-sub')).toHaveTextContent('—');
  });

  it('null status renders skeleton dashes in all cells', () => {
    render(SysGrid, { props: { status: null, nextPollAt: null, now: 0 } });
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(4);
  });

  it('metrics_ok=false renders dashes in metric cells even when status present', () => {
    render(SysGrid, { props: { status: { ...baseStatus, metrics_ok: false }, nextPollAt: null, now: 1_700_000_840 } });
    expect(screen.getByTestId('sys-feeds-v')).toHaveTextContent('—');
    expect(screen.getByTestId('sys-entries-v')).toHaveTextContent('—');
    expect(screen.getByTestId('sys-errors-v')).toHaveTextContent('—');
  });

  it('error prop renders error message in place of grid', () => {
    render(SysGrid, { props: { status: null, error: 'forbidden', nextPollAt: null, now: 0 } });
    expect(screen.getByRole('alert')).toHaveTextContent('forbidden');
  });
});
