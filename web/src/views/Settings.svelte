<script lang="ts">
  import Segmented from '../components/Segmented.svelte';
  import Security from './settings/Security.svelte';
  import { theme, font, density, measure } from '../lib/preferences.svelte';

  type Section = 'appearance' | 'security';
  let activeSection = $state<Section>('appearance');
</script>

<div class="settings-wrap">
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
            <span class="pref-label">Theme</span>
            <Segmented
              options={[
                { value: 'light', label: 'Light' },
                { value: 'dark', label: 'Dark' },
                { value: 'sepia', label: 'Sepia' },
                { value: 'system', label: 'System' },
              ]}
              value={theme.stored}
              onChange={(v) => theme.stored = v}
              ariaLabel="Theme"
            />
          </div>
          <div class="pref-group">
            <span class="pref-label">Reading font</span>
            <Segmented
              options={[
                { value: 'serif', label: 'Serif' },
                { value: 'sans', label: 'Sans' },
              ]}
              value={font.value}
              onChange={(v) => font.value = v}
              ariaLabel="Font"
            />
          </div>
          <div class="pref-group">
            <span class="pref-label">Density</span>
            <Segmented
              options={[
                { value: 'compact', label: 'Compact' },
                { value: 'comfortable', label: 'Comfortable' },
                { value: 'cosy', label: 'Cosy' },
              ]}
              value={density.value}
              onChange={(v) => density.value = v}
              ariaLabel="Density"
            />
          </div>
          <div class="pref-group">
            <span class="pref-label">Article width</span>
            <Segmented
              options={[
                { value: 'narrow', label: 'Narrow' },
                { value: 'comfortable', label: 'Comfortable' },
                { value: 'wide', label: 'Wide' },
              ]}
              value={measure.value}
              onChange={(v) => measure.value = v}
              ariaLabel="Article width"
            />
          </div>
        </section>
      {:else}
        <section aria-labelledby="security-heading">
          <Security />
        </section>
      {/if}
    </div>
  </div>
</div>

<style>
  .settings-wrap { display: flex; flex-direction: column; }
  .settings-header { padding: 14px 0 20px; border-bottom: 1px solid var(--rule); }
  .settings-title { font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink); margin: 0; }
  .settings-body { display: flex; flex: 1; padding-top: 16px; }
  .settings-nav { width: 180px; flex-shrink: 0; padding: 0; border-right: 1px solid var(--rule); }
  .settings-nav-item {
    display: block; width: 100%; text-align: left; padding: 8px 20px;
    font-family: var(--sans); font-size: 13px; color: var(--ink-2);
    border-left: 2px solid transparent;
  }
  .settings-nav-item:hover { color: var(--ink); }
  .settings-nav-item.active { color: var(--ink); border-left-color: var(--accent); font-weight: 500; }
  .settings-content { flex: 1; padding: 0 0 0 32px; max-width: 560px; }
  .section-heading {
    font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink);
    margin: 0 0 20px; padding-bottom: 10px; border-bottom: 1px solid var(--rule);
  }
  .pref-group { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; gap: 16px; }
  .pref-label { font-family: var(--sans); font-size: 13px; color: var(--ink-2); flex-shrink: 0; }
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
