import { describe, it, expect, vi } from 'vitest';
import { isFormControl, buildHandler } from '../keyboard';

describe('isFormControl', () => {
  it('returns true for INPUT', () => expect(isFormControl(document.createElement('input'))).toBe(true));
  it('returns true for TEXTAREA', () => expect(isFormControl(document.createElement('textarea'))).toBe(true));
  it('returns true for SELECT', () => expect(isFormControl(document.createElement('select'))).toBe(true));
  it('returns true for contenteditable', () => {
    const el = document.createElement('div');
    el.setAttribute('contenteditable', 'true');
    expect(isFormControl(el)).toBe(true);
  });
  it('returns false for a plain div', () => expect(isFormControl(document.createElement('div'))).toBe(false));
  it('returns false for a button', () => expect(isFormControl(document.createElement('button'))).toBe(false));
});

function makeCtx() {
  return {
    onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(),
    onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(),
    onEscape: vi.fn(), setModalOpen: vi.fn(),
    onMeasureNarrow: vi.fn(), onMeasureComfortable: vi.fn(), onMeasureWide: vi.fn(),
    onBack: vi.fn(),
  };
}

function fire(key: string, target?: Element): KeyboardEvent {
  const el = target ?? document.createElement('div');
  const event = new KeyboardEvent('keydown', { key });
  Object.defineProperty(event, 'target', { value: el, configurable: true });
  return event;
}

describe('buildHandler', () => {
  it('calls onNext for j and ArrowDown', () => {
    const ctx = makeCtx(); const h = buildHandler(ctx);
    h(fire('j')); h(fire('ArrowDown'));
    expect(ctx.onNext).toHaveBeenCalledTimes(2);
  });
  it('calls onPrev for k and ArrowUp', () => {
    const ctx = makeCtx(); const h = buildHandler(ctx);
    h(fire('k')); h(fire('ArrowUp'));
    expect(ctx.onPrev).toHaveBeenCalledTimes(2);
  });
  it('calls onOpen for o and Enter', () => {
    const ctx = makeCtx(); const h = buildHandler(ctx);
    h(fire('o')); h(fire('Enter'));
    expect(ctx.onOpen).toHaveBeenCalledTimes(2);
  });
  it('calls onToggleRead for m', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('m'));
    expect(ctx.onToggleRead).toHaveBeenCalledOnce();
  });
  it('calls onToggleSaved for s', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('s'));
    expect(ctx.onToggleSaved).toHaveBeenCalledOnce();
  });
  it('calls onViewOriginal for v', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('v'));
    expect(ctx.onViewOriginal).toHaveBeenCalledOnce();
  });
  it('calls setModalOpen(true) for ?', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('?'));
    expect(ctx.setModalOpen).toHaveBeenCalledWith(true);
  });
  it('calls onEscape for Escape', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('Escape'));
    expect(ctx.onEscape).toHaveBeenCalledOnce();
  });
  it('does not throw when optional handlers are absent', () => {
    const ctx = {
      onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(),
      onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(),
      onEscape: vi.fn(), setModalOpen: vi.fn(),
    };
    expect(() => {
      const h = buildHandler(ctx);
      h(fire('1')); h(fire('2')); h(fire('3')); h(fire('h'));
    }).not.toThrow();
  });

  it('suppresses all bindings when target is INPUT', () => {
    const ctx = makeCtx(); const h = buildHandler(ctx);
    const inp = document.createElement('input');
    h(fire('j', inp)); h(fire('m', inp)); h(fire('?', inp));
    expect(ctx.onNext).not.toHaveBeenCalled();
    expect(ctx.onToggleRead).not.toHaveBeenCalled();
    expect(ctx.setModalOpen).not.toHaveBeenCalled();
  });
});

describe('measure and back key bindings', () => {
  it('calls onMeasureNarrow for 1', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('1'));
    expect(ctx.onMeasureNarrow).toHaveBeenCalledOnce();
  });
  it('calls onMeasureComfortable for 2', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('2'));
    expect(ctx.onMeasureComfortable).toHaveBeenCalledOnce();
  });
  it('calls onMeasureWide for 3', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('3'));
    expect(ctx.onMeasureWide).toHaveBeenCalledOnce();
  });
  it('calls onBack for h', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('h'));
    expect(ctx.onBack).toHaveBeenCalledOnce();
  });
  it('does not throw when optional handlers are absent', () => {
    const ctx = {
      onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(),
      onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(),
      onEscape: vi.fn(), setModalOpen: vi.fn(),
    };
    expect(() => {
      const h = buildHandler(ctx);
      h(fire('1')); h(fire('2')); h(fire('3')); h(fire('h'));
    }).not.toThrow();
  });
});
