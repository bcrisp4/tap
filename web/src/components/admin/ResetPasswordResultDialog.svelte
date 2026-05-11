<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import Button from '../Button.svelte';

  type Props = {
    open: boolean;
    username: string;
    password: string;
    onClose?: () => void;
  };
  const { open, username, password, onClose }: Props = $props();

  let copied = $state(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(password);
      copied = true;
      setTimeout(() => copied = false, 1500);
    } catch (_) { /* ignore */ }
  }
</script>

<Dialog {open} title={`Temporary password for ${username}`} onClose={onClose}>
  <div class="body">
    <p>This password works once. <b>{username}</b> will be asked to set a new one on next sign-in. Share it through a secure channel — it will not be shown again.</p>
    <div class="key-row">
      <span class="key">{password}</span>
      <button type="button" class="copy" onclick={copy}>
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="2.5" width="8" height="8" rx="1.2"/><path d="M5.5 13.5h6a1.2 1.2 0 0 0 1.2-1.2v-6"/></svg>
        <span>{copied ? 'Copied' : 'Copy'}</span>
      </button>
    </div>
    <div class="warn">All existing sessions for {username} were just signed out. Any passkeys remain valid.</div>
  </div>
  {#snippet foot()}
    <div class="foot-l">shown once</div>
    <Button variant="primary" onclick={onClose}>Done</Button>
  {/snippet}
</Dialog>

<style>
  .body { display: flex; flex-direction: column; gap: 14px; }
  p { font-family: var(--sans); font-size: 13.5px; color: var(--ink-2); margin: 0; line-height: 1.55; }
  .key-row {
    display: flex; align-items: center; gap: 12px;
    padding: 14px 16px;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: 4px;
  }
  .key { font-family: var(--mono); font-size: 15px; letter-spacing: 0.06em; color: var(--ink); font-weight: 500; flex: 1; }
  .copy {
    display: inline-flex; align-items: center; gap: 6px;
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    background: transparent;
    border: 1px solid var(--rule); border-radius: 3px;
    padding: 5px 9px; cursor: pointer;
  }
  .copy:hover { color: var(--ink); border-color: var(--ink-4); }
  .warn {
    font-family: var(--mono); font-size: 11px; color: var(--ink-3);
    padding: 10px 14px;
    border-left: 2px solid var(--accent);
    background: var(--accent-soft);
  }
  .foot-l { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-right: auto; }
</style>
