<script lang="ts">
  type Props = {
    size?: number;
    filled?: boolean;
    color?: string;
  };
  let { size = 6, filled = true, color = 'var(--accent)' }: Props = $props();
</script>

<span
  class="dot"
  style:--size={`${size}px`}
  style:--c={color}
  class:filled
  aria-hidden="true"
></span>

<style>
  .dot {
    display: inline-block;
    width: var(--size);
    height: var(--size);
    border-radius: 50%;
    background: transparent;
    border: 1px solid var(--ink-4);
  }
  .dot.filled {
    background: var(--c);
    border-color: var(--c);
  }
</style>
