import * as actions from '../actions.js';
import { h, clear, appendNodes } from '../dom.js';
import { formatDateTime, formatDuration, formatNumber, relativeTime, truncate } from '../format.js';
import { btn, empty, methodBadge, modal, pill, stat, statGrid, statusCode, table } from '../ui.js';

export function createHistoryView(root) {
  const head = h('div', { class: 'view-head' });
  const body = h('div', { class: 'view-body' });
  clear(root);
  root.append(h('div', { class: 'view' }, head, body));

  function update(s) {
    clear(head);
    head.append(
      h('h2', { text: 'Run history' }),
      h('span', { class: 'sub', text: `${formatNumber(s.history.total)} runs stored locally in SQLite` }),
      h('div', { class: 'spacer' }),
      btn('Refresh', { size: 'sm', busy: s.history.loading, onClick: () => actions.loadRuns({ reset: true }) }),
      btn('Delete all…', { size: 'sm', variant: 'danger', disabled: !s.history.runs.length, onClick: () => actions.clearRuns() }),
    );

    clear(body);
    if (!s.history.runs.length) {
      body.append(empty({
        glyph: '⧗',
        title: s.history.loading ? 'Loading history…' : 'No runs recorded yet',
        message: 'Every request you run from this app is stored in the local history database. Results appear here with assertions, timings and body previews.',
        actions: [btn('Go to requests', { variant: 'primary', onClick: () => actions.setView('workspace') })],
      }));
      return;
    }

    appendNodes(
      body,
      h('div', { class: 'card' },
        table({
          columns: [
            { label: '#', class: 'num', width: '58px', render: (run) => String(run.id) },
            { label: 'File', class: 'ell', render: (run) => h('span', { class: 'mono', title: run.filePath, text: run.filePath }) },
            { label: 'Env', width: '88px', render: (run) => (run.environment ? pill(run.environment) : h('span', { class: 'muted', text: '—' })) },
            { label: 'When', width: '150px', render: (run) => h('span', { title: formatDateTime(run.startedAt), text: relativeTime(run.startedAt) }) },
            { label: 'Duration', class: 'num', width: '92px', render: (run) => formatDuration(run.durationMs) },
            {
              label: 'Result',
              width: '170px',
              render: (run) => h('div', { class: 'row', style: { gap: '5px' } },
                pill(`${run.passed}✓`, 'success'),
                run.failed ? pill(`${run.failed}✕`, 'danger') : null,
                run.skipped ? pill(`${run.skipped}↷`) : null),
            },
            {
              label: '',
              width: '108px',
              render: (run) => h('div', { class: 'row', style: { gap: '5px' } },
                btn('Open', { size: 'sm', onClick: () => actions.openRun(run.id) }),
                btn('✕', { size: 'sm', variant: 'ghost', title: 'Delete run', onClick: () => actions.deleteRun(run.id) })),
            },
          ],
          rows: s.history.runs,
          emptyText: 'No runs',
          onRowClick: (run) => actions.openRun(run.id),
        })),
      s.history.runs.length < s.history.total
        ? h('div', { class: 'center', style: { padding: '14px' } },
            btn('Load more', { busy: s.history.loading, onClick: () => actions.loadMoreRuns() }))
        : null,
    );

    if (s.history.selected) showDetail(s.history.selected);
  }

  let openFor = null;
  let closeDetail = null;

  function showDetail(run) {
    if (openFor === run.id && closeDetail) return;
    closeDetail?.();
    openFor = run.id;
    closeDetail = modal({
      title: `Run #${run.id} — ${run.filePath}`,
      wide: true,
      onClose: () => { openFor = null; closeDetail = null; actions.closeRun(); },
      body: detailBody(run),
      actions: [
        btn('Delete run', { variant: 'danger', onClick: () => { closeDetail?.(); actions.deleteRun(run.id); } }),
        btn('Close', { variant: 'primary', onClick: () => closeDetail?.() }),
      ],
    });
  }

  return { update };
}

function detailBody(run) {
  const results = run.results ?? [];
  return h('div', { class: 'stack' },
    h('div', { class: 'row wrap' },
      run.environment ? pill(run.environment, 'accent') : null,
      pill(formatDateTime(run.startedAt)),
      pill(formatDuration(run.durationMs))),
    statGrid([
      stat('Total', String(run.total ?? results.length)),
      stat('Passed', String(run.passed ?? 0), 'success'),
      stat('Failed', String(run.failed ?? 0), run.failed ? 'danger' : ''),
      stat('Skipped', String(run.skipped ?? 0), run.skipped ? 'warning' : ''),
    ]),
    results.length
      ? h('div', { class: 'stack-sm' }, results.map((result) => resultDetails(result)))
      : h('div', { class: 'muted', text: 'No results stored for this run.' }));
}

function resultDetails(result) {
  return h('details', { class: 'card', open: result.passed ? undefined : 'open' },
    h('summary', {
      class: 'card-head',
      style: { cursor: 'pointer', listStyle: 'none' },
    },
      h('span', { class: result.passed ? 'success' : result.skipped ? 'muted' : 'danger', text: result.passed ? '✓' : result.skipped ? '↷' : '✕' }),
      methodBadge(result.method || 'GET'),
      h('strong', { text: result.requestName || 'request' }),
      h('span', { class: 'muted mono ell grow', text: truncate(result.url || '', 70) }),
      result.statusCode ? statusCode(result.statusCode) : null,
      h('span', { class: 'muted mono', text: formatDuration(result.durationMs) })),
    h('div', { class: 'card-body stack-sm' },
      result.error ? h('div', { class: 'row' }, pill('error', 'danger'), h('code', { class: 'mono', text: result.error })) : null,
      result.description ? h('div', { class: 'muted', text: result.description }) : null,
      (result.assertions ?? []).length
        ? h('div', {}, result.assertions.map((a) =>
            h('div', { class: ['assertion', a.passed ? 'pass' : 'fail'] },
              h('span', { class: 'mark', text: a.passed ? '✓' : '✕' }),
              h('div', { class: 'grow' },
                h('div', { class: 'expr' },
                  h('span', { class: 'tok-key', text: a.subject }),
                  h('span', { class: 'tok-op', text: ` ${a.operator} ` }),
                  h('span', { class: 'tok-str', text: a.expected ?? '' })),
                a.message ? h('div', { class: 'actual', text: a.message }) : null,
                !a.passed && a.actual ? h('div', { class: 'actual', text: `actual: ${a.actual}` }) : null))))
        : null,
      result.bodyPreview
        ? h('details', {},
            h('summary', { class: 'muted', style: { cursor: 'pointer' }, text: 'Body preview' }),
            h('pre', { class: 'code plain', style: { maxHeight: '220px', marginTop: '6px' }, text: truncate(result.bodyPreview, 20000) }))
        : null));
}
