<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '../../lib/auth';
  import { api } from '../../lib/api';
  import type { Session, Passkey, TOTPEnrolmentBegin } from '../../lib/types';
  import SystemStatus from '../../components/SystemStatus.svelte';

  let authState = $derived(get(auth));

  let sessions = $state<Session[]>([]);
  let passkeys = $state<Passkey[]>([]);
  let error = $state('');
  let busy = $state(false);

  // TOTP state
  let totpEnrolment = $state<TOTPEnrolmentBegin | null>(null);
  let totpConfirmCode = $state('');
  let recoveryCodes = $state<string[]>([]);
  let showRecoveryCodes = $state(false);
  let totpDisableCode = $state('');
  let totpRegenCode = $state('');
  let showDisableTOTP = $state(false);
  let showRegenerateCodes = $state(false);
  let newRecoveryCodes = $state<string[]>([]);

  onMount(async () => {
    await loadSessions();
    await loadPasskeys();
  });

  async function loadSessions() {
    try {
      sessions = await api.listSessions();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load sessions';
    }
  }

  async function loadPasskeys() {
    try {
      passkeys = await api.listPasskeys();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load passkeys';
    }
  }

  async function revokeSession(id: number) {
    busy = true;
    error = '';
    try {
      await api.revokeSession(id);
      sessions = sessions.filter(s => s.id !== id);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to revoke session';
    } finally {
      busy = false;
    }
  }

  async function revokeAllOthers() {
    busy = true;
    error = '';
    try {
      await api.revokeAllOtherSessions();
      sessions = sessions.filter(s => s.current);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to revoke sessions';
    } finally {
      busy = false;
    }
  }

  async function beginTOTPEnrolment() {
    busy = true;
    error = '';
    try {
      totpEnrolment = await api.beginTOTPEnrolment();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to begin TOTP enrolment';
    } finally {
      busy = false;
    }
  }

  async function confirmTOTPEnrolment() {
    busy = true;
    error = '';
    try {
      const result = await api.confirmTOTPEnrolment(totpConfirmCode);
      recoveryCodes = result.recovery_codes;
      showRecoveryCodes = true;
      totpEnrolment = null;
      totpConfirmCode = '';
      await auth.bootstrap();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Invalid TOTP code';
    } finally {
      busy = false;
    }
  }

  async function disableTOTP() {
    busy = true;
    error = '';
    try {
      await api.disableTOTP({ code: totpDisableCode });
      totpDisableCode = '';
      showDisableTOTP = false;
      await auth.bootstrap();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to disable TOTP';
    } finally {
      busy = false;
    }
  }

  async function regenerateCodes() {
    busy = true;
    error = '';
    try {
      const result = await api.regenerateRecoveryCodes(totpRegenCode);
      newRecoveryCodes = result.recovery_codes;
      totpRegenCode = '';
      showRegenerateCodes = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to regenerate codes';
    } finally {
      busy = false;
    }
  }

  async function addPasskey() {
    busy = true;
    error = '';
    try {
      const creationOptions = await api.beginPasskeyRegistration();
      const opts = creationOptions as { response: { user: { id: string }; challenge: string; rp: { id: string; name: string }; pubKeyCredParams: Array<{type: string; alg: number}> } };

      function b64urlToBytes(b64: string): Uint8Array {
        const pad = b64.length % 4 === 0 ? '' : '='.repeat(4 - (b64.length % 4));
        const b64std = (b64 + pad).replace(/-/g, '+').replace(/_/g, '/');
        return Uint8Array.from(atob(b64std), c => c.charCodeAt(0));
      }

      function bytesToB64url(buf: ArrayBuffer): string {
        return btoa(String.fromCharCode(...new Uint8Array(buf)))
          .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
      }

      const resp = opts.response;
      const credential = await navigator.credentials.create({
        publicKey: {
          rp: resp.rp as PublicKeyCredentialRpEntity,
          user: {
            id: b64urlToBytes(resp.user.id).buffer as ArrayBuffer,
            name: resp.user.id,
            displayName: resp.user.id,
          } as PublicKeyCredentialUserEntity,
          challenge: b64urlToBytes(resp.challenge).buffer as ArrayBuffer,
          pubKeyCredParams: resp.pubKeyCredParams as PublicKeyCredentialParameters[],
        },
      });
      if (!credential) throw new Error('No credential created');

      const pk = credential as PublicKeyCredential;
      const pkResp = pk.response as AuthenticatorAttestationResponse;
      const serialized = {
        id: pk.id,
        rawId: bytesToB64url(pk.rawId),
        type: pk.type,
        response: {
          clientDataJSON: bytesToB64url(pkResp.clientDataJSON),
          attestationObject: bytesToB64url(pkResp.attestationObject),
        },
      };

      const label = prompt('Name this passkey (e.g. "MacBook Touch ID"):') ?? 'Passkey';
      await api.finishPasskeyRegistration(serialized, label);
      await loadPasskeys();
      await auth.bootstrap();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to add passkey';
    } finally {
      busy = false;
    }
  }

  async function removePasskey(id: number) {
    busy = true;
    error = '';
    try {
      await api.deletePasskey(id);
      passkeys = passkeys.filter(p => p.id !== id);
      await auth.bootstrap();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to remove passkey';
    } finally {
      busy = false;
    }
  }

  function formatDate(ts: number): string {
    return new Date(ts * 1000).toLocaleString();
  }
