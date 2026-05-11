<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '../lib/auth';
  import { navigate } from '../lib/router';
  import { api } from '../lib/api';
  import { getStatus, type StatusResponse } from '../lib/status';
  import type { AdminUser } from '../lib/types';
  import EmptyState from '../components/EmptyState.svelte';
  import SysGrid from '../components/SysGrid.svelte';
  import ErrorsTable from '../components/ErrorsTable.svelte';
  import UserTable from '../components/UserTable.svelte';
  import AdminToolbar, { type AdminFilter } from '../components/AdminToolbar.svelte';
  import CreateUserDialog from '../components/admin/CreateUserDialog.svelte';
  import ResetPasswordResultDialog from '../components/admin/ResetPasswordResultDialog.svelte';
  import ConfirmDialog from '../components/admin/ConfirmDialog.svelte';

  const authState = $derived(get(auth));
  const isAdmin = $derived(authState?.user?.role === 'admin');
  const currentUserId = $derived(authState?.user?.id ?? -1);

  let users = $state<AdminUser[]>([]);
  let status = $state<StatusResponse | null>(null);
  let loadError = $state('');
  let busy = $state(false);
  let pollInterval = $state<ReturnType<typeof setInterval> | null>(null);
  let nowSec = $state(Math.floor(Date.now() / 1000));

  let filter = $state<AdminFilter>('all');
  let query = $state('');

  let overlay = $state<'create' | 'resetConfirm' | 'resetResult' | 'disable2fa' | 'disableUser' | 'enable' | 'delete' | null>(null);
  let overlayUser = $state<AdminUser | null>(null);
  let tempPassword = $state('');

  const filteredUsers = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return users.filter((u) => {
      if (q && !u.username.toLowerCase().includes(q)) return false;
      switch (filter) {
        case 'admins':   return u.role === 'admin';
        case 'users':    return u.role === 'user';
        case 'disabled': return !!u.disabled_at;
        default:         return true;
      }
    });
  });

  onMount(async () => {
    if (!isAdmin) {
      navigate('/');
      return;
    }
    await Promise.all([loadUsers(), loadStatus()]);
    pollInterval = setInterval(() => {
      nowSec = Math.floor(Date.now() / 1000);
      loadStatus();
    }, 60_000);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  async function loadUsers() {
    try {
      users = await api.listUsers();
      loadError = '';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Failed to load users';
    }
  }

  async function loadStatus() {
    try {
      status = await getStatus();
    } catch (e) {
      if (!status) loadError = e instanceof Error ? e.message : 'status load failed';
    }
  }

  function onAction(action: 'reset' | 'disable2fa' | 'disableUser' | 'enable' | 'delete', user: AdminUser) {
    overlayUser = user;
    if (action === 'reset')            overlay = 'resetConfirm';
    else if (action === 'disable2fa')  overlay = 'disable2fa';
    else if (action === 'disableUser') overlay = 'disableUser';
    else if (action === 'enable')      overlay = 'enable';
    else if (action === 'delete')      overlay = 'delete';
  }

  function closeOverlay() {
    overlay = null;
    overlayUser = null;
    tempPassword = '';
  }

  async function handleCreate(v: { username: string; password: string; role: 'admin' | 'user' }) {
    busy = true;
    try {
      const u = await api.createUser(v.username, v.password, v.role);
      users = [...users, u];
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Create user failed';
    } finally {
      busy = false;
    }
  }

  async function handleResetConfirm() {
    if (!overlayUser) return;
    busy = true;
    try {
      const r = await api.resetUserPassword(overlayUser.id);
      tempPassword = r.temporary_password;
      overlay = 'resetResult';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Reset password failed';
      closeOverlay();
    } finally {
      busy = false;
    }
  }

  async function handleDisable2FA() {
    if (!overlayUser) return;
    busy = true;
    try {
      await api.disableUserTOTP(overlayUser.id);
      await loadUsers();
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Disable 2FA failed';
    } finally {
      busy = false;
    }
  }

  async function handleToggleDisabled() {
    if (!overlayUser) return;
    busy = true;
    try {
      const updated = await api.patchUser(overlayUser.id, { disabled: !overlayUser.disabled_at });
      users = users.map((u) => (u.id === updated.id ? updated : u));
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Toggle disable failed';
    } finally {
      busy = false;
    }
  }

  async function handleDelete() {
    if (!overlayUser) return;
    busy = true;
    try {
      await api.deleteUser(overlayUser.id);
      users = users.filter((u) => u.id !== overlayUser!.id);
      closeOverlay();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'Delete failed';
    } finally {
      busy = false;
    }
  }
</script>

{#if !isAdmin}
  <main class="ts-shell">
    <EmptyState title="Access denied" description="You don't have permission to view this page." />
  </main>
{:else}
  <main class="ts-shell ts-shell-admin">
    <header class="head">
      <div class="eyebrow">Admin</div>
      <h1 class="title">Instance &amp; users</h1>
      <div class="id">
        <span>Signed in as <b>{authState?.user?.username}</b></span>
        <span class="dot" aria-hidden="true"></span>
        <span class="accent">admin</span>
      </div>
    </header>

    {#if loadError}
      <p role="alert" class="err">{loadError}</p>
    {/if}

    <section class="section">
      <div class="section-eyebrow">
        <span>User management</span>
        <span class="rule" aria-hidden="true"></span>
        <span class="tag">{users.length} accounts</span>
      </div>
      <AdminToolbar
        filter={filter}
        query={query}
        count={filteredUsers.length}
        onFilter={(f) => (filter = f)}
        onQuery={(q) => (query = q)}
        onCreate={() => (overlay = 'create')}
      />
      <UserTable users={filteredUsers} currentUserId={currentUserId} onAction={onAction} />
    </section>

    <section class="section">
      <div class="section-eyebrow">
        <span>System status</span>
        <span class="rule" aria-hidden="true"></span>
      </div>
      <SysGrid status={status} nextPollAt={null} now={nowSec} />
      <div class="errors-head">
        <span>Recent events</span>
        <span class="rule" aria-hidden="true"></span>
      </div>
      <ErrorsTable events={status?.recent_errors ?? []} />
    </section>
  </main>
{/if}

{#if overlay === 'create'}
  <CreateUserDialog open={true} onSubmit={handleCreate} onClose={closeOverlay} />
{:else if overlay === 'resetConfirm' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Reset password for ${overlayUser.username}?`}
    body={`A new one-shot password will be generated and shown once. ${overlayUser.username}'s other sessions will be signed out.`}
    cta="Reset password"
    onConfirm={handleResetConfirm}
    onCancel={closeOverlay}
  />
{:else if overlay === 'resetResult' && overlayUser}
  <ResetPasswordResultDialog
    open={true}
    username={overlayUser.username}
    password={tempPassword}
    onClose={closeOverlay}
  />
{:else if overlay === 'disable2fa' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Disable two-factor for ${overlayUser.username}?`}
    body={`This removes the authenticator binding from ${overlayUser.username}'s account. They'll sign in with password only until they re-enrol. Recovery codes are invalidated immediately.`}
    cta="Disable TOTP"
    danger
    footNote="acts immediately"
    onConfirm={handleDisable2FA}
    onCancel={closeOverlay}
  />
{:else if overlay === 'disableUser' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Disable ${overlayUser.username}?`}
    body={`${overlayUser.username} will be signed out everywhere and can't sign back in until you re-enable the account. Their data and feeds are preserved.`}
    cta="Disable account"
    footNote="reversible"
    onConfirm={handleToggleDisabled}
    onCancel={closeOverlay}
  />
{:else if overlay === 'enable' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Re-enable ${overlayUser.username}?`}
    body={`${overlayUser.username} will be able to sign in again with their existing password. Their feeds and saved entries are unchanged.`}
    cta="Re-enable account"
    onConfirm={handleToggleDisabled}
    onCancel={closeOverlay}
  />
{:else if overlay === 'delete' && overlayUser}
  <ConfirmDialog
    open={true}
    title={`Delete ${overlayUser.username}?`}
    body={`This permanently removes ${overlayUser.username}, all their feeds, saved entries and sessions. This can't be undone.`}
    cta={`Delete ${overlayUser.username}`}
    danger
    footNote="permanent · cannot be undone"
    list={[
      `${overlayUser.passkey_count} passkey${overlayUser.passkey_count === 1 ? '' : 's'} revoked`,
      'subscribed feeds released',
      'read/save records purged',
    ]}
    onConfirm={handleDelete}
    onCancel={closeOverlay}
  />
{/if}

<style>
  .ts-shell { max-width: 1080px; margin: 0 auto; padding: 24px 32px; }
  .head { padding: 28px 0 18px; border-bottom: 1px solid var(--rule); }
  .eyebrow {
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.14em;
    text-transform: uppercase; color: var(--ink-3); margin-bottom: 8px;
  }
  .title {
    font-family: var(--serif); font-size: 34px; font-weight: 600;
    letter-spacing: -0.02em; line-height: 1.1; margin: 0 0 10px;
  }
  .id {
    font-family: var(--mono); font-size: 11px; color: var(--ink-3);
    display: inline-flex; align-items: center; gap: 8px;
  }
  .id b { color: var(--ink-2); font-weight: 500; }
  .id .dot { width: 3px; height: 3px; border-radius: 50%; background: var(--ink-4); }
  .id .accent { color: var(--accent); }
  .err {
    font-family: var(--mono); font-size: 12px;
    color: #c43a3a; margin: 12px 0;
  }
  :global(html.theme-dark) .err { color: #ec7a7a; }
  .section { padding: 36px 0 6px; }
  .section-eyebrow {
    display: flex; align-items: center; gap: 12px;
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.1em;
    text-transform: uppercase; color: var(--ink-3);
    margin-bottom: 14px;
  }
  .section-eyebrow .rule { flex: 1; height: 1px; background: var(--rule); }
  .section-eyebrow .tag { color: var(--ink-3); }
  .errors-head {
    display: flex; align-items: center; gap: 12px;
    margin-top: 22px; padding-bottom: 8px;
    font-family: var(--mono); font-size: 9.5px; letter-spacing: 0.1em;
    text-transform: uppercase; color: var(--ink-3);
  }
  .errors-head .rule { flex: 1; height: 1px; background: var(--rule); }
</style>
