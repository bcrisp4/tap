// Persisted theme + font runes.
//
// `theme` is one of: system | light | sepia | dark.
// `font`  is one of: serif | sans (drives `data-tap-font` on <html>).
//
// `system` follows the OS preference via prefers-color-scheme; the
// other values pin the chosen palette. Selections persist in
// localStorage so the same theme is restored on reload.

type Theme = 'system' | 'light' | 'sepia' | 'dark';
type Font = 'serif' | 'sans';

function loadInitial(): { theme: Theme; font: Font } {
	if (typeof window === 'undefined') return { theme: 'system', font: 'serif' };
	const t = (localStorage.getItem('tap.theme') as Theme) ?? 'system';
	const f = (localStorage.getItem('tap.font') as Font) ?? 'serif';
	return { theme: t, font: f };
}

function preferred(): 'light' | 'dark' {
	if (typeof window === 'undefined') return 'light';
	return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

class ThemeStore {
	#initial = loadInitial();
	theme = $state<Theme>(this.#initial.theme);
	font = $state<Font>(this.#initial.font);

	resolved = $derived<'light' | 'sepia' | 'dark'>(
		this.theme === 'system' ? preferred() : this.theme
	);

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

export function applyThemeClasses() {
	if (typeof document === 'undefined') return;
	const c = document.body.classList;
	for (const t of ['theme-light', 'theme-sepia', 'theme-dark']) c.remove(t);
	c.add('theme-' + theme.resolved);
	document.documentElement.dataset.tapFont = theme.font;
}
