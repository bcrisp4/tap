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

describe('StatsPanel linked poller-error rows (Plan 21 T9)', () => {
	it('renders feed_title as a link to /feeds/{feed_id} when feed_id > 0', () => {
		// Plan 19 enriched the `recent_errors` payload from string[] to
		// PollerError[]. The settings UI surfaces each row with the feed
		// title as a link to the feed page, the (truncated) error
		// message, and a relative-time stamp.
		expect(SRC).toMatch(/href=\{[^}]*\/feeds\//);
		expect(SRC).toMatch(/e\.feed_title/);
	});

	it('shows the error message and a relative timestamp', () => {
		// The error string itself + a `formatAgo`-style relative time.
		expect(SRC).toMatch(/e\.error/);
		expect(SRC).toMatch(/formatAgo\(e\.at\)/);
	});

	it('falls back to plain text when feed_id is 0 (process-wide errors)', () => {
		// PollerError zeros FeedID for archival/dispatcher errors that
		// aren't tied to a specific feed; we render those as plain text
		// to avoid a 404 link.
		expect(SRC).toMatch(/{#if\s+e\.feed_id/);
	});
});
