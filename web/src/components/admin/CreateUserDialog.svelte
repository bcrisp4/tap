<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import Button from '../Button.svelte';

  type Role = 'user' | 'admin';
  type Props = {
    open: boolean;
    onSubmit?: (v: { username: string; password: string; role: Role }) => void;
    onClose?: () => void;
  };
  const { open, onSubmit, onClose }: Props = $props();

  let username = $state('');
  let password = $state(generatePassword());
  let role = $state<Role>('user');

  function generatePassword(): string {
    const words = ['summer','deck','quiet','spark','river','amber','loop','shadow','calm','willow','aspen','ember'];
    const arr = new Uint32Array(4);
    crypto.getRandomValues(arr);
    const pick = (i: number) => words[arr[i] % words.length];
    const n = 1000 + (arr[3] % 9000);
    return `${pick(0)}-${pick(1)}-${pick(2)}-${n}`;
  }

  function regenerate() { password = generatePassword(); }

  function submit() {
    if (!username.trim()) return;
    onSubmit?.({ username: username.trim(), password, role });
  }
</script>

<Dialog {open} title="Create user" onClose={onClose ?? (() => {})}>
  <div class="body">
    <p class="p">New users sign in with this password and are prompted to enrol in two-factor on first successful sign-in.</p>

    <label class="field">
      <span class="label" id="username-label">Username</span>
      <input type="text" bind:value={username} autocomplete="off" aria-labelledby="username-label" />
    </label>

    <label class="field">
      <span class="label" id="pw-label">Initial password</span>
      <div class="field-with-aff">
        <input type="text" bind:value={password} autocomplete="off" aria-labelledby="pw-label" />
        <button type="button" class="aff" aria-label="Regenerate password" onclick={regenerate} title="Regenerate">
          <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M14 8a6 6 0 1 1-1.76-4.24"/><path d="M14 2.5V6h-3.5"/></svg>
        </button>
      </div>
      <div class="hint">4-word passphrase · shown once, copy before saving</div>
    </label>

    <div class="field">
      <span class="label">Role</span>
      <div class="seg" role="radiogroup">
        <button type="button" class="seg-btn" class:active={role === 'user'} aria-pressed={role === 'user'} onclick={() => role = 'user'}>User</button>
        <button type="button" class="seg-btn" class:active={role === 'admin'} aria-pressed={role === 'admin'} onclick={() => role = 'admin'}>Admin</button>
      </div>
      <div class="hint">{role === 'admin' ? 'Admins can manage users and view system status.' : 'Users can read, save and manage their own feeds.'}</div>
    </div>
  </div>
  {#snippet foot()}
    <div class="foot-l">password is shown once</div>
    <Button variant="quiet" onclick={onClose}>Cancel</Button>
    <Button variant="primary" onclick={submit}>Create user</Button>
  {/snippet}
</Dialog>

<style>
  .body { display: flex; flex-direction: column; gap: 14px; }
  .p { font-family: var(--sans); font-size: 13.5px; color: var(--ink-2); margin: 0 0 6px; line-height: 1.55; }
  .field { display: flex; flex-direction: column; gap: 6px; }
  .label { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); }
  .field input {
    font-family: var(--mono); font-size: 13px; color: var(--ink);
    border: 1px solid var(--rule); background: var(--bg); border-radius: 4px;
    padding: 8px 10px; outline: none;
  }
  .field input:focus { border-color: var(--ink-4); }
  .field-with-aff { display: flex; align-items: stretch; gap: 6px; }
  .field-with-aff input { flex: 1; }
  .aff {
    width: 34px;
    display: inline-flex; align-items: center; justify-content: center;
    background: var(--bg); border: 1px solid var(--rule); border-radius: 4px;
    color: var(--ink-2); cursor: pointer;
  }
  .aff:hover { color: var(--ink); border-color: var(--ink-4); }
  .hint { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); }
  .seg { display: inline-flex; border: 1px solid var(--rule); border-radius: 4px; overflow: hidden; }
  .seg-btn {
    font-family: var(--sans); font-size: 12.5px; color: var(--ink-2);
    background: var(--bg); border: 0; padding: 6px 12px; cursor: pointer;
  }
  .seg-btn.active { background: var(--ink); color: var(--bg); }
  .foot-l { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-right: auto; }
</style>
