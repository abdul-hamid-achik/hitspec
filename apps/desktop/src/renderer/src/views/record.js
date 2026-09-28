import * as actions from '../actions.js';
import { h, clear, appendNodes } from '../dom.js';
import { formatDuration } from '../format.js';
import { btn, card, empty, methodBadge, pill, stat, statGrid, statusCode, table } from '../ui.js';
import { syncInput } from '../widgets.js';

export function createRecordView(root) {
  const head = h('div', { class: 'view-head' });
  const body = h('div', { class: 'view-body' });
  clear(root);
  root.append(h('div', { class: 'view' }, head, body));

  const targetInput = h('input', { class: 'input mono', placeholder: 'https://api.example.com', ariaLabel: 'Target URL' });
  const portInput = h('input', { class: 'input mono', type: 'number', min: '1', value: '8081', style: { width: '110px' }, ariaLabel: 'Proxy port' });
  const dedupeBox = h('input', { type: 'checkbox', checked: true });

  const syncForm = () =>
    actions.setRecordForm({
      targetUrl: targetInput.value.trim(),
      port: Number(portInput.value) || 8081,
      deduplicate: dedupeBox.checked,
    });

  targetInput.addEventListener('input', syncForm);
  portInput.addEventListener('change', syncForm);
  dedupeBox.addEventListener('change', syncForm);

  function update(s) {
    const form = s.record.form;
    syncInput(targetInput, form.targetUrl ?? '');
    syncInput(portInput, form.port ?? 8081);
    if (document.activeElement !== dedupeBox) dedupeBox.checked = Boolean(form.deduplicate);

    const status = s.record.status;
    const running = Boolean(status?.running);
    const recordings = status?.recordings ?? [];

    clear(head);
    appendNodes(
      head,
      h('h2', { text: 'Recording proxy' }),
      running
        ? pill(`recording :${status.port ?? form.port} → ${status.targetUrl ?? form.targetUrl}`, 'success')
        : pill('stopped'),
      h('div', { class: 'spacer' }),
      running
        ? [
            btn('Export .http', { variant: 'primary', onClick: () => actions.exportRecordings() }),
            btn('Clear', { onClick: () => actions.clearRecordings() }),
            btn('Stop', { variant: 'danger', onClick: () => actions.stopRecord() }),
          ]
        : btn('Start recording', { variant: 'primary', busy: Boolean(s.busy.record), onClick: () => { syncForm(); actions.startRecord(); } }),
    );

    clear(body);
    appendNodes(
      body,
      card('Target',
        h('div', { class: 'stack' },
          h('div', { class: 'row wrap' },
            h('div', { class: 'field grow' }, h('label', { text: 'Upstream base URL' }), targetInput),
            h('div', { class: 'field' }, h('label', { text: 'Proxy port' }), portInput)),
          h('label', { class: 'checkbox' }, dedupeBox, h('span', { text: 'Deduplicate repeated requests' })),
          h('div', { class: 'muted', style: { fontSize: '11.5px', lineHeight: 1.6 } },
            'Start the proxy, then point your app or curl at ',
            h('code', { class: 'mono accent', text: `http://localhost:${form.port ?? 8081}` }),
            ' instead of the real host. Every request is captured and can be exported as a runnable .http file — the fastest way to turn existing traffic into tests.'))),

      running || recordings.length
        ? card(`Recordings (${status?.count ?? recordings.length})`,
            recordings.length
              ? table({
                  columns: [
                    { label: 'Method', width: '90px', render: (rec) => methodBadge(rec.method) },
                    { label: 'Path', class: 'mono ell', render: (rec) => rec.path },
                    { label: 'Status', class: 'num', width: '80px', render: (rec) => statusCode(rec.statusCode) },
                    { label: 'Content type', class: 'mono', width: '180px', render: (rec) => rec.contentType || '—' },
                    { label: 'Duration', class: 'num', width: '92px', render: (rec) => formatDuration(rec.duration) },
                  ],
                  rows: recordings,
                })
              : h('div', { class: 'muted', text: running ? 'Listening — no requests captured yet.' : 'Nothing recorded.' }),
            {
              bodyClass: '',
              actions: [btn('Refresh', { size: 'sm', onClick: () => actions.loadRecord() })],
            })
        : null,

      running
        ? statGrid([
            stat('Proxy', `localhost:${status?.port ?? form.port}`, 'accent'),
            stat('Upstream', status?.targetUrl ?? form.targetUrl ?? '—'),
            stat('Captured', String(status?.count ?? recordings.length)),
          ])
        : null,

      !running && !recordings.length
        ? empty({
            glyph: '●',
            title: 'Nothing recorded yet',
            message: 'Give the proxy a target URL and start recording. The captured traffic can be exported straight into your workspace as assertions-ready .http files.',
          })
        : null,
    );
  }

  return { update };
}
