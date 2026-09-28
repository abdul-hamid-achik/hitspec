import { h } from './dom.js';
import { highlightHttp } from './highlight.js';

const DEBOUNCE_CHARS = 60_000;

/**
 * A `.http` editor: line-number gutter + syntax highlight layer underneath a
 * transparent textarea. No third-party editor dependency, and the three layers
 * share identical font metrics so they stay pixel-aligned.
 */
export function createEditor({ onChange, onSave, onCursor } = {}) {
  const gutter = h('div', { class: 'editor-gutter', ariaHidden: 'true' });
  const highlight = h('pre', { class: 'editor-hl', ariaHidden: 'true' });
  const input = h('textarea', {
    class: 'editor-input',
    spellcheck: 'false',
    autocapitalize: 'off',
    autocomplete: 'off',
    autocorrect: 'off',
    wrap: 'off',
    ariaLabel: 'hitspec source editor',
  });

  const stack = h('div', { class: 'editor-stack' }, highlight, input);
  const el = h('div', { class: 'editor' }, gutter, stack);

  let timer = null;
  let current = null;

  function paint(text) {
    highlight.innerHTML = highlightHttp(text);
    const lines = text.split('\n').length;
    let numbers = '';
    for (let i = 1; i <= lines; i++) numbers += `${i}\n`;
    gutter.textContent = numbers;
  }

  function syncScroll() {
    highlight.scrollTop = input.scrollTop;
    highlight.scrollLeft = input.scrollLeft;
    gutter.scrollTop = input.scrollTop;
  }

  function reportCursor() {
    if (!onCursor) return;
    const upto = input.value.slice(0, input.selectionStart ?? 0);
    const lines = upto.split('\n');
    onCursor({ line: lines.length, column: (lines[lines.length - 1]?.length ?? 0) + 1 });
  }

  function handleInput() {
    const text = input.value;
    if (text.length > DEBOUNCE_CHARS) {
      clearTimeout(timer);
      timer = setTimeout(() => paint(text), 70);
    } else {
      paint(text);
    }
    onChange?.(text);
    reportCursor();
  }

  input.addEventListener('input', handleInput);
  input.addEventListener('scroll', syncScroll, { passive: true });
  input.addEventListener('click', reportCursor);
  input.addEventListener('keyup', reportCursor);
  input.addEventListener('select', reportCursor);
  input.addEventListener('keydown', (event) => {
    const mod = event.metaKey || event.ctrlKey;
    if (mod && event.key.toLowerCase() === 's') {
      event.preventDefault();
      onSave?.();
      return;
    }
    // Plain Tab inserts indentation instead of moving focus out of the editor.
    if (event.key === 'Tab' && !mod) {
      event.preventDefault();
      const start = input.selectionStart;
      const end = input.selectionEnd;
      const value = input.value;
      input.value = `${value.slice(0, start)}  ${value.slice(end)}`;
      input.selectionStart = input.selectionEnd = start + 2;
      handleInput();
    }
  });

  return {
    el,
    focus() {
      input.focus();
    },
    getValue() {
      return input.value;
    },
    /** Replace the buffer (file switch / external reload). */
    setValue(text, { keepCursor = false } = {}) {
      const next = text ?? '';
      if (next === current && input.value === next) return;
      const caret = input.selectionStart;
      current = next;
      input.value = next;
      clearTimeout(timer);
      paint(next);
      syncScroll();
      if (keepCursor && caret != null) {
        const clamped = Math.min(caret, next.length);
        input.setSelectionRange(clamped, clamped);
      } else {
        input.scrollTop = 0;
        input.scrollLeft = 0;
        syncScroll();
      }
      reportCursor();
    },
    destroy() {
      clearTimeout(timer);
      input.remove();
    },
  };
}