</script>

<div class="security">
  <h2 id="security-heading">Security Settings</h2>
  {#if error}
    <p role="alert" class="error">{error}</p>
  {/if}

  <!-- Sessions -->
  <section>
    <h2>Active Sessions</h2>
    <button onclick={revokeAllOthers} disabled={busy}>Log out everywhere else</button>
    <table>
      <thead>
        <tr>
          <th>Device</th>
          <th>IP Address</th>
          <th>Last Seen</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each sessions as session (session.id)}
          <tr class:current={session.current}>
            <td>{session.user_agent || 'Unknown'}</td>
            <td>{session.address || 'Unknown'}</td>
            <td>{formatDate(session.last_seen_at)}</td>
            <td>
              {#if session.current}
                <em>current</em>
              {:else}
                <button onclick={() => revokeSession(session.id)} disabled={busy}>Revoke</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>

  <!-- TOTP -->
  <section>
    <h2>Authenticator App (TOTP)</h2>
    {#if authState?.user?.has_totp}
      {#if showRecoveryCodes && recoveryCodes.length > 0}
        <div class="codes">
          <p>Save these recovery codes. They will not be shown again.</p>
          <ul>
            {#each recoveryCodes as code}
              <li><code>{code}</code></li>
            {/each}
          </ul>
          <button onclick={() => { showRecoveryCodes = false; recoveryCodes = []; }}>I've saved these codes</button>
        </div>
      {:else if newRecoveryCodes.length > 0}
        <div class="codes">
          <p>New recovery codes. Save them now.</p>
          <ul>
            {#each newRecoveryCodes as code}
              <li><code>{code}</code></li>
            {/each}
          </ul>
          <button onclick={() => { newRecoveryCodes = []; }}>I've saved these codes</button>
        </div>
      {:else}
        <p>TOTP is enabled.</p>
        {#if showDisableTOTP}
          <div>
            <input type="text" bind:value={totpDisableCode} placeholder="6-digit code" maxlength="6" />
            <button onclick={disableTOTP} disabled={busy}>Disable 2FA</button>
            <button onclick={() => showDisableTOTP = false}>Cancel</button>
          </div>
        {:else}
          <button onclick={() => showDisableTOTP = true} disabled={busy}>Disable 2FA</button>
        {/if}
        {#if showRegenerateCodes}
          <div>
            <input type="text" bind:value={totpRegenCode} placeholder="6-digit code" maxlength="6" />
            <button onclick={regenerateCodes} disabled={busy}>Regenerate codes</button>
            <button onclick={() => showRegenerateCodes = false}>Cancel</button>
          </div>
        {:else}
          <button onclick={() => showRegenerateCodes = true} disabled={busy}>Regenerate recovery codes</button>
        {/if}
      {/if}
    {:else if totpEnrolment}
      <p>Scan this QR code with your authenticator app, or enter the key manually:</p>
      <code>{totpEnrolment.secret_uri}</code>
      <p>Secret key: <code>{totpEnrolment.secret}</code></p>
      <div>
        <input
          type="text"
          bind:value={totpConfirmCode}
          placeholder="6-digit code"
          maxlength="6"
          inputmode="numeric"
        />
        <button onclick={confirmTOTPEnrolment} disabled={busy}>Confirm</button>
        <button onclick={() => { totpEnrolment = null; totpConfirmCode = ''; }}>Cancel</button>
      </div>
    {:else}
      <button onclick={beginTOTPEnrolment} disabled={busy}>Set up authenticator app</button>
    {/if}
  </section>

  <!-- System Status (admin only) -->
  {#if authState?.user?.role === 'admin'}
    <SystemStatus />
  {/if}

  <!-- Passkeys -->
  <section>
    <h2>Passkeys</h2>
    {#each passkeys as passkey (passkey.id)}
      <div class="passkey-row">
        <span>{passkey.label} (added {formatDate(passkey.created_at)})</span>
        <button onclick={() => removePasskey(passkey.id)} disabled={busy}>Remove</button>
      </div>
    {/each}
    {#if typeof window !== 'undefined' && 'credentials' in navigator}
      <button onclick={addPasskey} disabled={busy}>Add a passkey</button>
    {/if}
  </section>
</div>

<style>
  .security { max-width: 48rem; margin: 2rem auto; }
  section { margin-top: 2rem; }
  h2 { border-bottom: 1px solid #ccc; padding-bottom: 0.5rem; }
  table { width: 100%; border-collapse: collapse; margin-top: 0.5rem; }
  th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #eee; }
  .current td { font-weight: bold; }
  .error { color: var(--color-danger, #b00); }
  .codes ul { list-style: none; columns: 2; }
  .codes li { margin: 0.25rem 0; }
  .passkey-row { display: flex; justify-content: space-between; margin: 0.5rem 0; }
</style>
