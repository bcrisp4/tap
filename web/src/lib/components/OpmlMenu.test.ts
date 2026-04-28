// Plan 21 / T4 — OPML import/export popover. The component owns:
//   * a trigger button that toggles a small <menu> below the footer
//   * an Export action that triggers a download of /api/v1/opml/export
//   * an Import action that opens a file picker and POSTs the raw
//     XML body (NOT multipart — the Go handler reads r.Body straight
//     and parses it as XML; see internal/api/opml.go).
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
const SRC = readFileSync(resolve(here, 'OpmlMenu.svelte'), 'utf8');

describe('OpmlMenu (Plan 21 T4)', () => {
	it('imports the trigger icon (FolderInput) from lucide-svelte', () => {
		expect(SRC).toMatch(/import\s*{[^}]*\bFolderInput\b[^}]*}\s*from\s*'lucide-svelte'/);
	});

	it('toggles a popover via an `open` rune', () => {
		expect(SRC).toMatch(/let\s+open\s*=\s*\$state\(false\)/);
		expect(SRC).toMatch(/aria-haspopup="menu"/);
		expect(SRC).toMatch(/aria-expanded=\{open\}/);
	});

	it('renders Import + Export actions when open', () => {
		expect(SRC).toMatch(/Import OPML/);
		expect(SRC).toMatch(/Export OPML/);
	});

	it('Import POSTs the raw file body to /opml/import via the typed client', () => {
		// The Go handler reads r.Body directly and parses it as XML —
		// NOT multipart. Sending FormData would have the server try to
		// xml.Unmarshal a `--boundary` header instead of <opml/>. We go
		// through the shared `getJSON` helper to inherit ApiError
		// unwrapping and the /api/v1 base prefix.
		expect(SRC).toMatch(/getJSON[^(]*\(['"]\/opml\/import['"]/);
		expect(SRC).toMatch(/method:\s*['"]POST['"]/);
		// The body is the file itself, not a FormData wrapper.
		expect(SRC).not.toMatch(/new\s+FormData\(/);
	});

	it('Export uses an anchor with download="tap-subscriptions.opml"', () => {
		expect(SRC).toMatch(/\/api\/v1\/opml\/export/);
		expect(SRC).toMatch(/tap-subscriptions\.opml/);
	});

	it('feeds back to the user via the toast store', () => {
		expect(SRC).toMatch(/from\s+'\$lib\/toast\.svelte'/);
		expect(SRC).toMatch(/toast\.push/);
	});

	it('invalidates the feeds query after a successful import', () => {
		// Newly-imported feeds need to surface in the sidebar without a
		// page reload.
		expect(SRC).toMatch(/invalidateQueries/);
		expect(SRC).toMatch(/keys\.feeds\(\)/);
	});

	it('exposes both menu actions as keyboard-accessible menuitem buttons', () => {
		// Earlier shape used a <label> wrapping a hidden <input type=file>,
		// which isn't tabbable inside `role="menu"` and gave keyboard /
		// screen-reader users no way to invoke Import. The popover now
		// renders two <button> menuitems; the Import button programmatically
		// triggers a separate file input that lives outside the menu DOM.
		const menuitems = SRC.match(/role="menuitem"/g) ?? [];
		expect(menuitems.length).toBe(2);
		// The Import button drives the file input by ref, not by being
		// the input's <label>.
		expect(SRC).toMatch(/fileInput\?\.click\(\)/);
		// The hidden file input is outside the menu and explicitly
		// removed from the tab order.
		expect(SRC).toMatch(/tabindex="-1"/);
	});
});
