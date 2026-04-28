// Shared formatting helpers. Kept module-only (no Svelte runes) so
// they can be imported from any component or test without the
// `.svelte` extension dance and without dragging the full EntryRow
// component graph into the import chain.

export function formatAgo(unix?: number | null): string {
	if (!unix) return 'recent';
	const s = Math.max(0, Math.floor(Date.now() / 1000 - unix));
	if (s < 60) return s + 's';
	if (s < 3600) return Math.floor(s / 60) + 'm';
	if (s < 86400) return Math.floor(s / 3600) + 'h';
	return Math.floor(s / 86400) + 'd';
}
