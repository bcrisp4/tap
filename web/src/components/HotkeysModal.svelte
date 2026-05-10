<script lang="ts">
  type Props = { open: boolean; onClose: () => void };
  let { open, onClose }: Props = $props();

  let dialog = $state<HTMLDialogElement | null>(null);

  $effect(() => {
    if (!dialog) return;
    if (open) {
      dialog.showModal();
    } else {
      dialog.close();
    }
  });
</script>

<dialog
  bind:this={dialog}
  class="tap-modal"
  aria-labelledby="hotkeys-title"
  onclose={onClose}
>
  <div class="tap-modal-head">
    <span id="hotkeys-title" class="modal-title">Keyboard shortcuts</span>
    <button class="tap-modal-close" onclick={onClose} aria-label="Close">✕</button>
  </div>
  <div class="tap-modal-body">
    <div>
      <div class="shortcut-group-title">Navigation</div>
      <div class="shortcut-row">
        <span class="shortcut-desc">Next entry</span>
        <span class="shortcut-keys"><kbd class="kbd">j</kbd><span class="shortcut-plus">/</span><kbd class="kbd">↓</kbd></span>
      </div>
      <div class="shortcut-row">
        <span class="shortcut-desc">Previous entry</span>
        <span class="shortcut-keys"><kbd class="kbd">k</kbd><span class="shortcut-plus">/</span><kbd class="kbd">↑</kbd></span>
      </div>
      <div class="shortcut-row">
        <span class="shortcut-desc">Open entry</span>
        <span class="shortcut-keys"><kbd class="kbd">o</kbd><span class="shortcut-plus">/</span><kbd class="kbd">↵</kbd></span>
      </div>
      <div class="shortcut-row">
        <span class="shortcut-desc">Back to list / close</span>
        <span class="shortcut-keys"><kbd class="kbd">Esc</kbd></span>
      </div>
    </div>
    <div>
      <div class="shortcut-group-title">Actions</div>
      <div class="shortcut-row">
        <span class="shortcut-desc">Toggle read</span>
        <span class="shortcut-keys"><kbd class="kbd">m</kbd></span>
      </div>
      <div class="shortcut-row">
        <span class="shortcut-desc">Toggle saved</span>
        <span class="shortcut-keys"><kbd class="kbd">s</kbd></span>
      </div>
      <div class="shortcut-row">
        <span class="shortcut-desc">View original</span>
        <span class="shortcut-keys"><kbd class="kbd">v</kbd></span>
      </div>
      <div class="shortcut-row">
        <span class="shortcut-desc">This modal</span>
        <span class="shortcut-keys"><kbd class="kbd">?</kbd></span>
      </div>
    </div>
  </div>
</dialog>

<style>
  .modal-title { font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink); }
  dialog::backdrop { background: rgba(0,0,0,0.32); backdrop-filter: blur(2px); }
</style>
