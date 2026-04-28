<script lang="ts">
	// Debounced search input. Owns nothing but the visible text — the
	// committed query (i.e. what's actually sent to the server) is
	// reported back through `onCommit` after a 250ms idle window so
	// FTS5 isn't hammered keystroke-by-keystroke. Two-way value
	// binding (`bind:value`) lets the route reflect the URL `?q=`
	// param into the input on first paint.
	let {
		value = $bindable(''),
		placeholder = 'search…',
		debounceMs = 250,
		onCommit = (_q: string) => {}
	}: {
		value?: string;
		placeholder?: string;
		debounceMs?: number;
		onCommit?: (q: string) => void;
	} = $props();

	// `committed` is what `onCommit` last saw. Holding it here means
	// the timer doesn't fire spurious calls when the parent re-syncs
	// `value` to the same string we already reported (e.g. via the
	// URL effect).
	let committed = $state(value);
	let timer: number | undefined;

	function schedule(next: string) {
		if (typeof window === 'undefined') return;
		window.clearTimeout(timer);
		timer = window.setTimeout(() => {
			if (next === committed) return;
			committed = next;
			onCommit(next);
		}, debounceMs);
	}

	function handleInput(ev: Event) {
		const next = (ev.target as HTMLInputElement).value;
		value = next;
		schedule(next);
	}

	function handleKey(ev: KeyboardEvent) {
		if (ev.key === 'Enter') {
			ev.preventDefault();
			window.clearTimeout(timer);
			if (value !== committed) {
				committed = value;
				onCommit(value);
			}
		} else if (ev.key === 'Escape') {
			value = '';
			window.clearTimeout(timer);
			if (committed !== '') {
				committed = '';
				onCommit('');
			}
		}
	}
</script>

<label class="search-box">
	<svg
		class="ico"
		width="16"
		height="16"
		viewBox="0 0 16 16"
		fill="none"
		stroke="currentColor"
		stroke-width="1.4"
		stroke-linecap="round"
		aria-hidden="true"
	>
		<circle cx="7" cy="7" r="4.5" />
		<path d="m10.5 10.5 3 3" />
	</svg>
	<input
		type="search"
		autocomplete="off"
		spellcheck="false"
		{placeholder}
		{value}
		oninput={handleInput}
		onkeydown={handleKey}
	/>
</label>

<style>
	.search-box {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 16px;
		border: 1px solid var(--rule);
		border-radius: 6px;
		background: var(--surface);
		transition:
			border-color 120ms ease,
			box-shadow 120ms ease;
	}
	.search-box:focus-within {
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-soft);
	}
	.ico {
		color: var(--ink-3);
		flex-shrink: 0;
	}
	input {
		flex: 1;
		background: transparent;
		border: 0;
		outline: 0;
		color: var(--ink);
		font-family: var(--serif);
		font-size: 16px;
		line-height: 1.4;
		padding: 0;
	}
	input::placeholder {
		color: var(--ink-3);
		font-style: italic;
	}
	/* Suppress the WebKit search "X" — we manage clear via Escape. */
	input::-webkit-search-cancel-button {
		appearance: none;
	}
</style>
