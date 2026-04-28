// Persisted theme + font runes.
//
// `theme` is one of: system | light | sepia | dark.
// `font`  is one of: serif | sans (drives `data-tap-font` on <html>).
//
// `system` follows the OS preference via prefers-color-scheme; the
// other values pin the chosen palette. Selections persist in
// localStorage so the same theme is restored on reload.

const THEMES = ['system', 'light', 'sepia', 'dark'] as const;
const FONTS = ['serif', 'sans'] as const;

export type Theme = (typeof THEMES)[number];
export type Font = (typeof FONTS)[number];

function isTheme(v: unknown): v is Theme {
	return typeof v === 'string' && (THEMES as readonly string[]).includes(v);
}
function isFont(v: unknown): v is Font {
	return typeof v === 'string' && (FONTS as readonly string[]).includes(v);
}

function loadInitial(): { theme: Theme; font: Font } {
	if (typeof window === 'undefined') return { theme: 'system', font: 'serif' };
	// Validate against allow-lists — localStorage is user-controlled
	// and can hold arbitrary strings (DevTools, prior versions, etc.).
	const t = localStorage.getItem('tap.theme');
	const f = localStorage.getItem('tap.font');
	return {
		theme: isTheme(t) ? t : 'system',
		font: isFont(f) ? f : 'serif'
	};
}

function preferred(): 'light' | 'dark' {
	if (typeof window === 'undefined') return 'light';
	return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

class ThemeStore {
	#initial = loadInitial();
	theme = $state<Theme>(this.#initial.theme);
	font = $state<Font>(this.#initial.font);

	setTheme(t: Theme) {
		this.theme = t;
		if (typeof window !== 'undefined') localStorage.setItem('tap.theme', t);
	}

	setFont(f: Font) {
		this.font = f;
		if (typeof window !== 'undefined') localStorage.setItem('tap.font', f);
	}
}

export const theme = new ThemeStore();

// Resolve the active palette on demand. Reading prefers-color-scheme
// here (rather than caching it in $derived) means the matchMedia
// `change` listener's call to applyThemeClasses() always picks up the
// latest OS preference — $derived would cache the value until a Svelte
// dep changed.
export function resolvedTheme(): 'light' | 'sepia' | 'dark' {
	return theme.theme === 'system' ? preferred() : theme.theme;
}

export function applyThemeClasses() {
	if (typeof document === 'undefined') return;
	const c = document.body.classList;
	for (const t of ['theme-light', 'theme-sepia', 'theme-dark']) c.remove(t);
	c.add('theme-' + resolvedTheme());
	document.documentElement.dataset.tapFont = theme.font;
}

// Quick-cycle order is light → sepia → dark → light. 'system' is the
// "follow OS" fallback and lives in Settings — the cycle's first click
// from 'system' lands on 'light' rather than incrementing past it.
const THEME_CYCLE: readonly Theme[] = ['light', 'sepia', 'dark'] as const;

export function cycleTheme() {
	if (theme.theme === 'system') {
		theme.setTheme(THEME_CYCLE[0]);
		return;
	}
	const i = THEME_CYCLE.indexOf(theme.theme as (typeof THEME_CYCLE)[number]);
	theme.setTheme(THEME_CYCLE[(i + 1) % THEME_CYCLE.length]);
}
