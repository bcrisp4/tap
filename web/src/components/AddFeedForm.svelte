<script lang="ts">
  import { subscriptions } from '../lib/store';

  let url = $state('');
  let busy = $state(false);
  let error = $state<string | null>(null);

  async function submit(ev: Event) {
    ev.preventDefault();
    if (!url.trim()) return;
    busy = true;
    error = null;
    try {
      await subscriptions.add(url.trim());
      url = '';
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit}>
  <input
    type="url"
    placeholder="Paste a feed URL"
    bind:value={url}
    disabled={busy}
    required
  />
  <button type="submit" disabled={busy || !url.trim()}>Add</button>
  {#if error}
    <p class="error">{error}</p>
  {/if}
</form>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 20px 12px;
  }
  input {
    font-family: var(--sans);
    font-size: 12px;
    padding: 6px 8px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--surface);
  }
  button {
    align-self: flex-start;
    font-family: var(--sans);
    font-size: 12px;
    padding: 6px 10px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--surface);
  }
  .error {
    margin: 0;
    font-family: var(--mono);
    font-size: 10px;
    color: #b14;
  }
</style>
