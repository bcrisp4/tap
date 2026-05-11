import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('qrcode', () => ({
  default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,xxx') },
}));

vi.mock('../../../../lib/auth', () => ({
  auth: { bootstrap: vi.fn().mockResolvedValue(undefined) },
  ERR_UNAUTHORIZED: 'unauthorized',
}));

vi.mock('../../../../lib/api', () => ({
  api: {
    beginTOTPEnrolment: vi.fn(),
    confirmTOTPEnrolment: vi.fn(),
  },
}));

const { default: EnrolTOTPDialog } = await import('../EnrolTOTPDialog.svelte');
const { api } = await import('../../../../lib/api');

describe('EnrolTOTPDialog', () => {
  beforeEach(() => vi.clearAllMocks());

  it('on mount fetches a fresh secret and renders the QR', async () => {
    vi.mocked(api.beginTOTPEnrolment).mockResolvedValue({ secret_uri: 'otpauth://x', secret: 'JBSW' } as never);
    const { findByAltText } = render(EnrolTOTPDialog, { props: { onClose: vi.fn(), onSuccess: vi.fn() } });
    expect(await findByAltText(/qr/i)).toBeInTheDocument();
  });

  it('Confirm calls confirmTOTPEnrolment and resolves onSuccess with recovery codes', async () => {
    vi.mocked(api.beginTOTPEnrolment).mockResolvedValue({ secret_uri: 'otpauth://x', secret: 'JBSW' } as never);
    vi.mocked(api.confirmTOTPEnrolment).mockResolvedValue({ recovery_codes: ['AAAA-BBBB-CCCC'] } as never);
    const onSuccess = vi.fn();
    const { findByRole, getByRole } = render(EnrolTOTPDialog, { props: { onClose: vi.fn(), onSuccess } });
    // Wait for the QR to load (implies beginTOTPEnrolment resolved).
    await findByRole('img', { name: /qr/i });
    // Simulate entering a 6-digit code — one digit per OTP cell.
    const digits = '123456'.split('');
    const otpCells = document.querySelectorAll<HTMLInputElement>('input[inputmode="numeric"]');
    for (let i = 0; i < 6; i++) {
      await fireEvent.input(otpCells[i], { target: { value: digits[i] } });
    }
    await fireEvent.click(getByRole('button', { name: /enable/i }));
    await waitFor(() => {
      expect(api.confirmTOTPEnrolment).toHaveBeenCalledWith('123456');
      expect(onSuccess).toHaveBeenCalledWith(['AAAA-BBBB-CCCC']);
    });
  });
});
