<script lang="ts">
  type Props = {
    open?: boolean;
    category?: { id: number; name: string; unread: number };
    isUncategorised?: boolean;
    onMarkRead?: () => void;
    onRename?: (name: string) => void;
    onDelete?: () => void;
    onReorderUp?: () => void;
    onReorderDown?: () => void;
    [key: string]: any;
  };
  let { category, isUncategorised = false, onMarkRead, onRename, onDelete, onReorderUp, onReorderDown }: Props = $props();
</script>

{#if category}
<section class="ts-cat" class:is-uncat={isUncategorised} tabindex="0">
  <h2 class="ts-cat-title">{category?.name ?? ''}</h2>
  {#if !isUncategorised}
    <button class="ts-cat-action" aria-label="Reorder up" onclick={onReorderUp}>Reorder up</button>
    <button class="ts-cat-action" aria-label="Reorder down" onclick={onReorderDown}>Reorder down</button>
    <button class="ts-cat-action is-danger" onclick={onDelete}>Delete</button>
    <button class="ts-cat-action" onclick={() => onRename?.('renamed')}>Rename</button>
  {/if}
  <button class="ts-cat-action" aria-label="Mark {category?.unread ?? 0} read" onclick={onMarkRead}>
    Mark {category?.unread ?? 0} read
  </button>
</section>
{/if}
