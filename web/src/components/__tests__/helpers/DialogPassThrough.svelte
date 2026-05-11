<script lang="ts">
  type Props = {
    open?: boolean;
    title?: string;
    onClose?: () => void;
    foot?: import('svelte').Snippet;
    children: import('svelte').Snippet;
  };
  let { open = true, title, foot, children }: Props = $props();
</script>

{#if open !== false}
  <div class="dialog-mock">
    {#if title}<div class="dialog-mock-title">{title}</div>{/if}
    <div class="dialog-mock-body">{@render children()}</div>
    {#if foot}<div class="dialog-mock-foot">{@render foot()}</div>{/if}
  </div>
{/if}
