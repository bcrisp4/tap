// Plan 21 / T7–T9 — StatsPanel grows three contracts:
//   * T7: an icon-only GitHub link inline with the version heading.
//   * T8: the recent-errors heading is hidden when no errors recorded.
//   * T9: each recent error renders as a linked feed-title row with
//         the error string and a relative timestamp.
//
// Source-inspection mirrors the project's other component tests.

import { describe, it, expect } from 'vitest';
// @ts-expect-error -- node builtin without @types/node
import { readFileSync } from 'node:fs';
// @ts-expect-error -- node builtin without @types/node
import { fileURLToPath } from 'node:url';
// @ts-expect-error -- node builtin without @types/node
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const SRC = readFileSync(resolve(here, 'StatsPanel.svelte'), 'utf8');

describe('StatsPanel inline GitHub link (Plan 21 T7)', () => {
	it('renders a link to github.com/bcrisp4/tap with target=_blank + rel', () => {
		// Lucide-svelte v1.0.1 dropped the GitHub brand glyph (Lucide
		// upstream removed it), so we use an inline SVG mark. The
		// contract is the link's metadata, not the icon font.
		expect(SRC).toMatch(/href="https:\/\/github\.com\/bcrisp4\/tap"/);
		expect(SRC).toMatch(/target="_blank"/);
		expect(SRC).toMatch(/rel="[^"]*noreferrer[^"]*"/);
		expect(SRC).toMatch(/title="View source on GitHub"/);
	});

	it('groups the version heading and the GitHub link in a header row', () => {
		// The header is a flex row with the version on the left and the
		// GitHub icon on the right.
		expect(SRC).toMatch(/<header[^>]*class="stats-header"/);
	});
});

describe('StatsPanel recent-errors empty state (Plan 21 T8)', () => {
	it('gates the recent-errors section on a non-empty list', () => {
		// Empty `recent_errors` ⇒ the heading + list collapse entirely.
		// Anything else (a 0-count badge, an empty <ul>) is noise on
		// what's already a sparse settings page.
		expect(SRC).toMatch(/{#if\s+recentErrors\.length\s*>\s*0\s*}/);
	});
});
