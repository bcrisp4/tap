<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import OtpInput from '../../../components/OtpInput.svelte';
  import { api } from '../../../lib/api';

  interface Props { onClose: () => void; onSuccess: (codes: string[]) => void; }
  let { onClose, onSuccess }: Props = $props();

  let code = $state('');
  let busy = $state(false);
  let error = $state('');

  async function submit() {
    busy = true; error = '';
    try {
      const res = await api.regenerateRecoveryCodes(code);
      onSuccess(res.recovery_codes);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Invalid code.';
    } finally { busy = false; }
  }
</script>

<Dialog open title="Regenerate recovery codes" {onClose}>
  {#snippet children()}
    <p class="p">Generating new codes invalidates the old set immediately. Enter your current authenticator code to continue.</p>
    <div class="label">6-digit code from your authenticator</div>
    <OtpInput value={code} onChange={(v) => code = v} />
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button onclick={onClose}>Cancel</Button>
    <Button variant="primary" onclick={submit} disabled={busy || code.length !== 6}>Generate new codes</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .label { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); margin-bottom: 8px; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
</style>
