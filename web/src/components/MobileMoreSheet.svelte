<script lang="ts">
  import { auth } from '../lib/auth';
  import { navigate } from '../lib/router';
  type Props = { open: boolean; onClose: () => void };
  let { open, onClose }: Props = $props();

  const items = $derived([
    { id: 'history',  label: 'History',  desc: "Everything you've read.", path: '/history' },
    { id: 'settings', label: 'Settings', desc: 'Theme, account, security.', path: '/settings' },
    ...($auth.user?.role === 'admin'
      ? [{ id: 'admin', label: 'Admin', desc: 'Operator surface.', path: '/admin' }]
      : []),
  ]);

  function pick(path: string) { onClose(); navigate(path); }
  async function logout() { onClose(); await auth.logout(); window.location.assign('/'); }
</script>

{#if open}
  <div class="tmnav-sheet-backdrop" onclick={onClose} role="presentation"></div>
  <div class="tmnav-sheet" role="dialog" aria-label="More">
    <div class="handle" aria-hidden="true"></div>
    {#if $auth.user}
      <div class="identity">
        <span class="avatar" aria-hidden="true">{($auth.user.username[0] ?? '?').toUpperCase()}</span>
        <span class="who">
          <span class="name">{$auth.user.username}</span>
          <span class="email">user #{$auth.user.id}</span>
        </span>
      </div>
    {/if}
    <div class="list">
      {#each items as it (it.id)}
        <button type="button" class="item" onclick={() => pick(it.path)}>
          <span class="ico"></span>
          <span class="body">
            <span class="name">{it.label}</span>
            <span class="desc">{it.desc}</span>
          </span>
          <span class="chev" aria-hidden="true">›</span>
        </button>
      {/each}
      <div class="sep" aria-hidden="true"></div>
      <button type="button" class="item is-logout" onclick={logout}>
        <span class="ico"></span>
        <span class="body">
          <span class="name">Log out</span>
          <span class="desc">Sign out of {$auth.user?.username ?? ''}</span>
        </span>
      </button>
    </div>
    <div class="foot">
      <button type="button" class="cancel" onclick={onClose}>Close</button>
    </div>
  </div>
{/if}

<style>
  .tmnav-sheet-backdrop {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.32);
    z-index: 90;
  }
  .tmnav-sheet {
    position: fixed; left: 0; right: 0; bottom: 0;
    z-index: 91;
    background: var(--bg);
    border-radius: 18px 18px 0 0;
    border-top: 1px solid var(--rule);
    padding: 14px 18px calc(env(safe-area-inset-bottom, 0px) + 16px);
    max-height: 80vh; overflow: auto;
  }
  .handle { width: 36px; height: 4px; background: var(--ink-4); border-radius: 4px; margin: 0 auto 14px; }
  .identity { display: flex; align-items: center; gap: 12px; padding: 0 4px 14px; border-bottom: 1px solid var(--rule); }
  .avatar {
    width: 36px; height: 36px; border-radius: 4px;
    background: var(--accent); color: #fff;
    display: inline-flex; align-items: center; justify-content: center;
    font-family: var(--mono); font-size: 15px; font-weight: 600;
  }
  .who { display: flex; flex-direction: column; gap: 2px; }
  .who .name { font-family: var(--sans); font-size: 15px; font-weight: 600; color: var(--ink); }
  .who .email { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  .list { padding-top: 8px; }
  .item {
    display: grid; grid-template-columns: 36px 1fr 18px;
    align-items: center; gap: 12px;
    width: 100%;
    padding: 12px 6px;
    background: transparent; border: 0;
    text-align: left; cursor: pointer;
  }
  .item:active { background: var(--bg-soft); }
  .item .ico { width: 36px; height: 36px; }
  .item .body { display: flex; flex-direction: column; gap: 2px; }
  .item .name { font-family: var(--sans); font-size: 15px; font-weight: 600; color: var(--ink); }
  .item .desc { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); }
  .chev { color: var(--ink-4); font-size: 18px; }
  .sep { height: 1px; background: var(--rule); margin: 6px 0; }
  .is-logout .name { color: #b3402c; }
  .is-logout .desc { color: #b3402c; opacity: 0.85; }
  :global(html.theme-dark) .is-logout .name { color: #e9846f; }
  :global(html.theme-dark) .is-logout .desc { color: #e9846f; }
  .foot { display: flex; justify-content: center; padding: 12px 0 4px; }
  .cancel {
    font-family: var(--mono); font-size: 11px;
    color: var(--ink-3);
    padding: 8px 12px; background: transparent; border: 0; cursor: pointer;
    text-transform: uppercase; letter-spacing: 0.12em;
  }
  .cancel:active { background: var(--bg-soft); }
</style>
