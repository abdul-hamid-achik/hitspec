/**
 * Tiny DOM builder. Everything is created through createElement/text nodes so
 * no user-supplied string is ever parsed as HTML.
 */

const ATTR_ALIASES = {
  className: 'class',
  htmlFor: 'for',
  ariaLabel: 'aria-label',
  ariaHidden: 'aria-hidden',
  ariaSelected: 'aria-selected',
  ariaExpanded: 'aria-expanded',
  ariaLive: 'aria-live',
  tabIndex: 'tabindex',
  role: 'role',
  colSpan: 'colspan',
  rowSpan: 'rowspan',
};

function applyProp(node, key, value) {
  if (value === null || value === undefined || value === false) return;

  if (key === 'class' || key === 'className') {
    const classes = Array.isArray(value) ? value : String(value).split(/\s+/);
    node.classList.add(...classes.filter(Boolean));
    return;
  }
  if (key === 'style') {
    if (typeof value === 'string') node.style.cssText = value;
    else Object.assign(node.style, value);
    return;
  }
  if (key === 'dataset') {
    Object.assign(node.dataset, value);
    return;
  }
  if (key === 'text') {
    node.textContent = value;
    return;
  }
  if (key.startsWith('on') && typeof value === 'function') {
    node.addEventListener(key.slice(2).toLowerCase(), value);
    return;
  }
  if (key === 'value' || key === 'checked' || key === 'disabled' || key === 'selected' || key === 'hidden') {
    node[key] = value === true ? true : value;
    return;
  }
  const attr = ATTR_ALIASES[key] ?? key;
  node.setAttribute(attr, value === true ? '' : String(value));
}

export function append(parent, child) {
  if (child === null || child === undefined || child === false || child === true) return parent;
  if (Array.isArray(child)) {
    for (const c of child) append(parent, c);
    return parent;
  }
  parent.appendChild(child instanceof Node ? child : document.createTextNode(String(child)));
  return parent;
}

/**
 * @param {string} tag element name
 * @param {object|null} [props] properties/attributes/handlers
 * @param {...any} children nodes, strings, arrays or falsy values
 */
export function h(tag, props, ...children) {
  const node = document.createElement(tag);
  if (props) {
    for (const [key, value] of Object.entries(props)) applyProp(node, key, value);
  }
  append(node, children);
  return node;
}

export function frag(...children) {
  const f = document.createDocumentFragment();
  append(f, children);
  return f;
}

/**
 * Native `Element.append` stringifies null/undefined/array arguments, which
 * leaks literal "null" text into the UI. Use this whenever a child list is
 * built conditionally.
 */
export function appendNodes(parent, ...children) {
  return append(parent, children);
}

export function clear(node) {
  while (node.firstChild) node.removeChild(node.firstChild);
  return node;
}

/** Replace children in one pass; cheaper than clear + repeated append. */
export function replace(node, ...children) {
  const f = frag(...children);
  node.replaceChildren(f);
  return node;
}

export function on(node, event, handler, options) {
  node.addEventListener(event, handler, options);
  return () => node.removeEventListener(event, handler, options);
}

/**
 * Views rebuild their DOM on every state change, and stable form controls are
 * moved (not recreated) in the process. Moving a focused <input>/<textarea>
 * drops focus, which would make typing impossible, so restore focus and the
 * caret around any such rebuild.
 */
export function withPreservedFocus(mutate) {
  const active = document.activeElement;
  const editable =
    active && (active.tagName === 'INPUT' || active.tagName === 'TEXTAREA' || active.isContentEditable);
  const selection = editable && active.selectionStart != null
    ? [active.selectionStart, active.selectionEnd]
    : null;

  mutate();

  if (!editable || !active.isConnected) return;
  active.focus({ preventScroll: true });
  if (selection && active.selectionStart != null &&
      (active.selectionStart !== selection[0] || active.selectionEnd !== selection[1])) {
    try {
      active.setSelectionRange(selection[0], selection[1]);
    } catch {
      // input types like number reject setSelectionRange
    }
  }
}
