<script lang="ts">
  type Code = { code: string; used?: boolean };
  type Props = { codes: Code[] };
  let { codes }: Props = $props();
</script>

<div class="codes">
  {#each codes as c, i (c.code)}
    <span class="code" class:is-used={c.used}>
      <span class="n">{i + 1}</span>
      <span>{c.code}</span>
    </span>
  {/each}
</div>

<style>
  .codes {
    display: grid; grid-template-columns: 1fr 1fr; gap: 6px 18px;
    padding: 14px 18px;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: var(--radius-input, 4px);
  }
  .code {
    font-family: var(--mono); font-size: 13px; color: var(--ink);
    letter-spacing: 0.06em; padding: 4px 0;
    display: flex; align-items: center; gap: 10px;
  }
  .code .n { color: var(--ink-4); font-size: 10px; width: 14px; text-align: right; }
  .code.is-used { color: var(--ink-4); text-decoration: line-through; }
</style>
