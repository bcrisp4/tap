// .svelte.ts enables Svelte runes outside components.
type Scope = 'unread' | 'saved' | 'all';

function make() {
  let open = $state(false);
  let query = $state('');
  let scope = $state<Scope>('unread');
  return {
    get open() { return open; },
    get query() { return query; },
    get scope() { return scope; },
    openOverlay() { open = true; },
    close() { open = false; query = ''; },
    setQuery(v: string) { query = v; },
    setScope(v: Scope) { scope = v; },
  };
}

export const searchOverlay = make();
