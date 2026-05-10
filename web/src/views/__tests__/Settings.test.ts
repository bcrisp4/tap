import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { userEvent } from '@testing-library/user-event';

vi.mock('../../components/Sidebar.svelte', () => ({ default: vi.fn() }));
vi.mock('../../lib/preferences.svelte', () => ({
  theme: { stored: 'system', resolved: 'light' },
  font: { value: 'serif' },
  density: { value: 'default' },
}));

import Settings from '../Settings.svelte';

describe('Settings', () => {
  it('renders the Appearance section by default', () => {
    render(Settings);
    // Both nav button and heading say "Appearance" — check for the select controls
    expect(screen.getByLabelText('Theme')).toBeTruthy();
    expect(screen.getByLabelText('Reading font')).toBeTruthy();
    expect(screen.getByLabelText('Density')).toBeTruthy();
  });

  it('switches to Security section when Security nav item is clicked', async () => {
    const user = userEvent.setup();
    render(Settings);
    await user.click(screen.getByRole('button', { name: 'Security' }));
    expect(screen.getByText(/Security settings/)).toBeTruthy();
    expect(screen.queryByLabelText('Theme')).toBeNull();
  });

  it('Appearance nav item has aria-current="true" when active', () => {
    render(Settings);
    const appearanceBtn = screen.getByRole('button', { name: 'Appearance' });
    expect(appearanceBtn.getAttribute('aria-current')).toBe('true');
  });
});
