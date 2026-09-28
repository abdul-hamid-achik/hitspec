import { h, clear, frag } from './dom.js';
import { escapeHtml, highlightJson } from './highlight.js';
import { methodClass } from './format.js';

/* ---- primitives ----------------------------------------------------------- */

export function btn(label, opts = {}) {
  const {
    onClick,
    variant = '',
    size = '',
    disabled = false,
    title,
    kbd,
    icon,
    busy = false,
    type = 'button',
  } = opts;

  const node = h(
    'button',
    {
      class: ['btn', variant, size, busy ? 'busy' : ''].filter(Boolean),
      type,
      disabled: disabled || busy,
      title: title ?? (typeof label === 'string' ? label : undefined),
      onClick: busy ? undefined : onClick,
    },
    busy ? h('span', { class: 'spinner' }) : null,
    icon && !busy ? h('span', { class: 'ico', text: icon }) : null,
    typeof label === 'string' ? h('span', { text: label }) : label,
    kbd ? h('span', { class: 'kbd', text: kbd }) : null,
  );
  return node;
}

export function pill(text, tone = '') {
  return h('span', { class: ['pill', tone].filter(Boolean), text: String(text ?? '') });
}

export function methodBadge(method) {
  const m = String(method ?? 'GET').toUpperCase();
  return h('span', { class: ['method', methodClass(m)], text: m });
}

export function statusCode(code) {
  const n = Number(code);
  const bucket = Number.isFinite(n) && n > 0 ? `s${Math.floor(n / 100)}` : '';
  return h('span', { class: ['status-code', bucket].filter(Boolean), text: n > 0 ? String(n) : '—' });
}

export function spinner(size = 14) {
  return h('span', { class: 'spinner', style: { width: `${size}px`, height: `${size}px` } });
}

export function progressBar(ratio) {
  const pct = Math.max(0, Math.min(1, Number(ratio) || 0));
  return h('div', { class: 'progress' }, h('i', { style: { width: `${pct * 100}%` } }));
}

export function dot(tone = '') {
  return h('span', { class: ['dot', tone].filter(Boolean) });
}

/* ---- layout --------------------------------------------------------------- */

export function card(title, body, opts = {}) {
  return h(
    'section',
    { class: ['card', opts.class].filter(Boolean) },
    title || opts.actions
      ? h(
          'header',
          { class: 'card-head' },
          typeof title === 'string' ? h('h3', { text: title }) : title,
          opts.sub ? h('span', { class: 'sub muted', text: opts.sub }) : null,
          opts.actions ? h('div', { class: 'row right' }, opts.actions) : null,
        )
      : null,
    h('div', { class: opts.bodyClass ?? 'card-body' }, body),
  );
}

export function stat(label, value, tone = '') {
  return h(
    'div',
    { class: ['stat', tone].filter(Boolean) },
    h('div', { class: 'label', text: label }),
    h('div', { class: ['value', String(value).length > 9 ? 'sm' : ''].filter(Boolean), text: String(value) }),
  );
}

export function statGrid(stats) {
  return h('div', { class: 'stat-grid' }, stats);
}

export function empty({ glyph = '◌', title = 'Nothing here', message = '', actions = [] }) {
  return h(
    'div',
    { class: 'empty' },
    h('div', { class: 'glyph', text: glyph }),
    h('h3', { text: title }),
    message ? h('p', { text: message }) : null,
    actions.length ? h('div', { class: 'actions' }, actions) : null,
  );
}

export function tabs(items, active, onSelect) {
  return h(
    'div',
    { class: 'tabs', role: 'tablist' },
    items.map((item) =>
      h(
        'button',
        {
          class: ['tab', item.id === active ? 'active' : ''],
          role: 'tab',
          ariaSelected: item.id === active ? 'true' : 'false',
          onClick: () => onSelect(item.id),
        },
        h('span', { text: item.label }),
        item.count !== undefined && item.count !== null
          ? h('span', { class: 'tab-count', text: String(item.count) })
          : null,
        item.badge ? item.badge : null,
      ),
    ),
  );
}

export function table({ columns, rows, emptyText = 'No data', onRowClick, isSelected }) {
  if (!rows.length) return h('div', { class: 'muted', style: { padding: '14px' }, text: emptyText });
  return h(
    'table',
    { class: 'table' },
    h(
      'thead',
      {},
      h(
        'tr',
        {},
        columns.map((c) => h('th', { class: c.class, text: c.label, style: c.width ? { width: c.width } : null })),
      ),
    ),
    h(
      'tbody',
      {},
      rows.map((row) =>
        h(
          'tr',
          {
            class: [onRowClick ? 'clickable' : '', isSelected?.(row) ? 'selected' : ''].filter(Boolean),
            onClick: onRowClick ? () => onRowClick(row) : undefined,
          },
          columns.map((c) => h('td', { class: c.class }, typeof c.render === 'function' ? c.render(row) : row[c.key])),
        ),
      ),
    ),
  );
}

