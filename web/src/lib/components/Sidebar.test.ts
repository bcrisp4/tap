// Plan 21 / T2 — feeds with `error_count > 0` get a warning glyph
// next to the title in the desktop sidebar. The component pulls
// `error_count` and `last_error` straight off the feeds-list payload;
// we lock the contract by asserting the Sidebar source renders a
// Lucide AlertTriangle gated on `error_count > 0`, with the
// (truncated) `last_error` surfaced via a hover/tooltip attribute.
//
// Source-inspection mirrors `mobile-tabbar.test.ts` and
// `touch-targets.test.ts`: vitest doesn't mount Svelte components in
// this codebase, so the contract is locked by reading the file.

import { describe, it, expect } from 'vitest';
// @ts-expect-error -- node builtin without @types/node
import { readFileSync } from 'node:fs';
// @ts-expect-error -- node builtin without @types/node
import { fileURLToPath } from 'node:url';
// @ts-expect-error -- node builtin without @types/node
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const SRC = readFileSync(resolve(here, 'Sidebar.svelte'), 'utf8');

describe('Sidebar feed warning indicator (Plan 21 T2)', () => {
	it('imports AlertTriangle from lucide-svelte', () => {
		expect(SRC).toMatch(/import\s*{\s*AlertTriangle\s*}\s*from\s*'lucide-svelte'/);
	});

	it('renders the warning only when error_count > 0', () => {
		// The condition gates on `error_count > 0`; the literal `0`
		// matters — `truthy(error_count)` would also fire when the
		// payload comes back malformed.
		expect(SRC).toMatch(/{#if\s+f\.error_count\s*>\s*0\s*}/);
		expect(SRC).toMatch(/<AlertTriangle\b/);
	});

	it('surfaces a truncated last_error via title (tooltip) and aria-label', () => {
		// The truncated last_error is the user's one-line clue about why
		// the feed is unhealthy. It needs to reach assistive tech AND
		// hover users; very long stack traces / TLS errors are clipped
		// so the tooltip stays readable. The full string lives on the
		// feed detail page.
		expect(SRC).toMatch(/truncateError\(f\.last_error\)/);
		expect(SRC).toMatch(/title=\{warn\}/);
		expect(SRC).toMatch(/aria-label=\{[^}]*warn[^}]*\}/);
	});

	it('caps the truncated copy at a sensible length with an ellipsis', () => {
		// The exact cap is a tuning knob; lock the contract via the
		// presence of a numeric WARN_MAX constant + an ellipsis suffix
		// so a future drive-by edit doesn't accidentally drop the cap.
		expect(SRC).toMatch(/WARN_MAX\s*=\s*\d+/);
		expect(SRC).toMatch(/'…'/);
	});
});
