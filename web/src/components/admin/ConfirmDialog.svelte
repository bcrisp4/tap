<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import Button from '../Button.svelte';

  type Props = {
    open: boolean;
    title: string;
    body: string;
    cta: string;
    danger?: boolean;
    list?: string[];
    footNote?: string;
    onConfirm?: () => void;
    onCancel?: () => void;
  };
  const { open, title, body, cta, danger = false, list = [], footNote = '', onConfirm, onCancel }: Props = $props();
</script>

<Dialog {open} {title} onClose={onCancel}>
  <div class="body">
    <p>{body}</p>
    {#if list.length > 0}
      <ul class="list">
        {#each list as item}
          <li>{item}</li>
        {/each}
      </ul>
    {/if}
  </div>
  {#snippet foot()}
    {#if footNote}<div class="foot-l">{footNote}</div>{/if}
    <Button variant="quiet" onclick={onCancel}>Cancel</Button>
    <button
      type="button"
      class="cta-btn"
      class:danger={danger}
      data-variant={danger ? 'danger' : 'primary'}
      onclick={onConfirm}
    >{cta}</button>
  {/snippet}
</Dialog>

<style>
  .body { display: flex; flex-direction: column; gap: 12px; }
  p { font-family: var(--sans); font-size: 13.5px; color: var(--ink-2); margin: 0; line-height: 1.55; }
  .list {
    margin: 0; padding: 10px 14px;
    list-style: none;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg-soft);
    font-family: var(--mono); font-size: 11.5px; color: var(--ink-2);
  }
  .list li { padding: 4px 0; }
  .foot-l { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-right: auto; }
  .cta-btn {
    display: inline-flex; align-items: center; gap: 7px;
    font-family: var(--sans); font-size: 12.5px; font-weight: 500;
    padding: 8px 14px;
    border-radius: var(--radius-input, 4px);
    border: 1px solid var(--ink);
    background: var(--ink);
    color: var(--bg);
    cursor: pointer;
    white-space: nowrap;
  }
  .cta-btn:hover { opacity: 0.88; }
  .cta-btn.danger {
    color: var(--color-error-light, #c43a3a);
    border-color: rgba(196, 58, 58, 0.4);
    background: transparent;
  }
  :global(html.theme-dark) .cta-btn.danger { color: var(--color-error-dark, #ec7a7a); border-color: rgba(236, 122, 122, 0.4); }
  .cta-btn.danger:hover { background: rgba(196, 58, 58, 0.06); }
</style>
