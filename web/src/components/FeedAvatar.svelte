<script lang="ts">
  import { colorForFeed } from '../lib/colors';

  type Props = {
    feedURL: string;
    size?: number;
    radius?: number;
  };
  let { feedURL, size = 14, radius = 3 }: Props = $props();
  const color = $derived(colorForFeed(feedURL));
</script>

<span
  class="avatar"
  style:--size={`${size}px`}
  style:--r={`${radius}px`}
  style:background={color}
  aria-hidden="true"
></span>

<style>
  .avatar {
    display: inline-block;
    width: var(--size);
    height: var(--size);
    border-radius: var(--r);
    flex: none;
  }
</style>
