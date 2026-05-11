<script lang="ts">
  import Sidebar from '../components/Sidebar.svelte';
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { auth } from '../lib/auth';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import type { AdminUser } from '../lib/types';

  let authState = $derived(get(auth));

  let users = $state<AdminUser[]>([]);
  let error = $state('');
  let busy = $state(false);

  // Create user form
  let showCreateForm = $state(false);
  let newUsername = $state('');
  let newPassword = $state('');
  let newRole = $state<'admin' | 'user'>('user');

  // Temp password modal
  let tempPassword = $state('');

  onMount(async () => {
    if (authState?.user?.role !== 'admin') {
      navigate('/');
      return;
    }
    await loadUsers();
  });

  async function loadUsers() {
    try {
      users = await api.listUsers();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load users';
    }
  }

  async function createUser() {
    busy = true;
    error = '';
    try {
      const u = await api.createUser(newUsername, newPassword, newRole);
      users = [...users, u];
      showCreateForm = false;
      newUsername = '';
      newPassword = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to create user';
    } finally {
      busy = false;
    }
  }

  async function resetPassword(id: number) {
    busy = true;
    error = '';
    try {
      const result = await api.resetUserPassword(id);
      tempPassword = result.temporary_password;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to reset password';
    } finally {
      busy = false;
    }
  }

  async function disableTOTP(id: number) {
    if (!confirm('Disable 2FA for this user?')) return;
    busy = true;
    error = '';
    try {
      await api.disableUserTOTP(id);
      await loadUsers();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to disable TOTP';
    } finally {
      busy = false;
    }
  }

  async function toggleDisabled(user: AdminUser) {
    busy = true;
    error = '';
    try {
      const updated = await api.patchUser(user.id, { disabled: !user.disabled_at });
      users = users.map(u => u.id === updated.id ? updated : u);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to update user';
    } finally {
      busy = false;
    }
  }

  async function deleteUser(id: number, username: string) {
    if (!confirm(`Delete user '${username}'? This cannot be undone.`)) return;
    busy = true;
    error = '';
    try {
      await api.deleteUser(id);
      users = users.filter(u => u.id !== id);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to delete user';
    } finally {
      busy = false;
    }
  }

  function formatDate(ts: number | null): string {
    if (!ts) return '—';
    return new Date(ts * 1000).toLocaleDateString();
  }
</script>

{#if authState?.user?.role !== 'admin'}
  <p>Access denied.</p>
{:else}
  <div class="layout">
    <Sidebar />
    <main class="admin-main">
      <div class="admin">
        <h1>User Management</h1>
        {#if error}
          <p role="alert" class="error">{error}</p>
        {/if}

        {#if tempPassword}
          <div class="modal">
            <p>Temporary password (shown once):</p>
            <code>{tempPassword}</code>
            <button onclick={() => tempPassword = ''}>Close</button>
          </div>
        {/if}

        <button onclick={() => showCreateForm = !showCreateForm} disabled={busy}>
          {showCreateForm ? 'Cancel' : 'Create user'}
        </button>

        {#if showCreateForm}
          <form onsubmit={(e) => { e.preventDefault(); createUser(); }} class="create-form">
            <input type="text" bind:value={newUsername} placeholder="Username" required />
            <input type="password" bind:value={newPassword} placeholder="Password" required minlength="8" />
            <select bind:value={newRole}>
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
            <button type="submit" disabled={busy}>Create</button>
          </form>
        {/if}

        <table>
          <thead>
            <tr>
              <th>Username</th>
              <th>Role</th>
              <th>Created</th>
              <th>Status</th>
              <th>2FA</th>
              <th>Passkeys</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each users as user (user.id)}
              <tr class:disabled={!!user.disabled_at}>
                <td>{user.username}</td>
                <td>{user.role}</td>
                <td>{formatDate(user.created_at)}</td>
                <td>{user.disabled_at ? 'Disabled' : 'Active'}</td>
                <td>{user.has_totp ? 'Enabled' : '—'}</td>
                <td>{user.passkey_count}</td>
                <td class="actions">
                  <button onclick={() => resetPassword(user.id)} disabled={busy}>Reset password</button>
                  {#if user.has_totp}
                    <button onclick={() => disableTOTP(user.id)} disabled={busy}>Disable 2FA</button>
                  {/if}
                  <button onclick={() => toggleDisabled(user)} disabled={busy}>
                    {user.disabled_at ? 'Re-enable' : 'Disable'}
                  </button>
                  {#if user.id !== authState?.user?.id}
                    <button onclick={() => deleteUser(user.id, user.username)} disabled={busy} class="danger">
                      Delete
                    </button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </main>
  </div>
{/if}

<style>
  .layout { display: flex; height: 100vh; }
  .admin-main { flex: 1; overflow-y: auto; background: var(--bg); padding: 24px 32px; }
  .admin { max-width: 64rem; margin: 0 auto; }
  table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
  th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #eee; }
  .disabled td { opacity: 0.6; }
  .error { color: var(--color-danger, #b00); }
  .modal { border: 1px solid #ccc; padding: 1rem; margin: 1rem 0; }
  .modal code { display: block; font-size: 1.2rem; margin: 0.5rem 0; }
  .create-form { display: flex; gap: 0.5rem; margin: 0.5rem 0; flex-wrap: wrap; }
  .actions { display: flex; gap: 0.25rem; flex-wrap: wrap; }
  .danger { color: var(--color-danger, #b00); }
</style>
