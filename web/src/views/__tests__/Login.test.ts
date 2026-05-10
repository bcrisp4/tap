import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/svelte';
import Login from '../Login.svelte';

const loginMock = vi.fn();
vi.mock('../../lib/auth', () => ({
  auth: {
    login: (...args: unknown[]) => loginMock(...args),
    subscribe: () => () => {},
  },
}));

beforeEach(() => loginMock.mockReset());
afterEach(() => {/* nothing */});

describe('Login.svelte', () => {
  it('renders username + password inputs and a submit button', () => {
    render(Login);
    expect(screen.getByLabelText(/username/i)).toBeTruthy();
    expect(screen.getByLabelText(/password/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeTruthy();
  });

  it('calls auth.login with the entered values on submit', async () => {
    loginMock.mockResolvedValueOnce(undefined);
    render(Login);

    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'ben' } });
    await fireEvent.input(screen.getByLabelText(/password/i), { target: { value: 'pw12345678' } });
    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    expect(loginMock).toHaveBeenCalledWith('ben', 'pw12345678');
  });

  it('shows an error message when login rejects', async () => {
    loginMock.mockRejectedValueOnce(new Error('unauthorized'));
    render(Login);

    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'ben' } });
    await fireEvent.input(screen.getByLabelText(/password/i), { target: { value: 'wrong' } });
    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    // Wait for the promise rejection + reactive update.
    await screen.findByText(/invalid username or password/i);
  });
});
