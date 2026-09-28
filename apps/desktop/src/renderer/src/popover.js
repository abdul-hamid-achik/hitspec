import { h } from './dom.js';

/**
 * Anchored popover menu (project switcher, file actions…). Closes on outside
 * click, Escape, scroll or resize; returns the close function.
 */
export function openPopover(anchor, buildContent, { width = 300 } = {}) {
  const node = h('div', { class: 'popover', role: 'menu' });
  document.body.appendChild(node);

  let closed = false;
  const close = () => {
    if (closed) return;
    closed = true;
    document.removeEventListener('mousedown', onDocDown, true);
    document.removeEventListener('keydown', onKey, true);
    window.removeEventListener('resize', close);
    window.removeEventListener('scroll', close, true);
    node.remove();
  };
  const onDocDown = (event) => {
    if (!node.contains(event.target)) close();
  };
  const onKey = (event) => {
    if (event.key === 'Escape') {
      event.stopPropagation();
      close();
    }
  };

  const place = () => {
    const rect = anchor.getBoundingClientRect();
    const left = Math.max(8, Math.min(rect.left, window.innerWidth - width - 8));
    node.style.width = `${width}px`;
    node.style.left = `${left}px`;
    node.style.top = `${rect.bottom + 6}px`;
  };

  const refresh = () => {
    node.replaceChildren();
    node.append(buildContent(refresh));
    place();
  };
  refresh();

  setTimeout(() => document.addEventListener('mousedown', onDocDown, true), 0);
  document.addEventListener('keydown', onKey, true);
  window.addEventListener('resize', close);
  window.addEventListener('scroll', close, true);
  return close;
}
