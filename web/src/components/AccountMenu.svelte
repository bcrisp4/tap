<script lang="ts">
  import Popover from './Popover.svelte';
  import type { User } from '../lib/types';
  import { navigate } from '../lib/router';
  type Props = { open: boolean; user: User; onClose: () => void; onLogout: () => void };
  let { open, user, onClose, onLogout }: Props = $props();

  function go(path: string) { onClose(); navigate(path); }
</script>

<Popover {open} {onClose} anchor="top-right">
  <div class="head">
    <div class="name">{user.username}</div>
    <div class="email">user #{user.id}</div>
  </div>
  <button class="item" type="button" onclick={() => go('/settings')}>
    <span class="ico" aria-hidden="true">
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="6" r="2.5"/><path d="M3.5 13.5a4.5 4.5 0 0 1 9 0"/></svg>
    </span>
    <span>Account settings</span>
  </button>
  <button class="item" type="button" onclick={() => go('/settings')}>
    <span class="ico" aria-hidden="true"></span>
    <span>Switch theme</span>
  </button>
  <div class="sep" aria-hidden="true"></div>
  <button class="item logout" type="button" onclick={() => { onClose(); onLogout(); }}>
    <span class="ico" aria-hidden="true">
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 3.5H4.5a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h5"/><path d="m11 5.5 2.5 2.5L11 10.5"/><path d="M7 8h6.5"/></svg>
    </span>
    <span>Log out</span>
  </button>
</Popover>

<style>
  .head { padding: 10px 12px 12px; border-bottom: 1px solid var(--rule); margin-bottom: 4px; }
  .name { font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink); line-height: 1.2; }
  .email { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-top: 3px; line-height: 1.2; }
  .item {
    display: flex; align-items: center; gap: 10px;
    width: 100%; padding: 6px 10px;
    background: transparent; border: 0;
    color: var(--ink-2);
    font-family: var(--mono); font-size: 11.5px;
    letter-spacing: 0.01em; cursor: pointer;
    border-radius: 4px;
    text-align: left;
  }
  .item:hover { background: rgba(0,0,0,0.06); color: var(--ink); }
  .ico { width: 16px; display: inline-flex; align-items: center; justify-content: center; color: var(--ink-3); }
  .sep { height: 1px; background: var(--rule); margin: 4px 0; }
  .logout { color: #b3402c; }
  .logout:hover { background: rgba(179, 64, 44, 0.08); color: #b3402c; }
  :global(html.theme-dark) .logout { color: #e9846f; }
  :global(html.theme-dark) .logout:hover { background: rgba(233, 132, 111, 0.10); color: #e9846f; }
</style>
