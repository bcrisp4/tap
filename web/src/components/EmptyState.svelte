<script lang="ts">
  import type { Snippet } from 'svelte';
  type Props = {
    title: string;
    subtitle?: string | Snippet;
    cta?: { label: string; onClick: () => void };
    dot?: 'accent' | 'ink-4';
  };
  let { title, subtitle, cta, dot = 'accent' }: Props = $props();
</script>

<div class="ts-empty">
  <span class="ts-empty-dot" data-tone={dot} aria-hidden="true"></span>
  <h2 class="ts-empty-title">{title}</h2>
  {#if typeof subtitle === 'string'}
    <p class="ts-empty-sub">{subtitle}</p>
  {:else if subtitle}
    <p class="ts-empty-sub">{@render subtitle()}</p>
  {/if}
  {#if cta}
    <button type="button" class="ts-btn is-primary" onclick={cta.onClick}>{cta.label}</button>
  {/if}
</div>

<style>
  .ts-empty { padding: 80px 24px; text-align: center; color: var(--ink-3); }
  .ts-empty-dot {
    display: inline-block;
    width: 8px; height: 8px; border-radius: 50%;
    background: var(--accent);
    margin: 0 auto 18px;
  }
  .ts-empty-dot[data-tone="ink-4"] { background: var(--ink-4); }
  .ts-empty-title {
    font-family: var(--serif); font-size: 20px; font-weight: 500;
    color: var(--ink-2);
    margin: 0 0 8px;
    letter-spacing: -0.01em;
  }
  .ts-empty-sub {
    font-family: var(--sans); font-size: 13px; color: var(--ink-3);
    max-width: 360px; margin: 0 auto;
    line-height: 1.5;
  }
  .ts-empty .ts-btn { margin-top: 18px; }
</style>
