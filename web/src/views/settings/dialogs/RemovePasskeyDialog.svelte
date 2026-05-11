<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';

  interface Props { passkeyId: number; passkeyLabel: string; onClose: () => void; onRemoved: () => void; }
  let { passkeyId, passkeyLabel, onClose, onRemoved }: Props = $props();

  let busy = $state(false);
  let error = $state('');

  async function submit() {
    error = ''; busy = true;
    try {
      await api.deletePasskey(passkeyId);
      await auth.bootstrap();
      onRemoved();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not remove passkey.';
    } finally { busy = false; }
  }
</script>

<Dialog open title="Remove passkey?" {onClose}>
  {#snippet children()}
    <p class="p">Removing <b>{passkeyLabel}</b> means it can no longer sign in to Tap. You can add it back any time.</p>
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button onclick={onClose}>Cancel</Button>
    <Button variant="danger" onclick={submit} disabled={busy}>Remove passkey</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
</style>
