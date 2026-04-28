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

	it('surfaces last_error via the title (tooltip) and aria-label attributes', () => {
		// The truncated last_error is the user's one-line clue about why
		// the feed is unhealthy. It needs to reach assistive tech AND
		// hover users.
		expect(SRC).toMatch(/title=\{f\.last_error[^}]*\}/);
		expect(SRC).toMatch(/aria-label=\{[^}]*f\.last_error[^}]*\}/);
	});
});
