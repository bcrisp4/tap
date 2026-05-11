<script lang="ts">
  import Sidebar from '../components/Sidebar.svelte';
  import Security from './settings/Security.svelte';
  import { theme, font, density } from '../lib/preferences.svelte';

  type Section = 'appearance' | 'security';
  let activeSection = $state<Section>('appearance');
</script>

<div class="layout">
  <Sidebar />
  <main class="settings-main">
    <header class="settings-header">
      <h1 class="settings-title">Settings</h1>
    </header>
    <div class="settings-body">
      <nav class="settings-nav" aria-label="Settings sections">
        <button
          class="settings-nav-item"
          class:active={activeSection === 'appearance'}
          aria-current={activeSection === 'appearance' ? 'true' : undefined}
          onclick={() => { activeSection = 'appearance'; }}
        >Appearance</button>
        <button
          class="settings-nav-item"
          class:active={activeSection === 'security'}
          aria-current={activeSection === 'security' ? 'true' : undefined}
          onclick={() => { activeSection = 'security'; }}
        >Security</button>
      </nav>
      <div class="settings-content">
        {#if activeSection === 'appearance'}
          <section aria-labelledby="appearance-heading">
            <h2 id="appearance-heading" class="section-heading">Appearance</h2>
            <div class="pref-group">
              <label class="pref-label" for="theme-select">Theme</label>
              <select id="theme-select" class="pref-select" bind:value={theme.stored}>
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
                <option value="sepia">Sepia</option>
              </select>
            </div>
            <div class="pref-group">
              <label class="pref-label" for="font-select">Reading font</label>
              <select id="font-select" class="pref-select" bind:value={font.value}>
                <option value="serif">Serif</option>
                <option value="sans">Sans-serif</option>
              </select>
            </div>
            <div class="pref-group">
              <label class="pref-label" for="density-select">Density</label>
              <select id="density-select" class="pref-select" bind:value={density.value}>
                <option value="compact">Compact</option>
                <option value="default">Default</option>
                <option value="comfortable">Comfortable</option>
              </select>
            </div>
          </section>
        {:else}
          <section aria-labelledby="security-heading">
            <Security />
          </section>
        {/if}
      </div>
    </div>
  </main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .settings-main { flex: 1; display: flex; flex-direction: column; background: var(--bg); overflow-y: auto; }
  .settings-header { padding: 14px 24px; border-bottom: 1px solid var(--rule); background: var(--bg); position: sticky; top: 0; }
  .settings-title { font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink); margin: 0; }
  .settings-body { display: flex; flex: 1; }
  .settings-nav { width: 180px; flex-shrink: 0; padding: 16px 0; border-right: 1px solid var(--rule); }
  .settings-nav-item {
    display: block; width: 100%; text-align: left; padding: 8px 20px;
    font-family: var(--sans); font-size: 13px; color: var(--ink-2);
    border-left: 2px solid transparent;
  }
  .settings-nav-item:hover { color: var(--ink); }
  .settings-nav-item.active { color: var(--ink); border-left-color: var(--accent); font-weight: 500; }
  .settings-content { flex: 1; padding: 24px 32px; max-width: 560px; }
  .section-heading {
    font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink);
    margin: 0 0 20px; padding-bottom: 10px; border-bottom: 1px solid var(--rule);
  }
  .pref-group { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
  .pref-label { font-family: var(--sans); font-size: 13px; color: var(--ink-2); }
  .pref-select {
    font-family: var(--sans); font-size: 12px; color: var(--ink);
    background: var(--bg-soft); border: 1px solid var(--rule);
    border-radius: 4px; padding: 4px 8px; cursor: pointer;
  }
  :global(.session-table) { width: 100%; border-collapse: collapse; font-family: var(--sans); font-size: 12px; }
  :global(.session-table td) { padding: 8px 0; border-bottom: 1px solid var(--rule); color: var(--ink-2); }
  :global(.session-badge) { font-family: var(--mono); font-size: 10px; color: var(--accent); }
  :global(.session-revoke) { font-family: var(--mono); font-size: 10px; color: var(--accent); letter-spacing: 0.04em; }
  :global(.danger-action) { color: #c97a1a; font-family: var(--mono); font-size: 10px; letter-spacing: 0.04em; }
  :global(.role-badge) {
    font-family: var(--mono); font-size: 10px;
    border: 1px solid var(--rule); border-radius: 2px; padding: 1px 5px;
    color: var(--ink-3);
  }
</style>
