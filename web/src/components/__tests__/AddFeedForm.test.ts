import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import AddFeedForm from '../AddFeedForm.svelte';

// Mock the subscriptions store so tests don't make real network calls.
vi.mock('../../lib/store', () => ({
  subscriptions: {
    subscribe: vi.fn((fn: (v: unknown[]) => void) => {
      fn([]);
      return () => {};
    }),
    add: vi.fn(),
    load: vi.fn(),
  },
}));

describe('AddFeedForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders a URL input and an Add button', () => {
    render(AddFeedForm);
    expect(screen.getByRole('textbox')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add/i })).toBeInTheDocument();
  });

  it('Add button is disabled when the URL input is empty', () => {
    render(AddFeedForm);
    const button = screen.getByRole('button', { name: /add/i });
    expect(button).toBeDisabled();
  });

  it('Add button becomes enabled once a non-blank URL is typed', async () => {
    render(AddFeedForm);
    const input = screen.getByRole('textbox');
    await fireEvent.input(input, { target: { value: 'https://example.com/feed' } });
    const button = screen.getByRole('button', { name: /add/i });
    expect(button).not.toBeDisabled();
  });

  it('does not call subscriptions.add when submitted with an empty URL', async () => {
    const { subscriptions } = await import('../../lib/store');
    render(AddFeedForm);
    const form = document.querySelector('form')!;
    await fireEvent.submit(form);
    expect(subscriptions.add).not.toHaveBeenCalled();
  });

  it('calls subscriptions.add with the trimmed URL on valid submission', async () => {
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add).mockResolvedValueOnce(undefined);

    render(AddFeedForm);
    const input = screen.getByRole('textbox');
    await fireEvent.input(input, { target: { value: '  https://example.com/feed  ' } });
    const form = document.querySelector('form')!;
    await fireEvent.submit(form);

    expect(subscriptions.add).toHaveBeenCalledWith('https://example.com/feed');
  });

  it('clears the URL input after a successful submission', async () => {
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add).mockResolvedValueOnce(undefined);

    render(AddFeedForm);
    const input = screen.getByRole('textbox');
    await fireEvent.input(input, { target: { value: 'https://example.com/feed' } });
    const form = document.querySelector('form')!;
    await fireEvent.submit(form);

    await waitFor(() => {
      expect((input as HTMLInputElement).value).toBe('');
    });
  });

  it('surfaces the server error message when subscriptions.add rejects', async () => {
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add).mockRejectedValueOnce(
      new Error('feed_url already subscribed'),
    );

    render(AddFeedForm);
    const input = screen.getByRole('textbox');
    await fireEvent.input(input, { target: { value: 'https://example.com/feed' } });
    const form = document.querySelector('form')!;
    await fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText('feed_url already subscribed')).toBeInTheDocument();
    });
  });

  it('clears the error when a subsequent submission succeeds', async () => {
    const { subscriptions } = await import('../../lib/store');
    vi.mocked(subscriptions.add)
      .mockRejectedValueOnce(new Error('feed_url already subscribed'))
      .mockResolvedValueOnce(undefined);

    render(AddFeedForm);
    const input = screen.getByRole('textbox');
    const form = document.querySelector('form')!;

    // First submission — error shown.
    await fireEvent.input(input, { target: { value: 'https://example.com/feed' } });
    await fireEvent.submit(form);
    await waitFor(() => {
      expect(screen.getByText('feed_url already subscribed')).toBeInTheDocument();
    });

    // Second submission — error should be cleared.
    await fireEvent.input(input, { target: { value: 'https://other.com/feed' } });
    await fireEvent.submit(form);
    await waitFor(() => {
      expect(screen.queryByText('feed_url already subscribed')).not.toBeInTheDocument();
    });
  });
});
