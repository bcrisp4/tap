<script lang="ts">
  type Props = {
    value: string;
    onChange: (v: string) => void;
    disabled?: boolean;
  };
  let { value, onChange, disabled = false }: Props = $props();

  let refs: (HTMLInputElement | null)[] = $state(Array.from({ length: 6 }, () => null));

  function setCell(i: number, ch: string) {
    const digits = value.padEnd(6, ' ').split('');
    digits[i] = ch || ' ';
    const next = digits.join('').replace(/ /g, '');
    onChange(next.slice(0, 6));
  }

  function onInput(i: number, e: Event) {
    const v = (e.currentTarget as HTMLInputElement).value;
    const ch = v.replace(/\D/g, '').slice(-1);
    setCell(i, ch);
    if (ch && i < 5) refs[i + 1]?.focus();
  }

  function onKey(i: number, e: KeyboardEvent) {
    if (e.key === 'Backspace' && !(e.currentTarget as HTMLInputElement).value && i > 0) {
      refs[i - 1]?.focus();
    } else if (e.key === 'ArrowLeft' && i > 0) refs[i - 1]?.focus();
    else if (e.key === 'ArrowRight' && i < 5) refs[i + 1]?.focus();
  }

  function onPaste(e: ClipboardEvent) {
    e.preventDefault();
    const text = (e.clipboardData?.getData('text') ?? '').replace(/\D/g, '').slice(0, 6);
    if (!text) return;
    onChange(text);
    refs[Math.min(text.length, 5)]?.focus();
  }
</script>

<div class="otp" role="group" aria-label="6-digit code">
  {#each Array(6) as _, i}
    <input
      bind:this={refs[i]}
      class="cell"
      inputmode="numeric"
      pattern="[0-9]*"
      maxlength="1"
      value={value[i] ?? ''}
      {disabled}
      oninput={(e) => onInput(i, e)}
      onkeydown={(e) => onKey(i, e)}
      onpaste={(e) => onPaste(e)}
    />
  {/each}
</div>

<style>
  .otp { display: inline-flex; gap: 6px; }
  .cell {
    width: 36px; height: 44px;
    border: 1px solid var(--rule);
    background: var(--bg);
    border-radius: 4px;
    text-align: center;
    font-family: var(--mono);
    font-size: 17px;
    color: var(--ink);
    font-weight: 500;
    font-variant-numeric: tabular-nums;
  }
  .cell:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-soft);
  }
</style>
