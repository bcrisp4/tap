import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mutable state so each test can observe writes.
const themePref = { stored: 'light' as string };
const fontPref = { value: 'serif' as string };
const densityPref = { value: 'comfortable' as string };
const measurePref = { value: 'comfortable' as string };

vi.mock('../../../lib/preferences.svelte', () => ({
  theme: {
    get stored() { return themePref.stored; },
    set stored(v: string) { themePref.stored = v; },
  },
  font: {
    get value() { return fontPref.value; },
    set value(v: string) { fontPref.value = v; },
  },
  density: {
    get value() { return densityPref.value; },
    set value(v: string) { densityPref.value = v; },
  },
  measure: {
    get value() { return measurePref.value; },
    set value(v: string) { measurePref.value = v; },
  },
}));

const { default: AppearanceSection } = await import('../AppearanceSection.svelte');

describe('AppearanceSection', () => {
  beforeEach(() => {
    themePref.stored = 'light';
    fontPref.value = 'serif';
    densityPref.value = 'comfortable';
    measurePref.value = 'comfortable';
  });

  it('shows the current theme as the active segmented button', () => {
    themePref.stored = 'sepia';
    const { getByRole } = render(AppearanceSection);
    expect(getByRole('radio', { name: /Sepia/i })).toHaveAttribute('aria-checked', 'true');
  });

  it('clicking a theme button updates the theme store', async () => {
    const { getByRole } = render(AppearanceSection);
    await fireEvent.click(getByRole('radio', { name: /Dark/i }));
    expect(themePref.stored).toBe('dark');
  });

  it('clicking a font button updates the font store', async () => {
    const { getByRole } = render(AppearanceSection);
    await fireEvent.click(getByRole('radio', { name: /Sans/i }));
    expect(fontPref.value).toBe('sans');
  });

  it('clicking a density button updates the density store', async () => {
    const { getByRole } = render(AppearanceSection);
    await fireEvent.click(getByRole('radio', { name: /Compact/i }));
    expect(densityPref.value).toBe('compact');
  });
});
