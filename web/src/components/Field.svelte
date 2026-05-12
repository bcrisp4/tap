<script lang="ts">
  type Props = {
    label: string;
    value: string;
    type?: 'text' | 'email' | 'password' | 'number';
    placeholder?: string;
    mono?: boolean;
    description?: string;
    trailing?: import('svelte').Snippet;
    autocomplete?: AutoFill;
    autofocus?: boolean;
    required?: boolean;
    disabled?: boolean;
    inputmode?: 'none' | 'text' | 'decimal' | 'numeric' | 'tel' | 'search' | 'email' | 'url';
    pattern?: string;
    maxlength?: number;
    name?: string;
    onInput?: (v: string) => void;
  };
  let {
    label, value = $bindable(''), type = 'text', placeholder, mono = false,
    description, trailing, autocomplete, autofocus = false, required = false,
    disabled = false, inputmode, pattern, maxlength, name, onInput,
  }: Props = $props();

  let inputEl = $state<HTMLInputElement | null>(null);
  $effect(() => { if (autofocus && inputEl) inputEl.focus(); });
</script>

<label class="field" class:is-mono={mono}>
  <span class="field-label">
    <span>{label}</span>
    {#if trailing}<span class="trailing">{@render trailing()}</span>{/if}
  </span>
  <input
    {type}
    {placeholder}
    {autocomplete}
    {required}
    {disabled}
    {inputmode}
    {pattern}
    {maxlength}
    {name}
    class="input"
    bind:this={inputEl}
    bind:value
    oninput={(e) => onInput?.((e.currentTarget as HTMLInputElement).value)}
  />
  {#if description}<span class="desc">{description}</span>{/if}
</label>

<style>
  .field { display: flex; flex-direction: column; gap: 8px; }
  .field-label {
    display: flex; justify-content: space-between; align-items: baseline;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.12em; text-transform: uppercase;
    color: var(--ink-3);
  }
  .trailing { font-family: var(--mono); font-size: 10px; letter-spacing: 0.14em; }
  .input {
    font-family: var(--sans);
    font-size: 14px;
    padding: 9px 12px;
    border: 1px solid var(--rule);
    background: var(--bg);
    color: var(--ink);
    border-radius: var(--radius-input, 4px);
    width: 100%;
  }
  .field.is-mono .input { font-family: var(--mono); font-size: 13px; }
  .input:focus { outline: 2px solid var(--accent); outline-offset: 1px; border-color: var(--accent); }
  .desc { font-family: var(--sans); font-size: 11.5px; color: var(--ink-3); }
</style>
