<script lang="ts">
  type Props = { open: boolean; onClose: () => void };
  let { open, onClose }: Props = $props();
  let dialog = $state<HTMLDialogElement | null>(null);

  $effect(() => {
    if (!dialog) return;
    if (open) dialog.showModal();
    else dialog.close();
  });

  const groups = [
    {
      title: 'Navigation',
      rows: [
        { keys: ['j'], desc: 'Next entry' },
        { keys: ['k'], desc: 'Previous entry' },
        { keys: ['o', '↵'], desc: 'Open entry' },
        { keys: ['Esc'], desc: 'Back to list' },
      ],
    },
    {
      title: 'Actions',
      rows: [
        { keys: ['m'], desc: 'Toggle read' },
        { keys: ['s'], desc: 'Toggle saved' },
        { keys: ['v'], desc: 'View original' },
        { keys: ['r'], desc: 'Refresh' },
      ],
    },
    {
      title: 'App',
      rows: [
        { keys: ['/'], desc: 'Search' },
        { keys: ['?'], desc: 'This help' },
      ],
    },
  ];
</script>

<dialog bind:this={dialog} class="modal" aria-labelledby="hk-title" onclose={onClose}>
  <div class="head">
    <span id="hk-title" class="eyebrow">Keyboard shortcuts</span>
    <button class="close" type="button" onclick={onClose} aria-label="Close">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9"/></svg>
    </button>
  </div>
  <div class="body">
    {#each groups as g (g.title)}
      <div class="group">
        <div class="group-title">{g.title}</div>
        {#each g.rows as r, i (i)}
          <div class="row">
            <span class="keys">
              {#each r.keys as k, ki (ki)}
                {#if ki > 0}<span class="plus">then</span>{/if}
                <kbd class="kbd">{k}</kbd>
              {/each}
            </span>
            <span class="desc">{r.desc}</span>
          </div>
        {/each}
      </div>
    {/each}
  </div>
</dialog>

<style>
  .modal {
    background: var(--bg); color: var(--ink);
    border: 1px solid var(--rule);
    border-radius: var(--radius-dialog, 6px);
    width: min(560px, 100%);
    max-height: 80%;
    box-shadow: 0 20px 60px rgba(0,0,0,0.25);
    padding: 0;
  }
  .modal::backdrop { background: rgba(0,0,0,0.32); backdrop-filter: blur(2px); }
  .head { display: flex; align-items: center; justify-content: space-between; padding: 14px 18px; border-bottom: 1px solid var(--rule); }
  .eyebrow { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); }
  .close { width: 26px; height: 26px; border: 0; background: transparent; color: var(--ink-3); cursor: pointer; border-radius: 4px; display: inline-flex; align-items: center; justify-content: center; }
  .close:hover { background: rgba(0,0,0,0.05); color: var(--ink); }
  .body { padding: 18px 22px 22px; display: grid; grid-template-columns: 1fr 1fr; gap: 22px 32px; }
  .group-title { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); margin-bottom: 8px; }
  .row { display: flex; align-items: center; justify-content: space-between; padding: 5px 0; font-size: 13px; }
  .keys { display: inline-flex; align-items: center; gap: 6px; }
  .plus { font-family: var(--mono); font-size: 10px; color: var(--ink-3); }
  .desc { color: var(--ink-2); }
  .kbd { font-family: var(--mono); font-size: 11px; padding: 2px 6px; border: 1px solid var(--rule); border-bottom-width: 2px; border-radius: 4px; background: var(--bg-soft); color: var(--ink-2); min-width: 16px; text-align: center; }
</style>
