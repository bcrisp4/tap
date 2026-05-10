<script lang="ts">
  import { auth } from '../lib/auth';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await auth.login(username, password);
    } catch (err) {
      error = err instanceof Error && err.message !== 'unauthorized'
        ? err.message
        : 'Invalid username or password.';
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit}>
  <h1>Sign in to Tap</h1>
  <label>
    Username
    <input
      type="text"
      autocomplete="username"
      bind:value={username}
      disabled={busy}
      required
    />
  </label>
  <label>
    Password
    <input
      type="password"
      autocomplete="current-password"
      bind:value={password}
      disabled={busy}
      required
    />
  </label>
  {#if error}
    <p role="alert" class="error">{error}</p>
  {/if}
  <button type="submit" disabled={busy}>
    {busy ? 'Signing in…' : 'Sign in'}
  </button>
</form>

<style>
  form {
    max-width: 22rem;
    margin: 4rem auto;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  h1 {
    text-align: center;
    margin-bottom: 0.5rem;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  input {
    padding: 0.5rem;
  }
  .error {
    color: var(--color-danger, #b00);
  }
</style>
