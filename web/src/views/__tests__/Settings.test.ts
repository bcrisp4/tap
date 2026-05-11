import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { userEvent } from '@testing-library/user-event';

vi.mock('../../lib/preferences.svelte', () => ({
  theme: { stored: 'system', resolved: 'light' },
  font: { value: 'serif' },
  density: { value: 'comfortable' },
  measure: { value: 'comfortable' },
}));
vi.mock('../../lib/auth', () => ({
  auth: {
    subscribe: (fn: (v: null) => void) => { fn(null); return () => {}; },
    bootstrap: vi.fn(),
  },
}));
vi.mock('../../lib/api', () => ({
  api: {
    listSessions: vi.fn().mockResolvedValue([]),
    listPasskeys: vi.fn().mockResolvedValue([]),
  },
}));

import Settings from '../Settings.svelte';

describe('Settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the Appearance section by default', () => {
    render(Settings);
    // The Segmented component uses aria-label on its radiogroup element
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeTruthy();
    expect(screen.getByRole('radiogroup', { name: 'Font' })).toBeTruthy();
    expect(screen.getByRole('radiogroup', { name: 'Density' })).toBeTruthy();
    expect(screen.getByRole('radiogroup', { name: 'Article width' })).toBeTruthy();
  });

  it('renders the Security panel when the Security tab is selected', async () => {
    const user = userEvent.setup();
    render(Settings);
    await user.click(screen.getByRole('button', { name: 'Security' }));
    expect(await screen.findByRole('heading', { name: 'Security Settings' })).toBeTruthy();
  });

  it('Appearance nav item has aria-current="true" when active', () => {
    render(Settings);
    const appearanceBtn = screen.getByRole('button', { name: 'Appearance' });
    expect(appearanceBtn.getAttribute('aria-current')).toBe('true');
  });

  it('density Segmented shows Compact / Comfortable / Cosy options', () => {
    render(Settings);
    const densityGroup = screen.getByRole('radiogroup', { name: 'Density' });
    expect(densityGroup.querySelector('[aria-label="Compact"]') || densityGroup.textContent).toContain('Compact');
    expect(densityGroup.textContent).toContain('Comfortable');
    expect(densityGroup.textContent).toContain('Cosy');
  });
});
