<script lang="ts">
  import type { AdminUser } from '../lib/types';

  type Action = 'reset' | 'disable2fa' | 'disableUser' | 'enable' | 'delete';

  type Props = {
    users: AdminUser[];
    currentUserId: number;
    onAction?: (action: Action, user: AdminUser) => void;
  };

  const { users, currentUserId, onAction }: Props = $props();

  function fmtDate(ts: number): string {
    return new Date(ts * 1000).toLocaleDateString('en-US', { month: 'short', day: '2-digit', year: 'numeric' });
  }
</script>

<div class="table">
  <div class="head">
    <div>User</div><div>Role</div><div>Created</div><div>Status</div><div>2FA</div><div>Passkeys</div><div>Actions</div>
  </div>
  {#each users as u (u.id)}
    {@const isYou = u.id === currentUserId}
    {@const disabled = !!u.disabled_at}
    <div class="row" class:disabled class:is-you={isYou}>
      <div class="cell user">
        <span class="avatar" aria-hidden="true">{u.username[0].toUpperCase()}</span>
        <div class="stack">
          <span class="name">{u.username}{#if isYou}<span class="you">you</span>{/if}</span>
        </div>
      </div>
      <div class="cell"><span class="role role-{u.role}"><span class="dot"></span>{u.role}</span></div>
      <div class="cell date">{fmtDate(u.created_at)}</div>
      <div class="cell" data-testid="urow-status-{u.id}">
        <span class="pill pill-{disabled ? 'muted' : 'active'}">{disabled ? 'disabled' : 'active'}</span>
      </div>
      <div class="cell" data-testid="urow-totp-{u.id}">
        <span class="flag flag-{u.has_totp ? 'on' : 'off'}">{u.has_totp ? 'enabled' : 'not set'}</span>
      </div>
      <div class="cell pk" data-testid="urow-pk-{u.id}">{u.passkey_count}</div>
      <div class="cell actions">
        <button type="button" data-testid="urow-action-reset-{u.id}" onclick={() => onAction?.('reset', u)}>Reset password</button>
        <button type="button" data-testid="urow-action-disable2fa-{u.id}" disabled={!u.has_totp} onclick={() => onAction?.('disable2fa', u)}>Disable 2FA</button>
        {#if disabled}
          <button type="button" data-testid="urow-action-enable-{u.id}" onclick={() => onAction?.('enable', u)}>Re-enable</button>
        {:else}
          <button type="button" data-testid="urow-action-disableUser-{u.id}" disabled={isYou} onclick={() => onAction?.('disableUser', u)}>Disable</button>
        {/if}
        <button type="button" class="danger" data-testid="urow-action-delete-{u.id}" disabled={isYou} onclick={() => onAction?.('delete', u)}>Delete</button>
      </div>
    </div>
  {/each}
</div>

<style>
  .table {
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg);
    overflow: hidden;
  }
  .head, .row {
    display: grid;
    grid-template-columns: minmax(0, 1.7fr) 96px 110px 92px 110px 84px minmax(0, 1.6fr);
    align-items: center;
    gap: 14px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--rule);
  }
  .head {
    font-family: var(--mono);
    font-size: 9.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-3);
    background: var(--bg-soft);
  }
  .row:last-child { border-bottom: 0; }
  .row:hover { background: var(--bg-soft); }
  .row.disabled .name, .row.disabled .date { color: var(--ink-3); }
  .row.disabled .avatar { opacity: 0.55; }
  .cell { font-family: var(--sans); font-size: 13.5px; color: var(--ink); min-width: 0; }
  .user { display: flex; align-items: center; gap: 10px; }
  .avatar {
    width: 28px; height: 28px;
    border-radius: 50%;
    background: var(--accent-soft);
    color: var(--accent);
    display: inline-flex; align-items: center; justify-content: center;
    font-family: var(--mono); font-size: 12px; font-weight: 500;
  }
  .stack { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
  .name { display: inline-flex; gap: 6px; align-items: center; }
  .you {
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--ink-3);
    border: 1px solid var(--rule); padding: 1px 5px; border-radius: 2px;
  }
  .date { font-family: var(--mono); font-size: 12px; color: var(--ink-2); }
  .pk { display: inline-flex; align-items: center; gap: 6px; color: var(--ink-2); font-family: var(--mono); font-size: 12px; }
  .role {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    padding: 2px 8px; border-radius: 999px;
    border: 1px solid var(--rule); color: var(--ink-2);
    display: inline-flex; align-items: center; gap: 6px;
  }
  .role-admin { color: var(--accent); border-color: var(--accent); background: var(--accent-soft); }
  .dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
  .pill {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    display: inline-flex; align-items: center; gap: 6px;
  }
  .pill::before { content: ''; display: inline-block; width: 6px; height: 6px; border-radius: 50%; }
  .pill-active { color: var(--ink); }
  .pill-active::before { background: #4a9a4a; }
  :global(html.theme-dark) .pill-active::before { background: #6dbf6d; }
  .pill-muted { color: var(--ink-3); }
  .pill-muted::before { background: var(--ink-4); }
  .flag { font-family: var(--mono); font-size: 12px; }
  .flag-on { color: var(--ink); }
  .flag-off { color: var(--ink-3); font-style: italic; }
  .actions { display: inline-flex; gap: 4px; flex-wrap: wrap; justify-content: flex-end; }
  .actions button {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    color: var(--ink-2);
    background: transparent;
    border: 1px solid transparent;
    border-radius: 3px;
    padding: 4px 8px;
    cursor: pointer;
  }
  .actions button:hover { color: var(--ink); border-color: var(--rule); background: var(--bg-soft); }
  .actions button:disabled { color: var(--ink-4); cursor: not-allowed; }
  .actions button:disabled:hover { background: transparent; border-color: transparent; }
  .actions button.danger:hover { color: #c43a3a; border-color: rgba(196, 58, 58, 0.4); background: rgba(196, 58, 58, 0.04); }
  :global(html.theme-dark) .actions button.danger:hover { color: #ec7a7a; }
  @media (max-width: 920px) {
    .head { display: none; }
    .row { grid-template-columns: 1fr; gap: 6px; }
  }
</style>
