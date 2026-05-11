export interface KeyboardContext {
  onNext: () => void;
  onPrev: () => void;
  onOpen: () => void;
  onToggleRead: () => void;
  onToggleSaved: () => void;
  onViewOriginal: () => void;
  onEscape: () => void;
  setModalOpen: (open: boolean) => void;
  onMeasureNarrow?: () => void;
  onMeasureComfortable?: () => void;
  onMeasureWide?: () => void;
  onBack?: () => void;
  onNavigate?: (path: string) => void;
  onRefreshAll?: () => void;
  onMarkScopeRead?: () => void;
  onCycleTheme?: () => void;
}

const FORM_TAGS = new Set(['INPUT', 'TEXTAREA', 'SELECT']);
const CHORD_TIMEOUT_MS = 1500;

export function isFormControl(el: Element): boolean {
  if (FORM_TAGS.has(el.tagName)) return true;
  return el.getAttribute('contenteditable') !== null;
}

export function buildHandler(ctx: KeyboardContext) {
  let chordPending = false;
  let chordTimer: ReturnType<typeof setTimeout> | null = null;

  function clearChord() {
    chordPending = false;
    if (chordTimer !== null) { clearTimeout(chordTimer); chordTimer = null; }
  }

  return (e: KeyboardEvent) => {
    if (e.target instanceof Element && isFormControl(e.target)) return;

    if (chordPending) {
      clearChord();
      switch (e.key) {
        case 'u': ctx.onNavigate?.('/'); break;
        case 's': ctx.onNavigate?.('/saved'); break;
        case 'f': ctx.onNavigate?.('/feeds'); break;
        case 'c': ctx.onNavigate?.('/categories'); break;
        case ',': ctx.onNavigate?.('/settings'); break;
      }
      e.preventDefault();
      return;
    }

    if (e.shiftKey) {
      switch (e.key) {
        case 'R': ctx.onRefreshAll?.(); e.preventDefault(); return;
        case 'A': ctx.onMarkScopeRead?.(); e.preventDefault(); return;
      }
    }

    switch (e.key) {
      case 'g':
        chordPending = true;
        chordTimer = setTimeout(clearChord, CHORD_TIMEOUT_MS);
        e.preventDefault();
        break;
      case 't': ctx.onCycleTheme?.(); break;
      case 'j': case 'ArrowDown': ctx.onNext(); break;
      case 'k': case 'ArrowUp':   ctx.onPrev(); break;
      case 'o': case 'Enter':     ctx.onOpen(); break;
      case 'm':                   ctx.onToggleRead(); break;
      case 's':                   ctx.onToggleSaved(); break;
      case 'v':                   ctx.onViewOriginal(); break;
      case 'Escape':              ctx.onEscape(); break;
      case '?':                   ctx.setModalOpen(true); break;
      case '1':                   ctx.onMeasureNarrow?.(); break;
      case '2':                   ctx.onMeasureComfortable?.(); break;
      case '3':                   ctx.onMeasureWide?.(); break;
      case 'h':                   ctx.onBack?.(); break;
    }
  };
}
