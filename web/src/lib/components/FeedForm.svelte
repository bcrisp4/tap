<script lang="ts">
	// Shared add/edit form for a feed.
	//
	//   mode = 'add'  → URL field + Discover + Subscribe controls.
	//   mode = 'edit' → full editable surface for an existing feed,
	//                   including credentials. Empty credential
	//                   inputs are *omitted* from the PATCH body
	//                   (Plan 08 redaction discipline) — never sent
	//                   as empty strings.
	import type { Feed, FeedPatch, DiscoverCandidate } from '$api/types';

	let {
		mode,
		feed = null,
		busy = false,
		errorMessage = null,
		discoverPending = false,
		discoverError = null,
		candidates = [],
		onDiscover = (_url: string) => {},
		onSubscribe = (_body: { feed_url: string; title?: string }) => {},
		onUpdate = (_patch: FeedPatch) => {},
		onCancel = () => {}
	}: {
		mode: 'add' | 'edit';
		feed?: Feed | null;
		busy?: boolean;
		errorMessage?: string | null;
		discoverPending?: boolean;
		discoverError?: string | null;
		candidates?: DiscoverCandidate[];
		onDiscover?: (url: string) => void;
		onSubscribe?: (body: { feed_url: string; title?: string }) => void;
		onUpdate?: (patch: FeedPatch) => void;
		onCancel?: () => void;
	} = $props();

	// Inputs are uncontrolled $state seeded from the (possibly nullish)
	// feed prop on first render. Subsequent feed swaps re-seed via the
	// effect below; without that, navigating between sibling /feeds/[id]
	// pages would leave the form bound to the old feed.
	let urlInput = $state('');
	let titleInput = $state('');
	let crawlerInput = $state(false);
	let disabledInput = $state(false);
	let userAgentInput = $state('');
	let cookieInput = $state('');
	let usernameInput = $state('');
	let passwordInput = $state('');
	let proxyInput = $state('');

	// Re-seed when the feed identity changes. We compare by id so a
	// fresh server response that updates fields-but-keeps-id doesn't
	// clobber a partial edit in flight.
	let lastSeededId = $state<number | null>(null);
	$effect(() => {
		const f = feed;
		if (f && f.id !== lastSeededId) {
			urlInput = f.feed_url;
			titleInput = f.title;
			crawlerInput = f.crawler;
			disabledInput = f.disabled;
			userAgentInput = '';
			cookieInput = '';
			usernameInput = '';
			passwordInput = '';
			proxyInput = '';
			lastSeededId = f.id;
		}
	});

	function handleSubmit(ev: Event) {
		ev.preventDefault();
		if (busy) return;
		if (mode === 'add') {
			const url = urlInput.trim();
			if (!url) return;
			onSubscribe({ feed_url: url, title: titleInput.trim() || undefined });
			return;
		}

		// Edit mode — build a minimal FeedPatch. Only assign fields
		// the user touched OR that differ from the feed's current
		// state. Empty credential inputs are deliberately omitted so
		// the server keeps any stored value (Plan 08).
		const patch: FeedPatch = {};
		if (feed) {
			if (titleInput !== feed.title) patch.title = titleInput;
			if (urlInput !== feed.feed_url) patch.feed_url = urlInput;
			if (crawlerInput !== feed.crawler) patch.crawler = crawlerInput;
			if (disabledInput !== feed.disabled) patch.disabled = disabledInput;
		}
		if (userAgentInput.trim()) patch.user_agent = userAgentInput.trim();
		if (cookieInput) patch.cookie = cookieInput;
		if (usernameInput) patch.username = usernameInput;
		if (passwordInput) patch.password = passwordInput;
		if (proxyInput) patch.proxy_url = proxyInput;
		onUpdate(patch);
	}

	function handleDiscover() {
		if (discoverPending) return;
		const url = urlInput.trim();
		if (!url) return;
		onDiscover(url);
	}
</script>

