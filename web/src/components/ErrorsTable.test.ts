import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ErrorsTable from './ErrorsTable.svelte';
import type { StatusEvent } from '../lib/status';

const events: StatusEvent[] = [
  { time: '2026-05-11T10:42:14Z', level: 'error', event: 'phoronix.com — 502', attrs: {} },
  { time: '2026-05-11T10:38:41Z', level: 'warn',  event: 'poll worker 3 slow', attrs: {} },
];

describe('ErrorsTable', () => {
  it('renders one row per event with level tag and message', () => {
    render(ErrorsTable, { props: { events } });
    expect(screen.getByText('phoronix.com — 502')).toBeInTheDocument();
    expect(screen.getByText('poll worker 3 slow')).toBeInTheDocument();
    expect(screen.getByText('error')).toBeInTheDocument();
    expect(screen.getByText('warn')).toBeInTheDocument();
  });

  it('formats timestamp as HH:MM:SS', () => {
    render(ErrorsTable, { props: { events: [events[0]] } });
    expect(screen.getByTestId('err-time')).toHaveTextContent(/\d{2}:\d{2}:\d{2}/);
  });

  it('renders empty state when no events', () => {
    render(ErrorsTable, { props: { events: [] } });
    expect(screen.getByText(/no recent events/i)).toBeInTheDocument();
  });

  it('applies level-error class to error-level rows', () => {
    const { container } = render(ErrorsTable, { props: { events: [events[0]] } });
    expect(container.querySelector('.err.level-error')).toBeTruthy();
  });
});
