import { h, clear } from './dom.js';
import { fuzzyScore, matches } from './format.js';
import { openOverlay } from './ui.js';

/**
 * Fuzzy command palette. Items are supplied by the caller so the palette can
 * offer views, actions, files, requests, environments and themes in one list.
 *
 * @param {{title:string, hint?:string, section?:string, run:Function, keywords?:string}[]} items
 */
export function openPalette(items) {
  const input = h('input', { class: 'input', type: 'text', placeholder: 'Type a command, file or request…', ariaLabel: 'Command palette' });
  const list = h('div', { class: 'palette-list', role: 'listbox' });
  const panel = h('div', { class: 'palette', role: 'dialog', ariaLabel: 'Command palette' }, input, list);

  let filtered = items;
  let activeIndex = 0;

  const close = openOverlay(panel);

  function paint() {
    const query = input.value.trim();
    filtered = query
      ? items
          .map((item) => {
            const haystack = `${item.title} ${item.section ?? ''} ${item.hint ?? ''} ${item.keywords ?? ''}`;
            const exact = matches(haystack, query) ? 1000 : 0;
            const fuzzy = Math.max(0, fuzzyScore(item.title, query));
            return { item, score: exact + fuzzy };
          })
          .filter((entry) => entry.score > 0)
          .sort((a, b) => b.score - a.score)
          .map((entry) => entry.item)
      : items;

    activeIndex = Math.min(activeIndex, Math.max(0, filtered.length - 1));
    clear(list);

    if (!filtered.length) {
      list.append(h('div', { class: 'palette-empty', text: 'No matches' }));
      return;
    }

    let lastSection = null;
    filtered.forEach((item, index) => {
      if (!query && item.section && item.section !== lastSection) {
        lastSection = item.section;
        list.append(h('div', { class: 'section-title', style: { padding: '8px 10px 3px' }, text: item.section }));
      }
      list.append(
        h('button', {
          class: ['palette-item', index === activeIndex ? 'active' : ''].filter(Boolean),
          role: 'option',
          ariaSelected: index === activeIndex ? 'true' : 'false',
          onClick: () => choose(index),
          onMousemove: () => {
            if (activeIndex !== index) {
              activeIndex = index;
              paintActiveOnly();
            }
          },
        },
          h('span', { class: 'title', text: item.title }),
          item.hint ? h('span', { class: 'hint', text: item.hint }) : null),
      );
    });
    scrollActiveIntoView();
  }

  function paintActiveOnly() {
    const nodes = [...list.querySelectorAll('.palette-item')];
    nodes.forEach((node, index) => {
      node.classList.toggle('active', index === activeIndex);
      node.setAttribute('aria-selected', index === activeIndex ? 'true' : 'false');
    });
    scrollActiveIntoView();
  }

  function scrollActiveIntoView() {
    list.querySelectorAll('.palette-item')[activeIndex]?.scrollIntoView({ block: 'nearest' });
  }

  function choose(index) {
    const item = filtered[index];
    if (!item) return;
    close();
    // Run after teardown so any dialog the action opens lands on a clean stack.
    setTimeout(() => {
      try {
        item.run();
      } catch (err) {
        console.error('[hitspec] palette action failed:', err);
      }
    }, 0);
  }

  input.addEventListener('input', () => {
    activeIndex = 0;
    paint();
  });
  input.addEventListener('keydown', (event) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      activeIndex = Math.min(filtered.length - 1, activeIndex + 1);
      paintActiveOnly();
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      activeIndex = Math.max(0, activeIndex - 1);
      paintActiveOnly();
    } else if (event.key === 'Enter') {
      event.preventDefault();
      choose(activeIndex);
    } else if (event.key === 'Tab') {
      event.preventDefault();
    }
  });

  paint();
  input.focus();
  return close;
}
