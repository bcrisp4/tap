<script lang="ts">
  import { onMount } from 'svelte';
  import QRCode from 'qrcode';
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import OtpInput from '../../../components/OtpInput.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';

  interface Props {
    onClose: () => void;
    onSuccess: (codes: string[]) => void;
  }
  let { onClose, onSuccess }: Props = $props();

  let secret = $state('');
  let secretUri = $state('');
  let qrDataUrl = $state('');
  let code = $state('');
  let busy = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      const res = await api.beginTOTPEnrolment();
      secret = res.secret;
      secretUri = res.secret_uri;
      qrDataUrl = await QRCode.toDataURL(res.secret_uri, { margin: 1, width: 140 });
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not start enrolment.';
    }
  });

  async function confirm() {
    error = '';
    busy = true;
    try {
      const res = await api.confirmTOTPEnrolment(code);
      await auth.bootstrap();
      onSuccess(res.recovery_codes);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Invalid code.';
    } finally { busy = false; }
  }

  async function copySecret() {
    try {
      await navigator.clipboard.writeText(secret.replace(/\s+/g, ''));
    } catch { /* swallow */ }
  }
</script>

<Dialog open title="Set up authenticator" {onClose}>
  {#snippet children()}
    <div class="setup">
      {#if qrDataUrl}
        <img class="qr" src={qrDataUrl} alt="QR code for authenticator app" width="140" height="140" />
      {:else}
        <div class="qr placeholder" aria-hidden="true"></div>
      {/if}
      <div class="right">
        <div class="step">Step 1 · Scan or enter secret</div>
        <p class="p">Add Tap to your authenticator app — 1Password, Authy, or any TOTP-compatible client.</p>
        <div class="secret">
          <span class="key">{secret}</span>
          <button type="button" class="copy" onclick={copySecret}>Copy</button>
        </div>
      </div>
    </div>
    <div class="step2">
      <div class="step">Step 2 · Confirm code from your app</div>
      <OtpInput value={code} onChange={(v) => code = v} />
    </div>
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">Algorithm SHA-1 · 30s · 6 digits</div>
    <Button onclick={onClose}>Cancel</Button>
    <Button variant="primary" onclick={confirm} disabled={busy || code.length !== 6}>Enable</Button>
  {/snippet}
</Dialog>

<style>
  .setup { display: flex; gap: 18px; align-items: flex-start; }
  .qr { width: 140px; height: 140px; border: 1px solid var(--rule); border-radius: 4px; flex-shrink: 0; }
  .qr.placeholder { background: var(--bg-soft); }
  .right { flex: 1; min-width: 0; }
  .step { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); margin-bottom: 6px; }
  .p { font-family: var(--serif); font-size: 14px; line-height: 1.55; color: var(--ink-2); margin: 0 0 10px; }
  .secret { display: flex; align-items: center; gap: 12px; padding: 12px 14px; background: var(--bg-soft); border: 1px solid var(--rule); border-radius: 4px; }
  .key { font-family: var(--mono); font-size: 14px; letter-spacing: 0.12em; color: var(--ink); font-weight: 500; }
  .copy { margin-left: auto; font-family: var(--mono); font-size: 10px; letter-spacing: 0.08em; text-transform: uppercase; color: var(--ink-2); border: 1px solid var(--rule); background: var(--bg); padding: 4px 8px; border-radius: 3px; cursor: pointer; }
  .copy:hover { color: var(--ink); border-color: var(--ink-4); }
  .step2 { margin-top: 22px; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
  .foot-l { margin-right: auto; font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); align-self: center; }
</style>
