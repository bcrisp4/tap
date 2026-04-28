// Plan 21 / T3 — the sidebar footer's Settings glyph is the Lucide
// cog (was a hand-drawn sliders SVG). T5 adds the OPML popover as a
// sibling action; that contract lives further down once T5 lands.
//
// Source-inspection mirrors the project's other component tests
// (`mobile-tabbar.test.ts`, `touch-targets.test.ts`).

import { describe, it, expect } from 'vitest';
// @ts-expect-error -- node builtin without @types/node
import { readFileSync } from 'node:fs';
// @ts-expect-error -- node builtin without @types/node
import { fileURLToPath } from 'node:url';
// @ts-expect-error -- node builtin without @types/node
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const SRC = readFileSync(resolve(here, 'SidebarFooter.svelte'), 'utf8');

describe('SidebarFooter chrome (Plan 21)', () => {
	it('imports Settings from lucide-svelte (T3)', () => {
		expect(SRC).toMatch(/import\s*{[^}]*\bSettings\b[^}]*}\s*from\s*'lucide-svelte'/);
	});

	it('renders <Settings /> for the settings link (T3)', () => {
		// We render the Lucide component, not a hand-rolled <svg> with
		// gear paths.
		expect(SRC).toMatch(/<Settings\b/);
		// Locking the contract that the prior sliders SVG (the small
		// circle + radial paths) is gone — preserves the regression
		// guard the plan calls for.
		expect(SRC).not.toMatch(/M8 1\.5v2\.25/);
	});

	it('mounts <OpmlMenu /> in the footer icon strip (T5)', () => {
		expect(SRC).toMatch(/import\s+OpmlMenu\s+from\s+'\.\/OpmlMenu\.svelte'/);
		expect(SRC).toMatch(/<OpmlMenu\s*\/>/);
	});

	it('keeps Settings as the rightmost (last) action in the footer (T5)', () => {
		// Order: theme · hotkeys · OPML · Settings. Settings stays last
		// because it's the meta-info catch-all.
		const settingsIdx = SRC.indexOf('<Settings');
		const opmlIdx = SRC.indexOf('<OpmlMenu');
		expect(opmlIdx).toBeGreaterThan(0);
		expect(settingsIdx).toBeGreaterThan(opmlIdx);
	});
});
