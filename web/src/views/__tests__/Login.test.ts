import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import Login from '../Login.svelte';
import { auth } from '../../lib/auth';

describe('Login (rewritten)', () => {
  beforeEach(() => {
    vi.spyOn(auth, 'login').mockReset();
    vi.spyOn(auth, 'loginWithTOTP').mockReset();
  });

  it('renders password mode by default', () => {
    const { getByText, getByLabelText } = render(Login);
    expect(getByText('Sign in')).toBeTruthy();
    expect(getByLabelText(/username/i)).toBeTruthy();
    expect(getByLabelText(/password/i)).toBeTruthy();
  });

  it('shows passkey button when credentials API is available', () => {
    Object.defineProperty(window.navigator, 'credentials', {
      configurable: true,
      value: { get: vi.fn() },
    });
    const { getByText } = render(Login);
    expect(getByText(/use a passkey/i)).toBeTruthy();
  });

  it('transitions to OTP mode when login returns totp_required', async () => {
    vi.spyOn(auth, 'login').mockResolvedValueOnce({
      totp_required: true, pending_token: 'pt',
    } as unknown as ReturnType<typeof auth.login> extends Promise<infer R> ? R : never);
    const { getByText, getByLabelText, findByText } = render(Login);
    await fireEvent.input(getByLabelText(/username/i), { target: { value: 'ada@x' } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByText(/continue/i));
    expect(await findByText(/verification code/i)).toBeTruthy();
  });

  it('error chip appears when auth.login throws', async () => {
    vi.spyOn(auth, 'login').mockRejectedValueOnce(new Error('Invalid'));
    const { getByText, getByLabelText, findByRole } = render(Login);
    await fireEvent.input(getByLabelText(/username/i), { target: { value: 'ada@x' } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByText(/continue/i));
    expect(await findByRole('alert')).toBeTruthy();
  });

  it('OTP mode can toggle to recovery code', async () => {
    vi.spyOn(auth, 'login').mockResolvedValueOnce({
      totp_required: true, pending_token: 'pt',
    } as unknown as ReturnType<typeof auth.login> extends Promise<infer R> ? R : never);
    const { getByText, getByLabelText } = render(Login);
    await fireEvent.input(getByLabelText(/username/i), { target: { value: 'ada@x' } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByText(/continue/i));
    await fireEvent.click(getByText(/use a recovery code/i));
    expect(getByLabelText(/recovery code/i)).toBeTruthy();
  });
});
