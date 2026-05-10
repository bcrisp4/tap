<script lang="ts">
  import { auth, ERR_UNAUTHORIZED } from '../lib/auth';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  // TOTP second-step state.
  let pendingToken = $state('');
  let totpCode = $state('');
  let useRecovery = $state(false);
  let recoveryCode = $state('');
  let showTOTPStep = $state(false);

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      if (showTOTPStep) {
        await auth.loginWithTOTP(
          pendingToken,
          useRecovery ? undefined : totpCode,
          useRecovery ? recoveryCode : undefined,
        );
      } else {
        const result = await auth.login(username, password) as { totp_required?: boolean; pending_token?: string };
        if (result?.totp_required && result.pending_token) {
          pendingToken = result.pending_token;
          showTOTPStep = true;
        }
      }
    } catch (err) {
      error = err instanceof Error && err.message !== ERR_UNAUTHORIZED
        ? err.message
        : 'Invalid username or password.';
    } finally {
      busy = false;
    }
  }

  async function signInWithPasskey() {
    error = '';
    busy = true;
    try {
      const { session_id, options } = await auth.beginPasskeyLogin();
      const opts = options as PublicKeyCredentialRequestOptionsJSON;
      // Use the WebAuthn browser API.
      const credential = await navigator.credentials.get({
        publicKey: parseRequestOptions(opts),
      });
      if (!credential) throw new Error('No credential returned');
      await auth.finishPasskeyLogin(session_id, serializeAssertion(credential as PublicKeyCredential));
    } catch (err) {
      error = err instanceof Error ? err.message : 'Passkey login failed.';
    } finally {
      busy = false;
    }
  }

  // Minimal WebAuthn helpers — convert server options to browser API format.
  interface PublicKeyCredentialRequestOptionsJSON {
    challenge: string;
    rpId?: string;
    allowCredentials?: Array<{ id: string; type: string; transports?: string[] }>;
    userVerification?: string;
    timeout?: number;
  }

  function b64urlToBytes(b64: string): Uint8Array {
    const pad = b64.length % 4 === 0 ? '' : '='.repeat(4 - (b64.length % 4));
    const b64standard = (b64 + pad).replace(/-/g, '+').replace(/_/g, '/');
    return Uint8Array.from(atob(b64standard), c => c.charCodeAt(0));
  }

  function bytesToB64url(buf: ArrayBuffer): string {
    return btoa(String.fromCharCode(...new Uint8Array(buf)))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  }

  function parseRequestOptions(opts: PublicKeyCredentialRequestOptionsJSON): PublicKeyCredentialRequestOptions {
    return {
      challenge: b64urlToBytes(opts.challenge).buffer as ArrayBuffer,
      rpId: opts.rpId,
      allowCredentials: opts.allowCredentials?.map(c => ({
        id: b64urlToBytes(c.id).buffer as ArrayBuffer,
        type: c.type as PublicKeyCredentialType,
        transports: c.transports as AuthenticatorTransport[] | undefined,
      })),
      userVerification: opts.userVerification as UserVerificationRequirement | undefined,
      timeout: opts.timeout,
    };
  }

  function serializeAssertion(cred: PublicKeyCredential) {
    const resp = cred.response as AuthenticatorAssertionResponse;
    return {
      id: cred.id,
      rawId: bytesToB64url(cred.rawId),
      type: cred.type,
      response: {
        authenticatorData: bytesToB64url(resp.authenticatorData),
        clientDataJSON: bytesToB64url(resp.clientDataJSON),
        signature: bytesToB64url(resp.signature),
        userHandle: resp.userHandle ? bytesToB64url(resp.userHandle) : null,
      },
    };
  }
</script>

{#if showTOTPStep}
  <form onsubmit={submit}>
    <h1>Two-factor authentication</h1>
    {#if !useRecovery}
      <label>
        6-digit code
        <input
          type="text"
          inputmode="numeric"
          pattern="[0-9]{6}"
          maxlength="6"
          autocomplete="one-time-code"
          bind:value={totpCode}
          disabled={busy}
          required
          placeholder="000000"
        />
      </label>
    {:else}
      <label>
        Recovery code
        <input
          type="text"
          bind:value={recoveryCode}
          disabled={busy}
          required
          placeholder="XXXXXXXXXX"
        />
      </label>
    {/if}
    <button type="button" onclick={() => { useRecovery = !useRecovery; totpCode = ''; recoveryCode = ''; }}>
      {useRecovery ? 'Use authenticator code instead' : 'Use a recovery code instead'}
    </button>
    {#if error}
      <p role="alert" class="error">{error}</p>
    {/if}
    <button type="submit" disabled={busy}>
      {busy ? 'Verifying…' : 'Verify'}
    </button>
  </form>
{:else}
  <form onsubmit={submit}>
    <h1>Sign in to Tap</h1>
    <label>
      Username
      <input
        type="text"
        autocomplete="username"
        bind:value={username}
        disabled={busy}
        required
      />
    </label>
    <label>
      Password
      <input
        type="password"
        autocomplete="current-password"
        bind:value={password}
        disabled={busy}
        required
      />
    </label>
    {#if error}
      <p role="alert" class="error">{error}</p>
    {/if}
    <button type="submit" disabled={busy}>
      {busy ? 'Signing in…' : 'Sign in'}
    </button>
    {#if typeof window !== 'undefined' && 'credentials' in navigator}
      <button type="button" onclick={signInWithPasskey} disabled={busy} class="passkey-btn">
        Sign in with a passkey
      </button>
    {/if}
  </form>
{/if}

<style>
  form {
    max-width: 22rem;
    margin: 4rem auto;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  h1 {
    text-align: center;
    margin-bottom: 0.5rem;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  input {
    padding: 0.5rem;
    font-family: var(--sans);
    font-size: 14px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg-soft);
    color: var(--ink);
  }
  .error {
    color: var(--color-danger, #b00);
  }
  .passkey-btn {
    background: none;
    border: 1px solid currentColor;
  }
  /* TOTP 6-digit input: monospaced, wide letter-spacing for code readability */
  input[inputmode="numeric"] {
    font-family: var(--mono);
    letter-spacing: 0.2em;
    font-size: 18px;
  }
  /* Recovery code toggle as a secondary text link */
  button[type="button"]:not(.passkey-btn) {
    color: var(--accent);
    font-family: var(--sans);
    font-size: 12px;
    text-decoration: underline;
    text-align: left;
    padding: 0;
  }
</style>
