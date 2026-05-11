<script lang="ts">
  type Props = { title: string; count?: number; countLabel?: string };
  let { title, count, countLabel }: Props = $props();
</script>

<header class="mhead">
  <a class="wordmark" href="/" aria-label="Tap home">tap<span class="dot" aria-hidden="true"></span></a>
  <h1 class="title">{title}</h1>
  {#if count !== undefined}
    <span class="count"><b>{count}</b>{#if countLabel}<span> {countLabel}</span>{/if}</span>
  {/if}
</header>

<style>
  .mhead {
    display: flex; align-items: baseline; gap: 12px;
    padding: 60px 18px 10px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
  }
  .wordmark {
    font-family: var(--sans);
    font-weight: 600; font-size: 18px;
    letter-spacing: var(--tr-display, -0.02em);
    color: var(--ink); text-decoration: none;
    display: inline-flex; align-items: baseline; gap: 1px;
  }
  .dot { width: 5px; height: 5px; border-radius: 50%; background: var(--accent); transform: translateY(-1px); margin-left: 1px; display: inline-block; }
  .title {
    font-family: var(--sans); font-size: 16px; font-weight: 600;
    color: var(--ink);
    margin: 0;
    flex: 1;
  }
  .count { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  .count b { color: var(--ink); font-weight: 600; }
</style>
