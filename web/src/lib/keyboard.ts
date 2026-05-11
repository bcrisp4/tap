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
}

const FORM_TAGS = new Set(['INPUT', 'TEXTAREA', 'SELECT']);

export function isFormControl(el: Element): boolean {
  if (FORM_TAGS.has(el.tagName)) return true;
  return el.getAttribute('contenteditable') !== null;
}

export function buildHandler(ctx: KeyboardContext) {
  return (e: KeyboardEvent) => {
    if (e.target instanceof Element && isFormControl(e.target)) return;
    switch (e.key) {
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
