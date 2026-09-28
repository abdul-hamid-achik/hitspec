import * as actions from '../actions.js';
import { h, clear, appendNodes } from '../dom.js';
import { formatClock, formatDuration } from '../format.js';
import { btn, card, empty, methodBadge, pill, stat, statGrid, statusCode, table } from '../ui.js';
import { filePicker, syncInput } from '../widgets.js';

export function createMockView(root) {
  const head = h('div', { class: 'view-head' });
  const body = h('div', { class: 'view-body' });
  clear(root);
  root.append(h('div', { class: 'view' }, head, body));

  const portInput = h('input', { class: 'input mono', type: 'number', min: '1', value: '3000', style: { width: '110px' }, ariaLabel: 'Mock port' });
  const delayInput = h('input', { class: 'input mono', placeholder: 'e.g. 120ms', style: { width: '130px' }, ariaLabel: 'Response delay' });
  let picker = null;
  let pickerFiles = null;

  const syncForm = () =>
    actions.setMockForm({
      port: Number(portInput.value) || 3000,
      delay: delayInput.value.trim(),
      files: picker?.selection() ?? [],
    });

  portInput.addEventListener('change', syncForm);
  delayInput.addEventListener('change', syncForm);

  function update(s) {
    const form = s.mock.form;
    syncInput(portInput, form.port ?? 3000);
    syncInput(delayInput, form.delay ?? '');

    if (!picker || pickerFiles !== s.files) {
      pickerFiles = s.files;
      picker = filePicker({
        files: s.files ?? [],
        selected: form.files ?? [],
        onChange: (files) => actions.setMockForm({ files }),
        height: '150px',
      });
    }

    const status = s.mock.status;
    const running = Boolean(status?.running);

    clear(head);
    head.append(
      h('h2', { text: 'Mock server' }),
      running ? pill(`listening on :${status.port ?? form.port}`, 'success') : pill('stopped'),
      h('div', { class: 'spacer' }),
      running
        ? btn('Stop', { variant: 'danger', onClick: () => actions.stopMock() })
        : btn('Start mock server', { variant: 'primary', busy: Boolean(s.busy.mock), onClick: () => { syncForm(); actions.startMock(); } }),
    );

    clear(body);
    if (!s.files?.length) {
      body.append(empty({ glyph: '◈', title: 'No files to mock', message: 'Open a workspace with .http files first — each request becomes a mock route.' }));
      return;
    }

    appendNodes(
      body,
      card('Configuration',
        h('div', { class: 'stack' },
          h('div', { class: 'row wrap' },
            h('div', { class: 'field' }, h('label', { text: 'Port' }), portInput),
            h('div', { class: 'field' }, h('label', { text: 'Artificial delay' }), delayInput,
              h('span', { class: 'muted', style: { fontSize: '10.5px' }, text: 'Optional, e.g. 200ms' }))),
          picker.el,
          h('div', { class: 'muted', style: { fontSize: '11.5px' }, text: 'Each request in the selected files is served from its own response block, so the mocks stay in git next to the tests.' })),
        {
          actions: running
            ? [btn('Copy base URL', { size: 'sm', onClick: () => actions.copyText(`http://localhost:${status?.port ?? form.port}`, 'Base URL copied') })]
            : [],
        }),

      running
        ? card('Routes',
            table({
              columns: [
                { label: 'Method', width: '90px', render: (route) => methodBadge(route.method) },
                { label: 'Path', class: 'mono ell', render: (route) => route.path },
                { label: 'Name', class: 'ell', render: (route) => route.name || h('span', { class: 'muted', text: '—' }) },
                { label: 'Status', class: 'num', width: '80px', render: (route) => statusCode(route.statusCode) },
                { label: 'Content type', class: 'mono', width: '180px', render: (route) => route.contentType || '—' },
              ],
              rows: status?.routes ?? [],
              emptyText: 'No routes were derived from the selected files.',
            }),
            { bodyClass: '' })
        : null,

      card(`Requests (${s.mock.requests.length})`,
        s.mock.requests.length
          ? h('div', { class: 'scroll', style: { maxHeight: '260px' } },
              table({
                columns: [
                  { label: 'At', class: 'mono', width: '86px', render: (req) => formatClock(req.at) },
                  { label: 'Method', width: '90px', render: (req) => methodBadge(req.method) },
                  { label: 'Path', class: 'mono ell', render: (req) => req.path },
                  { label: 'Status', class: 'num', width: '80px', render: (req) => statusCode(req.status) },
                  { label: 'Duration', class: 'num', width: '92px', render: (req) => formatDuration(req.duration) },
                ],
                rows: s.mock.requests,
              }))
          : h('div', { class: 'muted', text: running ? 'Waiting for requests…' : 'Start the mock server to capture traffic.' }),
        {
          bodyClass: '',
          actions: s.mock.requests.length ? [btn('Clear log', { size: 'sm', onClick: () => actions.clearMockLog() })] : [],
        }),

      running
        ? card('Try it',
            statGrid([
              stat('Base URL', `http://localhost:${status?.port ?? form.port}`, 'accent'),
              stat('Routes', String(status?.routes?.length ?? 0)),
              stat('Logged', String(s.mock.requests.length)),
            ]))
        : null,
    );
  }

  return { update };
}
