// Deterministic source-swatch colour. Hashes a string (typically a feed
// title) to one of a small palette of muted hues. The list is shared
// between the unread list (Plan 10's EntryRow) and the reader source
// strip (Plan 11's ReaderBody) so a given feed gets a stable colour
// wherever it appears in the UI.
//
// The palette is intentionally low-chroma so the Klein-blue accent stays
// the only saturated colour in the interface; these swatches are signal
// without being shouty.
const SWATCHES = [
	'#a23b3b', // muted brick
	'#3b6ea2', // dusty navy
	'#4f7a3b', // moss
	'#a26d3b', // ochre
	'#6d3ba2', // mulberry
	'#3ba29c', // teal
	'#a2823b', // mustard
	'#7a3b6e' // plum
] as const;

export function swatchFor(name: string): string {
	if (!name) return SWATCHES[0];
	let hash = 0;
	for (let i = 0; i < name.length; i++) {
		hash = (hash * 31 + name.charCodeAt(i)) | 0;
	}
	return SWATCHES[Math.abs(hash) % SWATCHES.length];
}
