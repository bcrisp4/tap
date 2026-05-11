<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import { auth } from '../../lib/auth';
  import EnrolTOTPDialog from './dialogs/EnrolTOTPDialog.svelte';
  import DisableTOTPDialog from './dialogs/DisableTOTPDialog.svelte';
  import RegenerateCodesDialog from './dialogs/RegenerateCodesDialog.svelte';
  import ViewRecoveryCodesDialog from './dialogs/ViewRecoveryCodesDialog.svelte';

  type Open = null | 'enrol' | 'disable' | 'regen' | 'view';
  let open = $state<Open>(null);
  let viewCodes = $state<string[]>([]);
  let regenerated = $state(false);

  const hasTOTP = $derived($auth.user?.has_totp ?? false);
</script>

<SetSection num="05" title="Security · Two-factor">
  <SetRow
    label="Authenticator app (TOTP)"
    desc="Pair Tap with an authenticator app. Required on new sign-ins once enabled."
    block
  >
    {#snippet children()}
      <div class="status" class:on={hasTOTP}>
        <span class="dot" aria-hidden="true"></span>
        {#if hasTOTP}
          <span class="strong">Enabled</span>
          <span class="sep" aria-hidden="true"></span>
          <span>SHA-1 · 30s · 6 digits</span>
        {:else}
          <span class="strong">Not enrolled</span>
          <span class="sep" aria-hidden="true"></span>
          <span>your account relies on password only</span>
        {/if}
      </div>
      <div class="actions">
        {#if hasTOTP}
          <Button variant="accent" onclick={() => { regenerated = true; open = 'regen'; }}>Regenerate recovery codes</Button>
          <Button variant="danger" onclick={() => open = 'disable'}>Disable TOTP</Button>
        {:else}
          <Button variant="primary" onclick={() => open = 'enrol'}>Set up authenticator</Button>
        {/if}
      </div>
    {/snippet}
  </SetRow>
</SetSection>

{#if open === 'enrol'}
  <EnrolTOTPDialog
    onClose={() => open = null}
    onSuccess={(codes) => { viewCodes = codes; regenerated = false; open = 'view'; }}
  />
{:else if open === 'disable'}
  <DisableTOTPDialog onClose={() => open = null} onSuccess={() => open = null} />
{:else if open === 'regen'}
  <RegenerateCodesDialog
    onClose={() => open = null}
    onSuccess={(codes) => { viewCodes = codes; open = 'view'; }}
  />
{:else if open === 'view'}
  <ViewRecoveryCodesDialog codes={viewCodes} {regenerated} onClose={() => { open = null; viewCodes = []; }} />
{/if}

<style>
  .status { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border: 1px solid var(--rule); background: var(--bg-soft); border-radius: 4px; font-family: var(--mono); font-size: 11px; color: var(--ink-2); letter-spacing: 0.02em; }
  .status .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ink-4); }
  .status.on .dot { background: var(--accent); }
  .status .sep { width: 3px; height: 3px; border-radius: 50%; background: var(--ink-4); }
  .strong { color: var(--ink); font-weight: 500; }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
</style>
