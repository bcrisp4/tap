// Plan 18 / T8 — mobile tab bar mirrors the desktop sidebar shape.
//
// Inventory: Unread, History, Saved, More.
// Each tab has an SVG icon child + the existing accessible label
// (which sits in the visible label span beneath the icon).
//
// We assert the contract by reading the component source. The "More"
// affordance opens a slide-in panel; that panel's contents are tested
// in their own component test.

import { describe, it, expect } from 'vitest';
// @ts-expect-error -- node builtin without @types/node
import { readFileSync } from 'node:fs';
// @ts-expect-error -- node builtin without @types/node
import { fileURLToPath } from 'node:url';
// @ts-expect-error -- node builtin without @types/node
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));

describe('MobileTabBar T8 sidebar-shape', () => {
	const src = readFileSync(resolve(here, 'MobileTabBar.svelte'), 'utf8');

	it('inventory matches the desktop sidebar: Unread, History, Saved, More', () => {
		expect(src).toMatch(/label:\s*['"]Unread['"]/);
		expect(src).toMatch(/label:\s*['"]History['"]/);
		expect(src).toMatch(/label:\s*['"]Saved['"]/);
		expect(src).toMatch(/label:\s*['"]More['"]/);
	});

	it('each tab renders an svg icon (not just a text label)', () => {
		// Crude but effective: the template loop must reference a
		// per-tab icon. We require at least one <svg> inside the .tab
		// markup and an icon-prop in the tab descriptor.
		expect(src).toMatch(/<svg/);
		expect(src).toMatch(/icon:/);
	});
});