<form class="feed-form" onsubmit={handleSubmit} data-testid="feed-form">
	{#if mode === 'add'}
		<label class="field">
			<span class="label mono">Feed or site URL</span>
			<input
				type="url"
				required
				placeholder="https://example.com/feed.xml"
				autocomplete="url"
				autocorrect="off"
				autocapitalize="off"
				spellcheck="false"
				bind:value={urlInput}
				data-testid="feed-url-input"
			/>
		</label>

		<label class="field">
			<span class="label mono">Title <em>(optional)</em></span>
			<input
				type="text"
				placeholder="defaults to the feed URL"
				bind:value={titleInput}
				data-testid="feed-title-input"
			/>
		</label>

		<div class="actions">
			<button
				type="button"
				class="btn ghost"
				onclick={handleDiscover}
				disabled={discoverPending || !urlInput.trim()}
				data-testid="feed-discover-btn"
			>
				{discoverPending ? 'discovering…' : 'Discover'}
			</button>
			<button
				type="submit"
				class="btn primary"
				disabled={busy || !urlInput.trim()}
				data-testid="feed-subscribe-btn"
			>
				{busy ? 'subscribing…' : 'Subscribe'}
			</button>
		</div>

		{#if discoverError}
			<p class="error mono">{discoverError}</p>
		{/if}

		{#if candidates.length > 0}
			<div class="candidates">
				<h3 class="mono">Discovered feeds</h3>
				<ul>
					{#each candidates as c (c.href)}
						<li>
							<button
								type="button"
								class="candidate"
								onclick={() => {
									urlInput = c.href;
									if (c.title && !titleInput) titleInput = c.title;
								}}
								data-testid="candidate"
							>
								<span class="c-title">{c.title || c.href}</span>
								<span class="c-href mono">{c.href}</span>
								{#if c.type}
									<span class="c-type mono">{c.type}</span>
								{/if}
							</button>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	{:else}
		<label class="field">
			<span class="label mono">Title</span>
			<input
				type="text"
				required
				bind:value={titleInput}
				data-testid="feed-title-input"
			/>
		</label>

		<label class="field">
			<span class="label mono">Feed URL</span>
			<input
				type="url"
				required
				bind:value={urlInput}
				data-testid="feed-url-input"
			/>
		</label>

		<div class="row">
			<label class="checkbox">
				<input type="checkbox" bind:checked={crawlerInput} data-testid="feed-crawler-input" />
				<span>Use crawler (full-content extraction)</span>
			</label>
			<label class="checkbox">
				<input
					type="checkbox"
					bind:checked={disabledInput}
					data-testid="feed-disabled-input"
				/>
				<span>Pause polling</span>
			</label>
		</div>

		<details class="advanced">
			<summary>Credentials and overrides</summary>
			<p class="hint mono">
				Stored credentials are redacted on read. Leave a field blank to keep the existing
				value; type a new value to replace.
			</p>
			<label class="field">
				<span class="label mono">User agent</span>
				<input type="text" bind:value={userAgentInput} placeholder="Tap/1.0" />
			</label>
			<label class="field">
				<span class="label mono">Cookie</span>
				<input
					type="text"
					autocomplete="off"
					bind:value={cookieInput}
					data-testid="feed-cookie-input"
					placeholder="leave empty to keep existing"
				/>
			</label>
			<div class="row">
				<label class="field">
					<span class="label mono">HTTP basic — username</span>
					<input
						type="text"
						autocomplete="off"
						bind:value={usernameInput}
						data-testid="feed-username-input"
						placeholder="leave empty to keep existing"
					/>
				</label>
				<label class="field">
					<span class="label mono">password</span>
					<input
						type="password"
						autocomplete="new-password"
						bind:value={passwordInput}
						data-testid="feed-password-input"
						placeholder="leave empty to keep existing"
					/>
				</label>
			</div>
			<label class="field">
				<span class="label mono">Proxy URL</span>
				<input
					type="text"
					autocomplete="off"
					bind:value={proxyInput}
					data-testid="feed-proxy-input"
					placeholder="leave empty to keep existing"
				/>
			</label>
		</details>

		<div class="actions">
			<button type="button" class="btn ghost" onclick={onCancel}>Cancel</button>
			<button
				type="submit"
				class="btn primary"
				disabled={busy}
				data-testid="feed-save-btn"
			>
				{busy ? 'saving…' : 'Save changes'}
			</button>
		</div>
	{/if}

	{#if errorMessage}
		<p class="error mono" data-testid="feed-form-error">{errorMessage}</p>
	{/if}
</form>

<style>
	.feed-form {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		flex: 1;
	}
	.label {
		font-size: 10.5px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--ink-3);
	}
	.label em {
		font-style: normal;
		text-transform: none;
		letter-spacing: 0;
		font-size: 10px;
		opacity: 0.7;
	}
	input[type='text'],
	input[type='url'],
	input[type='password'] {
		font-family: var(--serif);
		font-size: 15px;
		color: var(--ink);
		padding: 10px 12px;
		background: var(--surface);
		border: 1px solid var(--rule);
		border-radius: 5px;
		transition:
			border-color 120ms ease,
			box-shadow 120ms ease;
	}
	input:focus {
		outline: 0;
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-soft);
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
	}
	.checkbox {
		display: flex;
		align-items: center;
		gap: 8px;
		font-family: var(--sans);
		font-size: 13px;
		color: var(--ink-2);
	}
	.checkbox input {
		accent-color: var(--accent);
	}
	.actions {
		display: flex;
		gap: 10px;
		justify-content: flex-end;
		padding-top: 4px;
	}
	.btn {
		font-family: var(--sans);
		font-size: 13px;
		font-weight: 500;
		padding: 9px 18px;
		border-radius: 5px;
		cursor: pointer;
		transition:
			background 120ms ease,
			border-color 120ms ease,
			color 120ms ease;
	}
	.btn.primary {
		background: var(--accent);
		color: #fff;
		border: 1px solid var(--accent);
	}
	.btn.primary:hover:not(:disabled) {
		filter: brightness(1.05);
	}
	.btn.ghost {
		background: transparent;
		color: var(--ink-2);
		border: 1px solid var(--rule);
	}
	.btn.ghost:hover:not(:disabled) {
		color: var(--ink);
		border-color: var(--ink-4);
	}
	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.error {
		font-size: 11.5px;
		color: #c1282d;
		background: rgba(193, 40, 45, 0.08);
		padding: 8px 12px;
		border-left: 2px solid #c1282d;
		border-radius: 0 4px 4px 0;
		margin: 0;
	}
	.candidates {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding-top: 4px;
	}
	.candidates h3 {
		font-size: 10.5px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--ink-3);
		margin: 0;
		font-weight: 500;
	}
	.candidates ul {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.candidate {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 10px 12px;
		background: var(--surface);
		border: 1px solid var(--rule);
		border-radius: 5px;
		text-align: left;
		cursor: pointer;
		font: inherit;
		color: inherit;
	}
	.candidate:hover {
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.c-title {
		font-family: var(--serif);
		font-size: 14px;
		color: var(--ink);
	}
	.c-href {
		font-size: 11px;
		color: var(--ink-3);
		word-break: break-all;
	}
	.c-type {
		font-size: 10px;
		color: var(--ink-3);
		opacity: 0.8;
	}
	.advanced {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 12px;
		background: var(--bg-soft);
		border: 1px solid var(--rule);
		border-radius: 6px;
	}
	.advanced summary {
		cursor: pointer;
		font-family: var(--sans);
		font-size: 12.5px;
		color: var(--ink-2);
	}
	.hint {
		font-size: 10.5px;
		color: var(--ink-3);
		margin: 0;
		line-height: 1.5;
	}
</style>
