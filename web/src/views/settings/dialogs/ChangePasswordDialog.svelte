<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import Field from '../../../components/Field.svelte';
  import { api } from '../../../lib/api';

  interface Props { onClose: () => void; }
  let { onClose }: Props = $props();

  let current = $state('');
  let next = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit() {
    error = '';
    if (next !== confirm) { error = "New passwords don't match."; return; }
    busy = true;
    try {
      await api.changePassword(current, next);
      onClose();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not change password.';
    } finally { busy = false; }
  }
</script>

<Dialog open title="Change password" {onClose}>
  {#snippet children()}
    <Field label="Current password" type="password" bind:value={current} />
    <Field label="New password" type="password" bind:value={next} />
    <Field label="Confirm new password" type="password" bind:value={confirm} />
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button variant="quiet" onclick={onClose}>Cancel</Button>
    <Button variant="primary" onclick={submit} disabled={busy || !current || !next || !confirm}>Change password</Button>
  {/snippet}
</Dialog>

<style>
  .warn {
    margin-top: 12px;
    padding: 10px 12px;
    border-left: 2px solid var(--accent);
    background: var(--accent-soft);
    color: var(--ink-2);
    font-family: var(--sans);
    font-size: 12px;
    line-height: 1.5;
    border-radius: 0 3px 3px 0;
  }
</style>
