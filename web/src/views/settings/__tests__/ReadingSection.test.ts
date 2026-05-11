import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi } from 'vitest';

const readingPref = {
  markOnScroll: true,
  autoOpenNext: false,
  showSummaries: true,
  openLinksNewTab: true,
};

vi.mock('../../../lib/preferences.svelte', () => ({
  reading: {
    get markOnScroll() { return readingPref.markOnScroll; },
    set markOnScroll(v: boolean) { readingPref.markOnScroll = v; },
    get autoOpenNext() { return readingPref.autoOpenNext; },
    set autoOpenNext(v: boolean) { readingPref.autoOpenNext = v; },
    get showSummaries() { return readingPref.showSummaries; },
    set showSummaries(v: boolean) { readingPref.showSummaries = v; },
    get openLinksNewTab() { return readingPref.openLinksNewTab; },
    set openLinksNewTab(v: boolean) { readingPref.openLinksNewTab = v; },
  },
}));

const { default: ReadingSection } = await import('../ReadingSection.svelte');

describe('ReadingSection', () => {
  beforeEach(() => {
    readingPref.markOnScroll = true;
    readingPref.autoOpenNext = false;
    readingPref.showSummaries = true;
    readingPref.openLinksNewTab = true;
  });

  it('renders four toggles with the current values', () => {
    const { getByLabelText } = render(ReadingSection);
    expect(getByLabelText(/Mark read on scroll/i)).toBeChecked();
    expect(getByLabelText(/Auto-open next/i)).not.toBeChecked();
    expect(getByLabelText(/Show summaries/i)).toBeChecked();
    expect(getByLabelText(/Open links in new tab/i)).toBeChecked();
  });

  it('clicking a toggle flips the pref', async () => {
    readingPref.autoOpenNext = false;
    const { getByLabelText } = render(ReadingSection);
    await fireEvent.click(getByLabelText(/Auto-open next/i));
    expect(readingPref.autoOpenNext).toBe(true);
  });
});
