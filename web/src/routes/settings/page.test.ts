// Plan 21 / T6 — the About card on /settings was redundant with the
// version + GitHub-link bits surfaced inline by StatsPanel (T7). Drop
// it. The "self-hosted river of unread" tagline lives at the project
// README level; keeping it inside the app added a settings-page tax
// nobody asked for.
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
const SRC = readFileSync(resolve(here, '+page.svelte'), 'utf8');

describe('Settings page (Plan 21 T6)', () => {
	it('does not render an About card', () => {
		// The card had a literal <h2>About</h2> heading. Belt-and-braces:
		// also assert the tagline copy is gone.
		expect(SRC).not.toMatch(/<h2[^>]*>\s*About\s*<\/h2>/);
		expect(SRC).not.toMatch(/self-hosted river of unread/i);
	});

	it('drops the dedicated about-card scoped CSS', () => {
		// `.about` was the rule for the inside-the-card paragraph. With
		// no card, there's no .about rule.
		expect(SRC).not.toMatch(/\.about\s*\{/);
	});
});
