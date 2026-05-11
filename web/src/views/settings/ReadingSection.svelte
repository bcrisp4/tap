<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import { reading } from '../../lib/preferences.svelte';
</script>

<SetSection num="02" title="Reading">
  <SetRow
    label="Mark read on scroll"
    desc="Auto-marks an entry as read 1.5s after its lede leaves the viewport."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.markOnScroll} aria-label="Mark read on scroll" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
  <SetRow
    label="Auto-open next"
    desc="When you mark an entry read, jump to the next unread automatically."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.autoOpenNext} aria-label="Auto-open next" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
  <SetRow
    label="Show summaries in list"
    desc="Renders the 1–3 sentence excerpt under each entry title. Compact density overrides this."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.showSummaries} aria-label="Show summaries in list" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
  <SetRow
    label="Open links in new tab"
    desc="Adds target=_blank and rel=noopener to outbound links inside the reader."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.openLinksNewTab} aria-label="Open links in new tab" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
</SetSection>

<style>
  .toggle {
    position: relative; display: inline-block;
    width: 36px; height: 20px;
  }
  .toggle input {
    position: absolute; opacity: 0; width: 100%; height: 100%; cursor: pointer;
  }
  .slider {
    position: absolute; inset: 0;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: 999px;
    transition: background 100ms ease;
  }
  .slider::before {
    content: ""; position: absolute;
    width: 14px; height: 14px;
    left: 2px; top: 2px;
    background: var(--bg);
    border-radius: 50%;
    box-shadow: 0 1px 2px rgba(0,0,0,0.15);
    transition: transform 120ms ease;
  }
  .toggle input:checked + .slider { background: var(--accent); border-color: var(--accent); }
  .toggle input:checked + .slider::before { transform: translateX(16px); }
  .toggle input:focus-visible + .slider {
    outline: 2px solid var(--accent); outline-offset: 1px;
  }
</style>
