<script lang="ts">
	let { size = 14, accent = 'var(--accent)' }: { size?: number; accent?: string } = $props();
</script>

<svg width={size} height={size * 1.2} viewBox="0 0 14 17" aria-hidden="true">
	<!-- Horizontal line -->
	<rect x="0" y="4" width="14" height="1.5" rx="0.5" fill="currentColor" />
	<!-- Vertical descending from midpoint -->
	<rect x="6.25" y="4" width="1.5" height="11" rx="0.5" fill="currentColor" />
	<!-- Junction dot -->
	<circle cx="7" cy="4.75" r="3.5" fill={accent} />
</svg>
