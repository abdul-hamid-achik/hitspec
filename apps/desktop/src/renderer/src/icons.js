/**
 * Inline stroke icons (16x16 grid). No icon-font or SVG-sprite dependency: the
 * markup is static and authored here, so injecting it is safe under the app CSP.
 */

const PATHS = {
  requests:
    '<path d="M5 2.5h4.5L13 6v7a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V3.5a1 1 0 0 1 1-1Z"/>' +
    '<path d="M9.5 2.5V6H13"/><path d="M6.5 9h4M6.5 11.5h2.5"/>',
  history: '<circle cx="8" cy="8" r="5.5"/><path d="M8 4.8V8l2.2 1.8"/>',
  stress: '<path d="M8.8 1.5 3.5 9h3.4l-.7 5.5L11.5 7H8.1l.7-5.5Z"/>',
  mock:
    '<rect x="2.5" y="2.5" width="11" height="4.5" rx="1"/>' +
    '<rect x="2.5" y="9" width="11" height="4.5" rx="1"/>' +
    '<path d="M5 4.75h.01M5 11.25h.01"/>',
  record: '<circle cx="8" cy="8" r="5.5"/><circle cx="8" cy="8" r="1.8" fill="currentColor" stroke="none"/>',
  import:
    '<path d="M8 2v7"/><path d="m5 6.5 3 3 3-3"/>' +
    '<path d="M2.5 10.5v2a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1v-2"/>',
  contract:
    '<path d="M8 1.8 13 3.6v4c0 3.2-2.1 5.4-5 6.6-2.9-1.2-5-3.4-5-6.6v-4L8 1.8Z"/>' +
    '<path d="m5.8 7.6 1.6 1.6 2.8-3"/>',
  settings:
    '<path d="M2.5 4.5h6M12.6 4.5h.9M2.5 11.5h.9M6.4 11.5h7.1"/>' +
    '<circle cx="10.6" cy="4.5" r="1.7"/><circle cx="4.4" cy="11.5" r="1.7"/>',
  folder:
    '<path d="M2 4.5a1 1 0 0 1 1-1h3l1.5 2H13a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1v-8Z"/>',
  chevron: '<path d="m4.5 6.5 3.5 3.5 3.5-3.5"/>',
  pencil: '<path d="M11.3 2.7a1.2 1.2 0 0 1 1.7 1.7L5.5 12H3.8v-1.7l7.5-7.6Z"/>',
  search: '<circle cx="7" cy="7" r="4.5"/><path d="m10.5 10.5 3 3"/>',
  plus: '<path d="M8 3.5v9M3.5 8h9"/>',
  close: '<path d="m4.5 4.5 7 7M11.5 4.5l-7 7"/>',
  play: '<path d="M5 3.2v9.6L13 8 5 3.2Z"/>',
};

/**
 * @param {keyof typeof PATHS | string} name
 * @param {{size?:number, cls?:string}} [opts]
 */
export function icon(name, { size = 16, cls = '' } = {}) {
  const span = document.createElement('span');
  span.className = ['ico', cls].filter(Boolean).join(' ');
  span.innerHTML =
    `<svg width="${size}" height="${size}" viewBox="0 0 16 16" fill="none" stroke="currentColor" ` +
    `stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` +
    `${PATHS[name] ?? ''}</svg>`;
  return span;
}
