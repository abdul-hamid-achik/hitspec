import { h } from './dom.js';
import { errorMessage } from './format.js';

const DEFAULT_TIMEOUT = 4200;

function root() {
  return document.getElementById('toasts');
}

/**
 * Show a transient notification.
 * @param {string} message
 * @param {{severity?:'info'|'success'|'warning'|'error', title?:string, timeout?:number, action?:{label:string,onClick:Function}}} [opts]
 */
export function toast(message, opts = {}) {
  const { severity = 'info', title, timeout = DEFAULT_TIMEOUT, action } = opts;
  const container = root();
  if (!container) return () => {};

  let closed = false;
  const node = h(
    'div',
    { class: ['toast', severity].filter(Boolean), role: 'status' },
    h(
      'div',
      { class: 'msg' },
      title ? h('div', { class: 'title', text: title }) : null,
      h('div', { text: String(message ?? '') }),
    ),
    action
      ? h('button', { class: 'btn sm ghost', text: action.label, onClick: () => { action.onClick?.(); dismiss(); } })
      : null,
    h('button', { class: 'btn sm ghost icon', text: '✕', title: 'Dismiss', onClick: () => dismiss() }),
  );

  container.appendChild(node);

  function dismiss() {
    if (closed) return;
    closed = true;
    clearTimeout(timer);
    node.classList.add('leaving');
    setTimeout(() => node.remove(), 180);
  }

  const timer = timeout > 0 ? setTimeout(dismiss, timeout) : null;
  node.addEventListener('mouseenter', () => clearTimeout(timer));
  return dismiss;
}

export function toastError(err, opts = {}) {
  return toast(errorMessage(err), { severity: 'error', ...opts });
}

export function toastSuccess(message, opts = {}) {
  return toast(message, { severity: 'success', ...opts });
}
