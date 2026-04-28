// Plan 18 / T7 + T8 — the mobile tab bar mirrors the desktop sidebar
// shape: Unread, History, Saved, More. Search is reachable via the
// MobileTopBar icon, so we drop it from the tab bar.
//
// We assert the contract by reading the component source and checking
// the `tabs` array literal. This is a lo-fi test — it doesn't render
// the component — but it locks the inventory against accidental
// regressions.

import { describe, it, expect } from 'vitest';
// @ts-expect-error -- node builtin without @types/node
import { readFileSync } from 'node:fs';
// @ts-expect-error -- node builtin without @types/node
import { fileURLToPath } from 'node:url';
// @ts-expect-error -- node builtin without @types/node
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));

describe('MobileTabBar inventory', () => {
	function readSrc(): string {
		return readFileSync(resolve(here, 'MobileTabBar.svelte'), 'utf8');
	}

	it('does not include a Search tab (T7 — Search lives on MobileTopBar)', () => {
		const src = readSrc();
		expect(src).not.toMatch(/label:\s*['"]Search['"]/);
		// Also guard against the id 'search' staying in the tabs array.
		expect(src).not.toMatch(/id:\s*['"]search['"]/);
	});

	// T8 sidebar-shape inventory + per-tab icon contract is asserted
	// in mobile-tabbar-icons.test.ts; T7 only needs to verify Search
	// is gone.
});
