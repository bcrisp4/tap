<script lang="ts">
  type Props = { children: import('svelte').Snippet };
  let { children }: Props = $props();
</script>

<kbd class="kbd">{@render children()}</kbd>

<style>
  .kbd {
    display: inline-block;
    font-family: var(--mono);
    font-size: var(--fs-kbd, 10.5px);
    padding: 1px 5px;
    border: 1px solid var(--rule);
    border-bottom-width: 2px;
    border-radius: 3px;
    background: var(--surface);
    color: var(--ink-2);
    line-height: 1.2;
    vertical-align: 1px;
  }
</style>
