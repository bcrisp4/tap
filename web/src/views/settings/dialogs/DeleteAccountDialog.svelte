<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import Field from '../../../components/Field.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';
  import { navigate } from '../../../lib/router';

  interface Props { onClose: () => void; }
  let { onClose }: Props = $props();

  let password = $state('');
  let busy = $state(false);
  let error = $state('');

  async function submit() {
    error = ''; busy = true;
    try {
      await api.deleteAccount(password);
      await auth.logout();
      navigate('/sign-in');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not delete account.';
    } finally { busy = false; }
  }
</script>

<Dialog open title="Delete account?" {onClose}>
  {#snippet children()}
    <div class="dialog-warn">
      This cannot be undone. All your feeds, saved articles, sessions, passkeys, and read history will be permanently removed.
    </div>
    <Field label="Confirm with your account password" type="password" bind:value={password} />
    {#if error}<div role="alert" class="error">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">irreversible</div>
    <Button onclick={onClose}>Cancel</Button>
    <Button variant="danger" onclick={submit} disabled={busy || !password}>Delete my account</Button>
  {/snippet}
</Dialog>

<style>
  .dialog-warn { margin-bottom: 14px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
  .error { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
  .foot-l { margin-right: auto; font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); align-self: center; }
</style>
