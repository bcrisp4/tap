<script lang="ts">
  interface Props {
    num: string;
    title: string;
    tag?: string;
    children: import('svelte').Snippet;
  }
  let { num, title, tag, children }: Props = $props();
</script>

<section class="set-section">
  <div class="set-section-eyebrow">
    <span class="num">{num}</span>
    <span>{title}</span>
    <span class="rule" aria-hidden="true"></span>
    {#if tag}<span class="tag">{tag}</span>{/if}
  </div>
  {#if children}{@render children()}{/if}
</section>

<style>
  .set-section { padding: 36px 0 6px; }
  .set-section-eyebrow {
    display: flex; align-items: center; gap: 12px;
    padding: 4px 0 18px;
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.16em;
    text-transform: uppercase;
    color: var(--ink-3);
  }
  .set-section-eyebrow .num { color: var(--ink-4); letter-spacing: 0; }
  .set-section-eyebrow .rule { flex: 1; height: 1px; background: var(--rule); }
  .set-section-eyebrow .tag {
    font-size: 9.5px; color: var(--accent); letter-spacing: 0.08em;
  }
</style>
