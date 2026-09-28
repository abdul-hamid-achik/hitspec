import * as actions from '../actions.js';
import { h, clear } from '../dom.js';
import { createEditor } from '../editor.js';
import {
  formatBytes, formatDuration, formatNumber, prettyJSON, statusTone, truncate,
} from '../format.js';
import { escapeHtml, highlightJson } from '../highlight.js';
import { state, set } from '../store.js';
import {
  btn, codeBlock, empty, kv, methodBadge, pill, progressBar, stat, statGrid, statusCode, tabs,
} from '../ui.js';

const DETAIL_TABS = [
  { id: 'request', label: 'Request' },
  { id: 'response', label: 'Response' },
  { id: 'source', label: 'Source' },
];

const RESPONSE_TABS = [
  { id: 'body', label: 'Body' },
  { id: 'headers', label: 'Headers' },
  { id: 'assertions', label: 'Assertions' },
  { id: 'timing', label: 'Timing' },
  { id: 'captures', label: 'Captures' },
];

export function createWorkspaceView(root) {
  const headEl = h('div', { class: 'view-head' });
  const tabsEl = h('div', { class: 'tabs-wrap' });
  const bodyEl = h('div', { class: 'view-body flush grow' });
  const cursorEl = h('span', { class: 'item mono' });

  let editorPath = null;

  const editor = createEditor({
    onChange: (text) => actions.setSourceText(text),
    onSave: () => actions.saveSource(),
    onCursor: (next) => {
      cursorEl.textContent = `Ln ${next.line}, Col ${next.column}`;
    },
  });

  const sourceFoot = h(
    'div',
    { class: 'editor-foot' },
    h('span', { class: 'item', text: '⌘S save · Tab indents · ⌘↵ run request' }),
    cursorEl,
  );

  // The Source tab is built once and only patched afterwards: re-creating the
  // textarea on every render would drop focus mid-keystroke.
  const sourcePathEl = h('span', { class: 'mono muted ell', text: '' });
  const sourceDirtyEl = h('span', {});
  const saveBtn = h(
    'button',
    { class: 'btn primary sm', type: 'button', onClick: () => actions.saveSource() },
    h('span', { text: 'Save' }),
    h('span', { class: 'kbd', text: '⌘S' }),
  );
  const revertBtn = h('button', { class: 'btn sm', type: 'button', onClick: () => actions.revertSource() },
    h('span', { text: 'Revert' }));
  const sourceHead = h(
    'div',
    { class: 'row', style: { padding: '8px 12px', flex: '0 0 auto', borderBottom: '1px solid var(--border-soft)' } },
    sourcePathEl,
    sourceDirtyEl,
    h('div', { class: 'spacer' }),
    revertBtn,
    saveBtn,
  );
  const sourceRoot = h('div', { class: 'view-body flush grow' }, sourceHead, editor.el, sourceFoot);

  clear(root);
  root.append(h('div', { class: 'view' }, headEl, tabsEl, bodyEl));

  const selectedRequest = (s) => (s.parsed?.requests ?? []).find((r) => r.name === s.selectedRequest) ?? null;

  /* ---- head --------------------------------------------------------------- */

  function renderHead(s) {
    clear(headEl);
    const request = selectedRequest(s);
    if (!request) {
      headEl.append(
        h('h2', { class: 'ell', text: s.selectedFile ?? 'No file selected' }),
        h('span', { class: 'sub', text: s.selectedFile ? 'This file has no requests' : 'Pick a file from the sidebar' }),
      );
      return;
    }

    headEl.append(
      methodBadge(request.method),
      h('div', { class: 'stack-sm', style: { minWidth: 0, flex: '1 1 auto' } },
        h('div', { class: 'row', style: { gap: '8px' } },
          h('h2', { class: 'ell', text: request.name || request.url }),
          (request.tags ?? []).map((tag) => h('span', { class: 'tag', text: tag })),
          s.source.dirty ? pill('unsaved', 'warning') : null),
        h('div', { class: 'sub mono ell', text: `${request.method} ${request.url}` })),
      h('div', { class: 'spacer' }),
      btn('Run', {
        variant: 'primary',
        kbd: '⌘↵',
        busy: Boolean(s.busy.run),
        disabled: s.running.active,
        title: 'Run this request',
        onClick: () => actions.runRequest(),
      }),
      btn('Run file', {
        kbd: '⌘⇧↵',
        busy: Boolean(s.busy.runFile),
        disabled: s.running.active,
        title: 'Run every request in this file',
        onClick: () => actions.runFile(),
      }),
      btn('curl', { title: 'Export this request as curl', onClick: () => actions.exportCurl(request.name) }),
      btn('⋯', { variant: 'ghost', title: 'File actions', onClick: (event) => openFileMenu(event) }),
    );
  }

  function openFileMenu(event) {
    const items = [
      { label: 'Save file', kbd: '⌘S', run: () => actions.saveSource() },
      { label: 'Revert file', run: () => actions.revertSource() },
      { label: 'New file…', kbd: '⌘N', run: () => actions.newFileDialog() },
      { label: 'Reveal in file manager', run: () => actions.revealCurrentFile() },
      { label: 'Open in external editor', run: () => actions.openCurrentFileInEditor() },
      { label: 'Export file as curl', run: () => actions.exportCurl(undefined) },
      { label: 'Delete file…', danger: true, run: () => actions.deleteCurrentFile() },
    ];

    const rect = event.currentTarget.getBoundingClientRect();
    const menu = h(
      'div',
      {
        class: 'modal',
        style: {
          position: 'fixed',
          top: `${rect.bottom + 4}px`,
          right: `${Math.max(8, window.innerWidth - rect.right)}px`,
          width: '250px',
        },
      },
      h('div', { class: 'palette-list', style: { padding: '5px', maxHeight: 'none' } },
        items.map((item) =>
          h('button', {
            class: 'palette-item',
            onClick: () => { close(); item.run(); },
          },
            h('span', { class: 'title', style: item.danger ? { color: 'var(--danger)' } : null, text: item.label }),
            item.kbd ? h('span', { class: 'kbd', text: item.kbd }) : null))),
    );

    document.body.appendChild(menu);
    const close = () => {
      menu.remove();
      document.removeEventListener('mousedown', onDocDown, true);
      window.removeEventListener('resize', close);
      window.removeEventListener('scroll', close, true);
    };
    const onDocDown = (e) => { if (!menu.contains(e.target)) close(); };
    setTimeout(() => document.addEventListener('mousedown', onDocDown, true), 0);
    window.addEventListener('resize', close);
    window.addEventListener('scroll', close, true);
  }

  /* ---- tabs --------------------------------------------------------------- */

  function renderTabs(s) {
    const result = s.selectedResult ?? s.lastRun?.results?.[0] ?? null;
    clear(tabsEl);
    tabsEl.append(
      tabs(
        DETAIL_TABS.map((tab) => ({
          ...tab,
          badge:
            tab.id === 'response' && result
              ? pill(result.passed ? 'pass' : result.skipped ? 'skip' : 'fail',
                  result.passed ? 'success' : result.skipped ? '' : 'danger')
              : tab.id === 'source' && s.source.dirty ? pill('•', 'warning') : null,
        })),
        s.detailTab,
        (id) => set({ detailTab: id }),
      ),
    );
  }

  function renderBody(s) {
    if (s.detailTab === 'source') return renderSource(s);
    if (s.detailTab === 'response') return renderResponse(s);
    return renderRequest(s);
  }

  /* ---- request tab -------------------------------------------------------- */

  function renderRequest(s) {
    const request = selectedRequest(s);
    clear(bodyEl);
    bodyEl.classList.remove('flush');

    if (!request) {
      bodyEl.append(empty({
        glyph: '⌁',
        title: s.selectedFile ? 'No request selected' : 'No file open',
        message: s.selectedFile
          ? 'Choose a request from the sidebar to inspect and run it.'
          : 'Open a workspace folder that contains .http or .hitspec files.',
        actions: s.selectedFile ? [] : [btn('Open workspace…', { variant: 'primary', onClick: () => actions.openWorkspaceDialog() })],
      }));
      return;
    }

    const meta = request.metadata ?? {};
    const badges = [
      meta.timeout ? pill(`timeout ${meta.timeout}ms`) : null,
      meta.retry ? pill(`retry ${meta.retry}`) : null,
      meta.skip ? pill(`skip: ${meta.skip}`, 'warning') : null,
      meta.only ? pill('only', 'info') : null,
      meta.auth ? pill(`auth ${meta.auth.type}`, 'accent') : null,
      (meta.depends ?? []).length ? pill(`depends: ${meta.depends.join(', ')}`) : null,
    ].filter(Boolean);

    bodyEl.append(
      h('div', { class: 'stack' },
        request.description ? h('p', { class: 'muted', style: { margin: 0 }, text: request.description }) : null,
        badges.length ? h('div', { class: 'row wrap' }, badges) : null,

        h('div', { class: 'card' },
          h('div', { class: 'card-head' },
            h('h3', { text: 'URL' }),
            h('div', { class: 'spacer' }),
            btn('Copy', { size: 'sm', variant: 'ghost', onClick: () => actions.copyText(request.url, 'URL copied') })),
          h('div', { class: 'row', style: { padding: '10px 12px', gap: '10px' } },
            methodBadge(request.method),
            h('code', { class: 'mono grow', style: { overflowWrap: 'anywhere' } }, withVars(request.url)))),

        (request.queryParams ?? []).length ? sectionCard('Query parameters', request.queryParams) : null,
        (request.headers ?? []).length ? sectionCard('Headers', request.headers) : null,

        request.body
          ? h('div', { class: 'card' },
              h('div', { class: 'card-head' },
                h('h3', { text: 'Body' }),
                h('div', { class: 'spacer' }),
                request.body.contentType ? pill(request.body.contentType) : null),
              h('div', { class: 'card-body', style: { padding: 0 } }, requestBody(request.body)))
          : null,

        (request.assertions ?? []).length
          ? h('div', { class: 'card' },
              h('div', { class: 'card-head' },
                h('h3', { text: 'Assertions' }),
                h('div', { class: 'spacer' }),
                pill(String(request.assertions.length))),
              h('div', {}, request.assertions.map((a) => assertionRow(a, null))))
          : null,

        (request.captures ?? []).length
          ? h('div', { class: 'card' },
              h('div', { class: 'card-head' }, h('h3', { text: 'Captures' })),
              kv(request.captures.map((c) => [c.name, `${c.source ?? 'body'}${c.path ? ` → ${c.path}` : ''}`])))
          : null,

        (s.parsed?.variables ?? []).length
          ? h('div', { class: 'card' },
              h('div', { class: 'card-head' }, h('h3', { text: 'File variables' })),
              kv(s.parsed.variables.map((v) => [v.name, withVars(v.value)])))
          : null,
      ),
    );
  }

  function sectionCard(title, rows) {
    return h('div', { class: 'card' },
      h('div', { class: 'card-head' },
        h('h3', { text: title }),
        h('div', { class: 'spacer' }),
        pill(String(rows.length))),
      kv(rows.map((r) => [r.key, withVars(r.value)])));
  }

  /** Render `{{interpolations}}` as highlighted spans inside plain text. */
  function withVars(text) {
    const value = String(text ?? '');
    const parts = value.split(/(\{\{[^}]*\}\})/g).filter((part) => part !== '');
    if (parts.length <= 1) return h('span', { text: value });
    return h('span', {}, parts.map((part) =>
      part.startsWith('{{')
        ? h('span', { class: 'tok-var', text: part })
        : h('span', { text: part })));
  }

  function requestBody(body) {
    if (body.graphql) {
      return h('div', { class: 'stack', style: { padding: '12px' } },
        codeBlock(body.graphql, { lang: 'text', wrap: true }),
        body.variables ? codeBlock(prettyJSON(body.variables) ?? body.variables, { lang: 'json' }) : null);
    }
    const raw = body.raw ?? '';
    const pretty = prettyJSON(raw);
    if (pretty === null) return codeBlock(raw, { lang: 'text', wrap: true });
    const pre = h('pre', { class: 'code', style: { maxHeight: '46vh' } });
    pre.innerHTML = highlightJson(pretty);
    return pre;
  }

  function assertionRow(assertion, actualOverride) {
    const passed = assertion.passed;
    const mark = passed === false ? '✕' : passed ? '✓' : '·';
    const actual = actualOverride ?? assertion.actual;
    return h('div', { class: ['assertion', passed === false ? 'fail' : passed ? 'pass' : ''].filter(Boolean) },
      h('span', { class: 'mark', text: mark }),
      h('div', { class: 'grow' },
        h('div', { class: 'expr' },
          h('span', { class: 'tok-key', text: String(assertion.subject ?? '') }),
          h('span', { class: 'tok-op', text: ` ${assertion.operator} ` }),
          h('span', { class: 'tok-str', text: stringify(assertion.expected) })),
        assertion.message ? h('div', { class: 'actual', text: assertion.message }) : null,
        passed === false && actual !== undefined && actual !== null
          ? h('div', { class: 'actual', text: `actual: ${stringify(actual)}` })
          : null));
  }

  function stringify(value) {
    if (value === null || value === undefined) return '';
    if (typeof value === 'object') {
      try { return JSON.stringify(value); } catch { return String(value); }
    }
    return String(value);
  }

  /* ---- response tab ------------------------------------------------------- */

  function renderResponse(s) {
    clear(bodyEl);
    bodyEl.classList.remove('flush');

    if (s.running.active) {
      const total = s.running.total || 0;
      const index = s.running.index || 0;
      bodyEl.append(
        h('div', { class: 'stack', style: { maxWidth: '520px', margin: '48px auto' } },
          h('div', { class: 'row' },
            h('span', { class: 'spinner' }),
            h('strong', { text: s.running.current ? `Running ${s.running.current}…` : 'Running…' })),
          total ? progressBar(index / total) : null,
          total ? h('div', { class: 'muted mono', text: `${Math.min(index, total)} / ${total} requests` }) : null));
      return;
    }

    const run = s.lastRun;
    if (!run || !(run.results ?? []).length) {
      bodyEl.append(empty({
        glyph: '↵',
        title: 'No response yet',
        message: 'Run the selected request with ⌘↵, or the whole file with ⌘⇧↵. Responses, assertions and captures land here.',
        actions: [
          btn('Run request', { variant: 'primary', kbd: '⌘↵', disabled: !s.selectedRequest, onClick: () => actions.runRequest() }),
          btn('Run file', { kbd: '⌘⇧↵', disabled: !s.selectedFile, onClick: () => actions.runFile() }),
        ],
      }));
      return;
    }

    const result = s.selectedResult ?? run.results[0];
    bodyEl.append(
      h('div', { class: 'stack' },
        h('div', { class: 'card' },
          h('div', { class: 'card-body' },
            statGrid([
              stat('Status', result?.response ? String(result.response.statusCode) : '—',
                toneForStatus(result?.response?.statusCode ?? 0)),
              stat('Duration', formatDuration(result?.response?.duration ?? result?.duration ?? 0)),
              stat('Size', formatBytes(result?.response?.size ?? 0)),
              stat('Assertions', assertionSummary(result), assertionTone(result)),
            ]),
            run.results.length > 1
              ? h('div', { class: 'row wrap', style: { marginTop: '12px', gap: '6px' } },
                  h('span', { class: 'section-title', style: { margin: 0 }, text: 'Run' }),
                  pill(`${run.passed} passed`, 'success'),
                  run.failed ? pill(`${run.failed} failed`, 'danger') : null,
                  run.skipped ? pill(`${run.skipped} skipped`) : null,
                  pill(formatDuration(run.duration)),
                  h('div', { class: 'spacer' }),
                  ...run.results.map((r) =>
                    h('button', {
                      class: ['btn', 'sm', r === result ? 'primary' : ''].filter(Boolean),
                      title: `${r.name} — ${r.passed ? 'passed' : r.skipped ? 'skipped' : 'failed'}`,
                      onClick: () => actions.selectResult(r),
                    },
                      methodBadge(r.request?.method ?? 'GET'),
                      h('span', { text: truncate(r.name || 'request', 26) }))))
              : null)),

        result ? resultCard(s, result) : null,
      ),
    );
  }

  function assertionSummary(result) {
    const all = result?.assertions ?? [];
    if (!all.length) return '—';
    return `${all.filter((a) => a.passed).length}/${all.length}`;
  }

  function assertionTone(result) {
    const all = result?.assertions ?? [];
    if (!all.length) return '';
    return all.every((a) => a.passed) ? 'success' : 'danger';
  }

  function toneForStatus(code) {
    const tone = statusTone(code);
    if (tone === 's2' || tone === 's3') return 'success';
    if (tone === 's4') return 'warning';
    if (tone === 's5') return 'danger';
    return '';
  }

  function resultCard(s, result) {
    const subTabs = [...RESPONSE_TABS];
    if ((result.sseEvents ?? []).length) subTabs.push({ id: 'sse', label: 'SSE', count: result.sseEvents.length });

    return h('div', { class: 'card' },
      tabs(subTabs, s.responseTab, (id) => set({ responseTab: id })),
      h('div', {}, responseTabBody(s, result)));
  }

  function responseTabBody(s, result) {
    if (result.error && !result.response) {
      return h('div', { class: 'row', style: { padding: '12px' } },
        pill('error', 'danger'),
        h('code', { class: 'mono grow', style: { overflowWrap: 'anywhere' }, text: result.error }));
    }

    switch (s.responseTab) {
      case 'headers': {
        const responseHeaders = Object.entries(result.response?.headers ?? {}).sort(([a], [b]) => a.localeCompare(b));
        const requestHeaders = Object.entries(result.request?.headers ?? {}).sort(([a], [b]) => a.localeCompare(b));
        return h('div', { class: 'stack' },
          h('div', {},
            h('div', { class: 'section-title', style: { padding: '10px 12px 6px' }, text: `Response headers (${responseHeaders.length})` }),
            responseHeaders.length ? kv(responseHeaders) : h('div', { class: 'muted', style: { padding: '0 12px 10px' }, text: 'None' })),
          requestHeaders.length
            ? h('div', {},
                h('div', { class: 'section-title', style: { padding: '10px 12px 6px' }, text: `Request headers sent (${requestHeaders.length})` }),
                kv(requestHeaders))
            : null);
      }

      case 'assertions':
        if (!(result.assertions ?? []).length) {
          return h('div', { class: 'muted', style: { padding: '14px' }, text: 'This request has no assertions.' });
        }
        return h('div', {}, result.assertions.map((a) => assertionRow(a, a.actual)));

      case 'timing':
        return h('div', { style: { padding: '12px' } },
          statGrid([
            stat('Total', formatDuration(result.duration ?? 0), 'accent'),
            stat('Response', formatDuration(result.response?.duration ?? 0)),
            stat('Size', formatBytes(result.response?.size ?? 0)),
            stat('Status', String(result.response?.statusCode ?? '—'), toneForStatus(result.response?.statusCode ?? 0)),
          ]),
          result.response?.status
            ? h('p', { class: 'muted mono', style: { margin: '10px 0 0' }, text: result.response.status })
            : null);

      case 'captures': {
        const entries = Object.entries(result.captures ?? {});
        if (!entries.length) {
          return h('div', { class: 'muted', style: { padding: '14px' }, text: 'No values were captured by this request.' });
        }
        return h('div', {},
          kv(entries.map(([key, value]) => [key, stringify(value)])),
          h('div', { style: { padding: '10px 12px' } },
            btn('Copy as JSON', {
              size: 'sm',
              onClick: () => actions.copyText(JSON.stringify(result.captures, null, 2), 'Captures copied'),
            })));
      }

      case 'sse':
        return h('div', {},
          (result.sseEvents ?? []).map((event) =>
            h('div', { class: 'assertion' },
              h('span', { class: 'mark', text: '⇢' }),
              h('div', { class: 'grow' },
                h('div', { class: 'expr' },
                  h('span', { class: 'tok-meta', text: event.type || 'message' }),
                  event.id ? h('span', { class: 'muted', text: ` id=${event.id}` }) : null),
                h('div', { class: 'actual', text: truncate(event.data, 500) })))));

      case 'body':
      default:
        return responseBody(s, result);
    }
  }

  function responseBody(s, result) {
    const body = result.response?.body ?? '';
    if (!body) return h('div', { class: 'muted', style: { padding: '14px' }, text: 'Empty response body.' });

    const pretty = prettyJSON(body);
    const text = s.compactBody && pretty !== null ? JSON.stringify(JSON.parse(body)) : (pretty ?? body);
    const pre = h('pre', { class: 'code', style: { maxHeight: '58vh' } });
    pre.innerHTML = pretty !== null && !s.compactBody ? highlightJson(text) : escapeHtml(text);

    return h('div', {},
      h('div', { class: 'row', style: { padding: '8px 12px', gap: '8px' } },
        statusCode(result.response?.statusCode ?? 0),
        h('span', { class: 'muted mono ell', text: truncate(result.response?.status ?? '', 60) }),
        h('div', { class: 'spacer' }),
        h('span', { class: 'muted mono', text: formatBytes(result.response?.size ?? body.length) }),
        h('span', { class: 'muted mono', text: `${formatNumber(text.split('\n').length)} lines` }),
        pretty !== null
          ? btn(s.compactBody ? 'Pretty' : 'Compact', {
              size: 'sm',
              variant: 'ghost',
              onClick: () => set({ compactBody: !s.compactBody }),
            })
          : null,
        btn('Copy', { size: 'sm', variant: 'ghost', onClick: () => actions.copyText(text, 'Body copied') })),
      pre);
  }

  /* ---- source tab --------------------------------------------------------- */

  function renderSource(s) {
    bodyEl.classList.add('flush');
    if (bodyEl.firstChild !== sourceRoot) {
      clear(bodyEl);
      bodyEl.append(sourceRoot);
    }

    sourcePathEl.textContent = s.source.path ?? '';
    clear(sourceDirtyEl);
    if (s.source.dirty) sourceDirtyEl.append(pill('unsaved changes', 'warning'));
    saveBtn.disabled = !s.source.dirty;
    revertBtn.disabled = !s.source.path;

    if (editorPath !== s.source.path) {
      editor.setValue(s.source.text ?? '');
      editorPath = s.source.path;
    } else if (!editorFocused() && editor.getValue() !== (s.source.text ?? '')) {
      editor.setValue(s.source.text ?? '');
    }
  }

  function editorFocused() {
    const active = document.activeElement;
    return Boolean(active && editor.el.contains(active));
  }

  return {
    update(s) {
      renderHead(s);
      renderTabs(s);
      renderBody(s);
    },
    focusEditor() {
      set({ detailTab: 'source' });
      editor.focus();
    },
    destroy() {
      editor.destroy();
    },
  };
}
