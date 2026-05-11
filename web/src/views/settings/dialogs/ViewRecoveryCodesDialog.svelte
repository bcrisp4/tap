<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import RecoveryCodesGrid from '../../../components/RecoveryCodesGrid.svelte';

  interface Props {
    codes: string[];
    regenerated?: boolean;
    onClose: () => void;
  }
  let { codes, regenerated = false, onClose }: Props = $props();

  function downloadTxt() {
    const text = codes.map((c, i) => `${String(i + 1).padStart(2, '0')}. ${c}`).join('\n');
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'tap-recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  }
</script>

<Dialog open title={regenerated ? 'New recovery codes' : 'Your recovery codes'} {onClose}>
  {#snippet children()}
    <p class="p">Each code is single-use. Store them in a password manager — they replace your authenticator if you lose access.</p>
    <RecoveryCodesGrid codes={codes.map(c => ({ code: c }))} />
    {#if regenerated}
      <div class="warn">The previous set of codes was just invalidated. Anything saved before now will no longer work.</div>
    {/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">{codes.length} codes · plain text</div>
    <Button onclick={downloadTxt}>Download .txt</Button>
    <Button variant="primary" onclick={onClose}>I've saved them</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .warn { margin-top: 14px; padding: 10px 12px; border-left: 2px solid var(--accent); background: var(--accent-soft); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
  .foot-l { margin-right: auto; font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); align-self: center; }
</style>
