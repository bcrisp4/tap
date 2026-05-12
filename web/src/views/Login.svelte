<!--
  Login is mounted by App.svelte under two conditions:
   (1) URL is /sign-in (route name 'signin'), OR
   (2) $auth.user == null on any other URL (state-aware fallback).
  App.svelte's redirect $effect keeps the two in sync: unauthenticated
  users are pushed to /sign-in; authenticated users on /sign-in are
  pushed to /. Magic-link mode is intentionally out of scope (umbrella §1).
-->
<script lang="ts">
  import Button from '../components/Button.svelte';
  import Field from '../components/Field.svelte';
  import KbdChip from '../components/KbdChip.svelte';
  import OtpInput from '../components/OtpInput.svelte';
  import { auth, ERR_UNAUTHORIZED } from '../lib/auth';
  import { isMobile } from '../lib/breakpoints.svelte';

  type Mode = 'password' | 'passkey' | 'otp';

  let mode = $state<Mode>('password');
  let username = $state('');
  let password = $state('');
  let pendingToken = $state('');
  let otp = $state('');
  let useRecovery = $state(false);
  let recovery = $state('');
  let error = $state('');
  let busy = $state(false);

  const passkeyAvailable = typeof window !== 'undefined' && 'credentials' in navigator;

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      if (mode === 'otp') {
        await auth.loginWithTOTP(
          pendingToken,
          useRecovery ? undefined : otp,
          useRecovery ? recovery : undefined,
        );
      } else {
        const result = await auth.login(username, password) as { totp_required?: boolean; pending_token?: string };
        if (result?.totp_required && result.pending_token) {
          pendingToken = result.pending_token;
          mode = 'otp';
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
      const credential = await navigator.credentials.get({
        publicKey: parseRequestOptions(options as PublicKeyCredentialRequestOptionsJSON),
      });
      if (!credential) throw new Error('No credential returned');
      await auth.finishPasskeyLogin(session_id, serializeAssertion(credential as PublicKeyCredential));
    } catch (err) {
      error = err instanceof Error ? err.message : 'Passkey login failed.';
    } finally {
      busy = false;
    }
  }

  interface PublicKeyCredentialRequestOptionsJSON {
    challenge: string;
    rpId?: string;
    allowCredentials?: Array<{ id: string; type: string; transports?: string[] }>;
    userVerification?: string;
    timeout?: number;
  }
  function b64urlToBytes(b64: string): Uint8Array {
    const pad = b64.length % 4 === 0 ? '' : '='.repeat(4 - (b64.length % 4));
    const std = (b64 + pad).replace(/-/g, '+').replace(/_/g, '/');
    return Uint8Array.from(atob(std), c => c.charCodeAt(0));
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

<div class="tl-root" class:is-mobile={$isMobile}>
  <header class="tl-header">
    <a class="wordmark" href="/" aria-label="Tap home">tap<span class="dot" aria-hidden="true"></span></a>
  </header>
  <main class="tl-main">
    <div class="tl-col">
      <form class="tl-form" onsubmit={submit}>
        <h1 class="tl-title">
          {mode === 'otp' ? 'Verification code' : 'Sign in'}
        </h1>

        {#if error}
          <div class="tl-error" role="alert">
            <svg class="tl-error-ico" width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M8 2 1.5 13.5h13L8 2Z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/>
              <path d="M8 6.5v3.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
              <circle cx="8" cy="11.7" r="0.7" fill="currentColor"/>
            </svg>
            <span>{error}</span>
          </div>
        {/if}

        {#if mode === 'password'}
          <Field
            label="USERNAME"
            bind:value={username}
            type="text"
            mono
            autofocus
            autocomplete="username"
            required
          />
          <Field
            label="PASSWORD"
            bind:value={password}
            type="password"
            autocomplete="current-password"
            required
          />
        {:else if mode === 'otp'}
          {#if !useRecovery}
            <label class="otp-row">
              <span class="otp-label">CODE</span>
              <OtpInput value={otp} onChange={(v) => otp = v} disabled={busy} />
            </label>
          {:else}
            <Field
              label="RECOVERY CODE"
              bind:value={recovery}
              type="text"
              mono
              autofocus
              required
            />
          {/if}
        {/if}

        <div class="tl-actions">
          <Button type="submit" variant="primary" disabled={busy}>
            <span>{mode === 'otp' ? 'Verify and continue' : 'Continue'}</span>
            <span aria-hidden="true">→</span>
            <KbdChip>Enter</KbdChip>
          </Button>
          {#if mode === 'password' && passkeyAvailable}
            <Button variant="quiet" disabled={busy} onclick={signInWithPasskey}>
              Use a passkey
            </Button>
          {:else if mode === 'otp'}
            <Button variant="quiet" onclick={() => { useRecovery = !useRecovery; otp = ''; recovery = ''; }}>
              {useRecovery ? 'Use authenticator code' : 'Use a recovery code'}
            </Button>
          {/if}
        </div>
      </form>
    </div>
  </main>
</div>

<style>
  .tl-root {
    background: var(--bg); color: var(--ink);
    font-family: var(--serif);
    min-height: 100vh;
    display: flex; flex-direction: column;
  }
  .tl-header { padding: 22px 32px; }
  .wordmark { font-family: var(--sans); font-weight: 600; font-size: 20px; letter-spacing: -0.02em; color: var(--ink); text-decoration: none; display: inline-flex; align-items: baseline; gap: 1px; }
  .dot { display: inline-block; width: 5px; height: 5px; border-radius: 50%; background: var(--accent); transform: translateY(-1px); margin-left: 1px; }
  .tl-main { flex: 1; display: flex; align-items: center; justify-content: center; padding: 24px; }
  .tl-col { width: 100%; max-width: 380px; }
  .tl-form { display: flex; flex-direction: column; gap: 14px; }
  .tl-title { font-family: var(--serif); font-weight: 600; font-size: 34px; line-height: 1.05; letter-spacing: -0.02em; margin: 0 0 4px; color: var(--ink); text-wrap: balance; }
  .tl-error {
    display: flex; gap: 10px; align-items: flex-start;
    background: rgba(196, 58, 58, 0.06);
    border: 1px solid rgba(196, 58, 58, 0.28);
    border-left-width: 2px; border-left-color: #c43a3a;
    border-radius: 4px;
    padding: 10px 12px;
    color: #c43a3a;
    font-family: var(--sans); font-size: 13px; line-height: 1.45;
  }
  :global(html.theme-dark) .tl-error {
    background: rgba(236, 122, 122, 0.06);
    border-color: rgba(236, 122, 122, 0.32);
    border-left-color: #ec7a7a;
    color: #ec7a7a;
  }
  .tl-error-ico { margin-top: 1px; flex-shrink: 0; }
  .tl-actions { display: flex; flex-direction: column; gap: 8px; margin-top: 14px; }
  .tl-actions :global(.btn.variant-primary) { min-height: 42px; }
  .otp-row { display: flex; flex-direction: column; gap: 8px; }
  .otp-label { font-family: var(--mono); font-size: 10px; letter-spacing: 0.14em; text-transform: uppercase; color: var(--ink-3); }
  /* Mobile overrides per tap-login.css §MOBILE */
  .tl-root.is-mobile { padding-top: 50px; }
  .tl-root.is-mobile .tl-title { font-size: 28px; }
  /* Hide keyboard shortcut hint chip on mobile (no hardware kbd) */
  .tl-root.is-mobile :global(.kbd) { display: none; }
</style>
