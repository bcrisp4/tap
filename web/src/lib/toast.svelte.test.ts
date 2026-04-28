// Toast store unit tests. The store is a Svelte 5 rune wrapper around
// a single-message slot, so behavior is observable without mounting
// the DOM component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { toast } from './toast.svelte';

beforeEach(() => {
	toast.dismiss();
});

describe('toast store', () => {
	it('exposes the latest pushed message', () => {
		toast.push('hello', 'error');
		expect(toast.current?.message).toBe('hello');
		expect(toast.current?.kind).toBe('error');
	});

	it('auto-dismisses after 4s', () => {
		vi.useFakeTimers();
		toast.push('bye', 'info');
		expect(toast.current).not.toBeNull();
		vi.advanceTimersByTime(4001);
		expect(toast.current).toBeNull();
		vi.useRealTimers();
	});

	it('replaces an active toast when a new one is pushed', () => {
		toast.push('first', 'info');
		toast.push('second', 'error');
		expect(toast.current?.message).toBe('second');
	});
});