export function kv(entries) {
  if (!entries.length) return h('div', { class: 'muted', text: '—' });
  return h(
    'div',
    { class: 'kv' },
    entries.map(([key, value]) =>
      frag(h('div', { class: 'k', text: String(key) }), h('div', { class: 'v' }, value instanceof Node ? value : String(value ?? '—'))),
    ),
  );
}

/** Highlighted, read-only code block. `lang` is 'json' or 'text'. */
export function codeBlock(text, { lang = 'text', wrap = false, maxHeight } = {}) {
  const body = lang === 'json' ? highlightJson(String(text ?? '')) : escapeHtml(String(text ?? ''));
  const pre = h('pre', { class: ['code', wrap ? 'plain' : ''].filter(Boolean) });
  // Token HTML is produced by our own escaper, never from raw user markup.
  pre.innerHTML = body;
  if (maxHeight) pre.style.maxHeight = maxHeight;
  return pre;
}

/* ---- overlays ------------------------------------------------------------- */

let overlayStack = [];

function overlayRoot() {
  return document.getElementById('overlay');
}

export function closeTopOverlay() {
  const top = overlayStack[overlayStack.length - 1];
  if (top) top.close();
  return Boolean(top);
}

export function openOverlay(node, { onClose, dismissOnBackdrop = true } = {}) {
  const root = overlayRoot();
  root.hidden = false;
  clear(root);
  root.appendChild(node);

  let closed = false;
  const close = () => {
    if (closed) return;
    closed = true;
    document.removeEventListener('keydown', onKey, true);
    overlayStack = overlayStack.filter((entry) => entry.close !== close);
    if (overlayStack.length === 0) {
      clear(root);
      root.hidden = true;
    }
    onClose?.();
  };
  const onKey = (event) => {
    if (event.key === 'Escape') {
      event.stopPropagation();
      event.preventDefault();
      close();
    }
  };

  if (dismissOnBackdrop) {
    root.addEventListener(
      'mousedown',
      (event) => {
        if (event.target === root) close();
      },
      { once: true },
    );
  }
  document.addEventListener('keydown', onKey, true);
  overlayStack.push({ close, node });
  return close;
}

export function modal({ title, body, actions = [], wide = false, onClose, onMount }) {
  const node = h(
    'div',
    { class: ['modal', wide ? 'wide' : ''].filter(Boolean), role: 'dialog', ariaLabel: String(title ?? 'Dialog') },
    h('header', { class: 'modal-head' }, h('h3', { text: String(title ?? '') }), h('div', { class: 'spacer' })),
    h('div', { class: 'modal-body' }, body),
    actions.length ? h('footer', { class: 'modal-foot' }, actions) : null,
  );
  const close = openOverlay(node, { onClose });
  onMount?.(node, close);
  return close;
}

export function confirmDialog({ title = 'Are you sure?', message = '', confirmLabel = 'Confirm', cancelLabel = 'Cancel', danger = false }) {
  return new Promise((resolve) => {
    let settled = false;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      resolve(value);
    };
    const close = modal({
      title,
      body: h('p', { class: 'muted', style: { margin: 0, lineHeight: 1.6 }, text: message }),
      onClose: () => finish(false),
      actions: [
        btn(cancelLabel, { onClick: () => { finish(false); close(); } }),
        btn(confirmLabel, { variant: danger ? 'danger' : 'primary', onClick: () => { finish(true); close(); } }),
      ],
      onMount: (node) => {
        const primary = node.querySelector('.modal-foot .btn:last-child');
        primary?.focus();
      },
    });
  });
}

/** Prompt for a single line of text. Resolves to null when cancelled. */
export function prompt({ title, label, value = '', placeholder = '', confirmLabel = 'OK', multiline = false }) {
  return new Promise((resolve) => {
    let settled = false;
    const input = multiline
      ? h('textarea', { class: 'textarea mono', rows: 8, placeholder })
      : h('input', { class: 'input mono', type: 'text', value, placeholder });
    if (!multiline) input.value = value;

    const finish = (result) => {
      if (settled) return;
      settled = true;
      resolve(result);
    };
    const submit = () => {
      finish(multiline ? input.value : input.value.trim() || null);
      close();
    };

    const close = modal({
      title,
      body: h('div', { class: 'stack' }, label ? h('label', { class: 'section-title', text: label }) : null, input),
      onClose: () => finish(null),
      actions: [
        btn('Cancel', { onClick: () => { finish(null); close(); } }),
        btn(confirmLabel, { variant: 'primary', onClick: submit }),
      ],
      onMount: () => {
        input.focus();
        if (!multiline && value) input.setSelectionRange(value.length, value.length);
        input.addEventListener('keydown', (event) => {
          if (event.key === 'Enter' && (!multiline || event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            submit();
          }
        });
      },
    });
  });
}
