<script lang="ts">
  import { auth } from '../lib/auth';
  import AppearanceSection from './settings/AppearanceSection.svelte';
  import ReadingSection from './settings/ReadingSection.svelte';
  import SyncingSection from './settings/SyncingSection.svelte';
  import AccountSection from './settings/AccountSection.svelte';
  import SecurityTOTPSection from './settings/SecurityTOTPSection.svelte';
  import SecurityPasskeysSection from './settings/SecurityPasskeysSection.svelte';
  import SessionsSection from './settings/SessionsSection.svelte';
  import DataSection from './settings/DataSection.svelte';

  const email = $derived($auth.user?.username ?? '');
  const role = $derived($auth.user?.role ?? 'user');
</script>

<div class="set">
  <header class="set-head">
    <div class="set-eyebrow">Settings</div>
    <h1 class="set-title">Account &amp; appearance</h1>
    <div class="set-id">
      <span>account</span>
      <span class="dot" aria-hidden="true"></span>
      <span class="email">{email}</span>
      <span class="dot" aria-hidden="true"></span>
      <span>plan: free</span>
      {#if role === 'admin'}
        <span class="dot" aria-hidden="true"></span>
        <span class="admin">admin</span>
      {/if}
    </div>
  </header>

  <AppearanceSection />
  <ReadingSection />
  <SyncingSection />
  <AccountSection />
  <SecurityTOTPSection />
  <SecurityPasskeysSection />
  <SessionsSection />
  <DataSection />
</div>

<style>
  .set { padding: 28px 0 24px; }
  .set-head { padding-bottom: 22px; border-bottom: 1px solid var(--rule); margin-bottom: 12px; }
  .set-eyebrow { font-family: var(--mono); font-size: 10px; letter-spacing: 0.14em; text-transform: uppercase; color: var(--ink-3); margin-bottom: 10px; }
  .set-title { font-family: var(--serif); font-size: 38px; line-height: 1.05; font-weight: 600; color: var(--ink); letter-spacing: -0.02em; margin: 0 0 8px; }
  .set-id { display: inline-flex; align-items: center; gap: 8px; font-family: var(--mono); font-size: 11.5px; color: var(--ink-3); }
  .set-id .email { color: var(--ink-2); }
  .set-id .dot { display: inline-block; width: 4px; height: 4px; border-radius: 50%; background: var(--ink-4); }
  .set-id .admin { color: var(--accent); }
</style>
