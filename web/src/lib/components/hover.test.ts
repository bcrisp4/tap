// Plan 18 / T2 — sticky :hover styles must be gated behind
// @media (hover: hover) so touch devices (which simulate :hover after
// a tap and never clear it) don't paint a stuck grey background until
// the next tap elsewhere.
//
// We assert the contract by reading each component's source and
// verifying that every :hover rule sits inside a matching media block.
// This is a lo-fi regression test — it doesn't mount components — but
// it locks the rule against accidental regressions when adding new
// hover affordances.

import { describe, it, expect } from 'vitest';
// `@types/node` isn't a dependency (the SPA uses Vite shims) — declare
// the slice of Node's API we actually consume in this test, mirroring
// the same pattern as tests/unread.spec.ts. The node: namespace is
// resolved at runtime by vitest.
// @ts-expect-error -- node builtin without @types/node
import { readFileSync } from 'node:fs';
// @ts-expect-error -- node builtin without @types/node
import { fileURLToPath } from 'node:url';
// @ts-expect-error -- node builtin without @types/node
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));

// Components that contain row-level / chrome :hover affordances
// reachable from the mobile viewport. Modal close buttons and form
// chrome are skipped because they're not reachable on mobile (and
// keeping the list short avoids false-positive churn when adding new
// modals).
const COMPONENTS_WITH_HOVER: ReadonlyArray<string> = [
	'EntryRow.svelte',
	'MobileTopBar.svelte',
	'reader/ReaderHeader.svelte',
	'BulkActionStrip.svelte',
	'Sidebar.svelte',
	'SidebarFooter.svelte',
	'TopBar.svelte'
];

function stripHoverMediaBlocks(src: string): string {
	const HOVER_MEDIA = /@media\s*\(\s*hover\s*:\s*hover\s*\)\s*\{/g;
	let out = src;
	let match: RegExpExecArray | null;
	while ((match = HOVER_MEDIA.exec(out)) !== null) {
		// Walk depth from the opening `{` of the media query block to
		// find the matching close, then excise the entire region.
		const openIdx = match.index + match[0].length - 1;
		let depth = 1;
		let i = openIdx + 1;
		while (i < out.length && depth > 0) {
			const ch = out[i];
			if (ch === '{') depth += 1;
			else if (ch === '}') depth -= 1;
			i += 1;
		}
		out = out.slice(0, match.index) + out.slice(i);
		HOVER_MEDIA.lastIndex = 0;
	}
	return out;
}

describe('hover styles gated on (hover: hover)', () => {
	for (const rel of COMPONENTS_WITH_HOVER) {
		it(rel + ' wraps every :hover rule in @media (hover: hover)', () => {
			const src = readFileSync(resolve(here, rel), 'utf8');
			const stripped = stripHoverMediaBlocks(src);
			expect(stripped.includes(':hover')).toBe(false);
		});
	}
});
