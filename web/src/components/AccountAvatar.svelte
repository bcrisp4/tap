<script lang="ts">
  import AccountMenu from './AccountMenu.svelte';
  import { auth } from '../lib/auth';

  let open = $state(false);
  const initial = $derived(($auth.user?.username ?? '?')[0].toUpperCase());

  async function logout() {
    await auth.logout();
    window.location.assign('/');
  }
</script>

{#if $auth.user}
  <button
    type="button"
    class="avatar-btn"
    class:is-open={open}
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label="Account: {$auth.user.username}"
    title={$auth.user.username}
    onclick={() => open = true}
  >
    <span class="avatar" aria-hidden="true">{initial}</span>
  </button>
  <AccountMenu open={open} user={$auth.user} onClose={() => open = false} onLogout={logout} />
{/if}

<style>
  .avatar-btn {
    position: absolute;
    top: 30px; right: 22px;
    z-index: 6;
    display: inline-flex; align-items: center; justify-content: center;
    background: transparent; border: 0;
    padding: 3px;
    border-radius: 999px;
    cursor: pointer;
    transition: background var(--dur-base, 120ms) ease;
  }
  .avatar-btn:hover { background: rgba(0,0,0,0.06); }
  :global(html.theme-dark) .avatar-btn:hover { background: rgba(255,255,255,0.07); }
  .avatar-btn.is-open { background: rgba(0,0,0,0.08); }
  :global(html.theme-dark) .avatar-btn.is-open { background: rgba(255,255,255,0.10); }
  .avatar {
    width: 20px; height: 20px;
    border-radius: 999px;
    background: var(--ink-4);
    color: var(--ink);
    display: inline-flex; align-items: center; justify-content: center;
    font-family: var(--mono);
    font-size: 9.5px; font-weight: 600;
    line-height: 1;
  }
  :global(html.theme-dark) .avatar { background: rgba(255,255,255,0.14); color: var(--ink); }
</style>
