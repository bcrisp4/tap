<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import { auth } from '../../lib/auth';
  import { api } from '../../lib/api';
  import ChangePasswordDialog from './dialogs/ChangePasswordDialog.svelte';

  let dialogOpen = $state(false);
  let busy = $state(false);
  let error = $state('');

  const email = $derived($auth.user?.username ?? '');

  async function signOutEverywhere() {
    busy = true; error = '';
    try {
      await api.revokeAllOtherSessions();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Sign-out failed.';
    } finally { busy = false; }
  }
</script>

<SetSection num="04" title="Account">
  <SetRow label="Email" desc="Used for sign-in and account recovery.">
    {#snippet control()}
      <span class="email">{email}</span>
    {/snippet}
  </SetRow>
  <SetRow label="Change password" desc="Re-authenticates and rotates your session token.">
    {#snippet control()}
      <Button onclick={() => dialogOpen = true}>Change password</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Sign out everywhere" desc="Revokes every other session except this one. You stay signed in on this device.">
    {#snippet control()}
      <Button variant="danger" onclick={signOutEverywhere} disabled={busy}>Sign out everywhere</Button>
    {/snippet}
  </SetRow>
  {#if error}<div role="alert" class="err">{error}</div>{/if}
</SetSection>

{#if dialogOpen}
  <ChangePasswordDialog onClose={() => dialogOpen = false} />
{/if}

<style>
  .email {
    font-family: var(--mono);
    font-size: 13px;
    color: var(--ink);
    letter-spacing: 0.01em;
  }
  .err {
    color: var(--ink); font-family: var(--mono); font-size: 11px;
    padding: 8px 0;
  }
</style>
