// Plan 18 / T4 - every interactive element in the mobile chrome must
// meet the iOS Human Interface Guidelines minimum 44x44 hit area.
//
// We cannot mount components in vitest without a DOM environment, so
// the contract is locked by reading each component scoped CSS and
// asserting the relevant selectors declare a min touch-target size.
// This catches regressions when adding new chrome buttons that miss
// the 44px floor.

import { describe, it, expect } from 'vitest';
// @ts-expect-error -- node builtin without @types/node
import { readFileSync } from 'node:fs';
// @ts-expect-error -- node builtin without @types/node
import { fileURLToPath } from 'node:url';
// @ts-expect-error -- node builtin without @types/node
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));

interface Rule {
	component: string;
	selector: string;
	minSize: number;
}

// Each row is a (component, selector, expected min size) contract.
// The matcher looks for the selector body inside scoped CSS and
// requires either an explicit min-height + min-width pair or a fixed
// height + width pair at >= the floor.
const RULES: ReadonlyArray<Rule> = [
	{ component: 'EntryRow.svelte', selector: '.read-dot', minSize: 44 },
	{ component: 'EntryRow.svelte', selector: '.select-box', minSize: 44 },
	{ component: 'MobileTopBar.svelte', selector: '.iconbtn', minSize: 44 },
	{ component: 'MobileTabBar.svelte', selector: '.tab', minSize: 44 },
	{ component: 'reader/MobileReaderTopBar.svelte', selector: '.m-back', minSize: 44 },
	{ component: 'reader/MobileReaderTopBar.svelte', selector: '.m-action', minSize: 44 },
	{ component: 'reader/MobileReaderFootBar.svelte', selector: '.m-foot-btn', minSize: 44 }
];

function findRuleBody(src: string, selector: string): string {
	// Find the rule by locating the selector token followed by either
	// the rule's opening `{` directly OR a comma chain that ends at a
	// `{`. We require the next non-whitespace after the selector to be
	// `,` or `{` so pseudo-state variants like `:hover` don't match.
	const escaped = selector.replace(/[.[\]()*+?^$|/\\]/g, '\\$&');
	const re = new RegExp('(^|[^a-zA-Z0-9_-])' + escaped + '\\s*[,{]', 'gm');
	let match: RegExpExecArray | null;
	while ((match = re.exec(src)) !== null) {
		// Walk forward from the match's end to the next `{` (skipping
		// other comma-chained selectors). Bail out if we hit `}`,
		// which would mean the selector landed inside another rule's
		// body (shouldn't happen for plain class selectors but guard
		// anyway).
		let i = match.index + match[0].length - 1;
		while (i < src.length && src[i] !== '{') {
			if (src[i] === '}') break;
			i += 1;
		}
		if (i >= src.length || src[i] !== '{') continue;
		// Now walk the matching close brace.
		let depth = 1;
		i += 1;
		const start = i;
		while (i < src.length && depth > 0) {
			const ch = src[i];
			if (ch === '{') depth += 1;
			else if (ch === '}') depth -= 1;
			i += 1;
		}
		return src.slice(start, i - 1);
	}
	return '';
}

function pickPxAtLeast(body: string, prop: string, minSize: number): boolean {
	const re = new RegExp('(?:^|[^-])' + prop + '\\s*:\\s*(\\d+)px', 'g');
	let match: RegExpExecArray | null;
	while ((match = re.exec(body)) !== null) {
		if (Number(match[1]) >= minSize) return true;
	}
	return false;
}

function meetsMinSize(body: string, minSize: number): boolean {
	const heightOK =
		pickPxAtLeast(body, 'min-height', minSize) ||
		pickPxAtLeast(body, 'height', minSize);
	const widthOK =
		pickPxAtLeast(body, 'min-width', minSize) ||
		pickPxAtLeast(body, 'width', minSize);
	return heightOK && widthOK;
}

describe('mobile touch targets meet 44x44 minimum (iOS HIG)', () => {
	for (const rule of RULES) {
		it(rule.component + ' ' + rule.selector + ' has min ' + rule.minSize + 'px hit area', () => {
			const src = readFileSync(resolve(here, rule.component), 'utf8');
			const body = findRuleBody(src, rule.selector);
			expect(body, 'rule body not found for ' + rule.selector).not.toBe('');
			expect(meetsMinSize(body, rule.minSize)).toBe(true);
		});
	}
});
