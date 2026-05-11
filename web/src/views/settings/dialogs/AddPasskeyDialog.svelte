<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import Field from '../../../components/Field.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';

  interface Props { onClose: () => void; onSaved: () => void; }
  let { onClose, onSaved }: Props = $props();

  let label = $state('My device');
  let busy = $state(false);
  let error = $state('');

  function b64urlToBytes(b64: string): Uint8Array {
    const pad = b64.length % 4 === 0 ? '' : '='.repeat(4 - (b64.length % 4));
    const b64std = (b64 + pad).replace(/-/g, '+').replace(/_/g, '/');
    return Uint8Array.from(atob(b64std), c => c.charCodeAt(0));
  }
  function bytesToB64url(buf: ArrayBuffer): string {
    return btoa(String.fromCharCode(...new Uint8Array(buf)))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  }

  async function submit() {
    error = ''; busy = true;
    try {
      const opts = await api.beginPasskeyRegistration() as { response: {
        user: { id: string };
        challenge: string;
        rp: PublicKeyCredentialRpEntity;
        pubKeyCredParams: PublicKeyCredentialParameters[];
      } };
      const credential = await navigator.credentials.create({
        publicKey: {
          rp: opts.response.rp,
          user: {
            id: b64urlToBytes(opts.response.user.id).buffer as ArrayBuffer,
            name: opts.response.user.id,
            displayName: opts.response.user.id,
          },
          challenge: b64urlToBytes(opts.response.challenge).buffer as ArrayBuffer,
          pubKeyCredParams: opts.response.pubKeyCredParams,
        },
      });
      if (!credential) throw new Error('No credential created');
      const pk = credential as PublicKeyCredential;
      const r = pk.response as AuthenticatorAttestationResponse;
      await api.finishPasskeyRegistration({
        id: pk.id,
        rawId: bytesToB64url(pk.rawId),
        type: pk.type,
        response: {
          clientDataJSON: bytesToB64url(r.clientDataJSON),
          attestationObject: bytesToB64url(r.attestationObject),
        },
      }, label);
      await auth.bootstrap();
      onSaved();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not add passkey.';
    } finally { busy = false; }
  }
</script>

<Dialog open title="Add a passkey" {onClose}>
  {#snippet children()}
    <p class="p">Give this passkey a memorable label so you can recognise it later — your device's name, or where it lives.</p>
    <Field label="Label" bind:value={label} />
    <p class="p sm">On the next step, your browser will ask which device or security key to use.</p>
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button onclick={onClose}>Cancel</Button>
    <Button variant="primary" onclick={submit} disabled={busy || !label.trim()}>Use this device →</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .p.sm { font-size: 13.5px; margin-top: 18px; margin-bottom: 0; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
</style>
