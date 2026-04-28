<script lang="ts">
	// Sidebar footer: small icon strip with theme switcher, hotkeys
	// help (toggles the global modal), and settings. Sits at the
	// bottom of the desktop sidebar. The wordmark in the sidebar
	// header is the only branding we render in the chrome.
	import { theme, cycleTheme, type Theme } from '$lib/theme.svelte';
	import { hotkeysModal } from '$lib/hotkeys-modal.svelte';

	function themeLabel(t: Theme): string {
		switch (t) {
			case 'light':
				return 'Light';
			case 'sepia':
				return 'Sepia';
			case 'dark':
				return 'Dark';
			default:
				return 'System';
		}
	}
</script>

<div class="sidebar-footer">
	<button
		type="button"
		class="icon-btn"
		aria-label={'Theme: ' + themeLabel(theme.theme) + ' (click to cycle)'}
		title={'Theme: ' + themeLabel(theme.theme)}
		onclick={cycleTheme}
	>
		<!-- circle-half glyph: communicates "theme" without depending on
		     a current-mode lookup. -->
		<svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
			<circle cx="8" cy="8" r="6.25" fill="none" stroke="currentColor" stroke-width="1.25" />
			<path d="M8 1.75 A6.25 6.25 0 0 1 8 14.25 Z" fill="currentColor" />
		</svg>
	</button>

	<button
		type="button"
		class="icon-btn"
		aria-label="Keyboard shortcuts"
		title="Keyboard shortcuts (?)"
		onclick={() => hotkeysModal.toggle()}
	>
		<svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
			<rect
				x="1.5"
				y="3.75"
				width="13"
				height="8.5"
				rx="1.5"
				fill="none"
				stroke="currentColor"
				stroke-width="1.1"
			/>
			<circle cx="4.25" cy="6.5" r="0.75" fill="currentColor" />
			<circle cx="8" cy="6.5" r="0.75" fill="currentColor" />
			<circle cx="11.75" cy="6.5" r="0.75" fill="currentColor" />
			<rect x="4" y="9" width="8" height="1.25" rx="0.5" fill="currentColor" />
		</svg>
	</button>

	<a class="icon-btn" href="/settings" aria-label="Settings" title="Settings">
		<svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
			<circle cx="8" cy="8" r="2.25" fill="none" stroke="currentColor" stroke-width="1.25" />
			<path
				d="M8 1.5v2.25 M8 12.25v2.25 M1.5 8h2.25 M12.25 8h2.25 M3.4 3.4l1.6 1.6 M11 11l1.6 1.6 M3.4 12.6l1.6-1.6 M11 5l1.6-1.6"
				stroke="currentColor"
				stroke-width="1.1"
				stroke-linecap="round"
			/>
		</svg>
	</a>
</div>

<style>
	.sidebar-footer {
		display: flex;
		gap: 4px;
		padding: 10px 16px 14px;
		border-top: 1px solid var(--rule);
	}
	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 4px;
		background: transparent;
		color: var(--ink-3);
		cursor: pointer;
		text-decoration: none;
		padding: 0;
	}
	@media (hover: hover) {
		.icon-btn:hover {
			color: var(--ink);
			background: var(--surface);
		}
	}
</style>
