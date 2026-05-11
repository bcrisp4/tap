<script lang="ts">
  type Props = {
    open: boolean;
    onClose: () => void;
    anchor?: any;
    children: import('svelte').Snippet;
  };
  let { open, children }: Props = $props();
</script>

{#if open}
  <div>{@render children()}</div>
{/if}
