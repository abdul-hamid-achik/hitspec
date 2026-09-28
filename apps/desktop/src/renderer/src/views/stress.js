import * as actions from '../actions.js';
import { h, clear } from '../dom.js';
import { formatDuration, formatNumber, formatPercent, formatRate } from '../format.js';
import { btn, card, empty, pill, stat, statGrid, table } from '../ui.js';
import { filePicker, lineChart, syncInput } from '../widgets.js';

export function createStressView(root) {
  const head = h('div', { class: 'view-head' });
  const body = h('div', { class: 'view-body' });
  clear(root);
  root.append(h('div', { class: 'view' }, head, body));

  const durationInput = h('input', { class: 'input mono', value: '10s', style: { width: '92px' }, ariaLabel: 'Duration' });
  const rateInput = h('input', { class: 'input mono', type: 'number', min: '0', value: '50', style: { width: '92px' }, ariaLabel: 'Requests per second' });
  const vusInput = h('input', { class: 'input mono', type: 'number', min: '0', value: '0', style: { width: '92px' }, ariaLabel: 'Virtual users' });
  const maxVusInput = h('input', { class: 'input mono', type: 'number', min: '0', value: '0', style: { width: '92px' }, ariaLabel: 'Maximum virtual users' });

  let picker = null;
  let pickerFiles = null;

  const syncForm = () =>
    actions.setStressForm({
      duration: durationInput.value.trim() || '10s',
      rate: Number(rateInput.value) || 0,
      vus: Number(vusInput.value) || 0,
      maxVUs: Number(maxVusInput.value) || 0,
      files: picker?.selection() ?? [],
    });

  for (const input of [durationInput, rateInput, vusInput, maxVusInput]) {
    input.addEventListener('change', syncForm);
    input.addEventListener('input', syncForm);
  }

  const liveEl = h('div', { class: 'stack' });
  const resultEl = h('div', { class: 'stack' });
  const profilesEl = h('div', { class: 'stack' });

  function update(s) {
    const form = s.stress.form;
    syncInput(durationInput, form.duration);
    syncInput(rateInput, form.rate ?? 0);
    syncInput(vusInput, form.vus ?? 0);
    syncInput(maxVusInput, form.maxVUs ?? 0);

    // The picker caches its own checkbox DOM, so rebuild it only when the
    // workspace file list actually changes.
    if (!picker || pickerFiles !== s.files) {
      pickerFiles = s.files;
      picker = filePicker({
        files: s.files ?? [],
        selected: form.files ?? [],
        onChange: (files) => actions.setStressForm({ files }),
      });
    }

    clear(head);
    head.append(
      h('h2', { text: 'Stress testing' }),
      s.stress.running
        ? pill('running', 'success')
        : pill('idle'),
      h('div', { class: 'spacer' }),
      s.stress.running
        ? btn('Stop', { variant: 'danger', onClick: () => actions.stopStress() })
        : btn('Start stress test', { variant: 'primary', busy: Boolean(s.busy.stress), onClick: () => { syncForm(); actions.startStress(); } }),
    );

    clear(body);
    if (!s.files?.length) {
      body.append(empty({ glyph: '⚡', title: 'No files to stress', message: 'Open a workspace with .http files first.' }));
      return;
    }

    body.append(
      card('Load profile',
        h('div', { class: 'stack' },
          h('div', { class: 'row wrap' },
            field('Duration', durationInput, 'Go duration, e.g. 30s or 2m'),
            field('Rate (rps)', rateInput, 'Target requests per second'),
            field('VUs', vusInput, 'Virtual users (0 = use rate)'),
            field('Max VUs', maxVusInput, 'Cap for VU ramp-up')),
          picker.el,
          h('div', { class: 'muted', style: { fontSize: '11.5px' }, text: 'Leaving the file list empty stresses every .http file in the workspace.' })),
        { actions: [btn('Save as profile', { size: 'sm', onClick: () => { syncForm(); actions.saveStressProfile(); } })] }),

      h('div', {}, liveEl),
      h('div', {}, resultEl),
      h('div', {}, profilesEl),
    );

    renderLive(s);
    renderResult(s);
    renderProfiles(s);
  }

  function field(label, control, hint) {
    return h('div', { class: 'field' },
      h('label', { text: label }),
      control,
      hint ? h('span', { class: 'muted', style: { fontSize: '10.5px' }, text: hint }) : null);
  }

  function renderLive(s) {
    clear(liveEl);
    const stats = s.stress.status?.stats ?? null;
    if (!stats && !s.stress.series.length) {
      liveEl.append(card('Live metrics', h('div', { class: 'muted', text: 'Start a stress test to stream metrics every 500ms.' })));
      return;
    }
    liveEl.append(
      card('Live metrics',
        h('div', { class: 'stack' },
          statGrid([
            stat('Elapsed', formatDuration((s.stress.status?.elapsed ?? 0) * 1000), 'accent'),
            stat('Requests', formatNumber(stats?.total ?? 0)),
            stat('RPS', formatRate(stats?.rps ?? 0)),
            stat('Errors', formatNumber(stats?.errors ?? 0), stats?.errors ? 'danger' : 'success'),
            stat('Error rate', formatPercent(stats?.errorRate ?? 0), (stats?.errorRate ?? 0) > 0 ? 'danger' : ''),
            stat('p50', `${(stats?.p50Ms ?? 0).toFixed(1)} ms`),
            stat('p95', `${(stats?.p95Ms ?? 0).toFixed(1)} ms`, 'warning'),
            stat('p99', `${(stats?.p99Ms ?? 0).toFixed(1)} ms`),
            stat('Max', `${(stats?.maxMs ?? 0).toFixed(1)} ms`),
            stat('Active VUs', String(stats?.activeVUs ?? 0)),
          ]),
          lineChart(s.stress.series))),
    );
  }

  function renderResult(s) {
    clear(resultEl);
    const result = s.stress.result;
    if (!result) return;

    resultEl.append(
      card('Last result',
        h('div', { class: 'stack' },
          statGrid([
            stat('Requests', formatNumber(result.total)),
            stat('Success', formatNumber(result.success), 'success'),
            stat('Errors', formatNumber(result.errors), result.errors ? 'danger' : ''),
            stat('Timeouts', formatNumber(result.timeouts), result.timeouts ? 'warning' : ''),
            stat('RPS', formatRate(result.rps), 'accent'),
            stat('Success rate', formatPercent(result.successRate), 'success'),
            stat('Mean', `${result.meanMs.toFixed(1)} ms`),
            stat('Std dev', `${result.stdDevMs.toFixed(1)} ms`),
          ]),
          (result.breakdown ?? []).length
            ? h('div', {},
                h('div', { class: 'section-title', text: 'Per request' }),
                table({
                  columns: [
                    { label: 'Request', class: 'ell', render: (row) => row.name },
                    { label: 'Total', class: 'num', render: (row) => formatNumber(row.total) },
                    { label: 'Success', class: 'num', render: (row) => formatNumber(row.success) },
                    { label: 'Errors', class: 'num', render: (row) => h('span', { class: row.errors ? 'danger' : 'muted', text: formatNumber(row.errors) }) },
                    { label: 'p50', class: 'num', render: (row) => `${row.p50Ms.toFixed(1)} ms` },
                    { label: 'p95', class: 'num', render: (row) => `${row.p95Ms.toFixed(1)} ms` },
                    { label: 'p99', class: 'num', render: (row) => `${row.p99Ms.toFixed(1)} ms` },
                    { label: 'Mean', class: 'num', render: (row) => `${row.meanMs.toFixed(1)} ms` },
                  ],
                  rows: result.breakdown,
                }))
            : null,
          (result.thresholds ?? []).length
            ? h('div', {},
                h('div', { class: 'section-title', text: 'Thresholds' }),
                result.thresholds.map((t) =>
                  h('div', { class: 'assertion ' + (t.passed ? 'pass' : 'fail') },
                    h('span', { class: 'mark', text: t.passed ? '✓' : '✕' }),
                    h('div', { class: 'grow' },
                      h('div', { class: 'expr', text: t.name }),
                      h('div', { class: 'actual', text: `expected ${t.expected}, actual ${t.actual}` })))))
            : null)));
  }

  function renderProfiles(s) {
    clear(profilesEl);
    const profiles = s.stress.profiles ?? [];
    if (!profiles.length) return;
    profilesEl.append(
      card('Saved profiles',
        h('div', { class: 'stack-sm' },
          profiles.map((profile) =>
            h('div', { class: 'row' },
              h('strong', { text: profile.name }),
              pill(profile.duration || '—'),
              profile.rate ? pill(`${profile.rate} rps`) : null,
              profile.vus ? pill(`${profile.vus} VUs`) : null,
              h('div', { class: 'spacer' }),
              btn('Apply', { size: 'sm', onClick: () => actions.applyStressProfile(profile) }),
              btn('✕', { size: 'sm', variant: 'ghost', title: 'Delete profile', onClick: () => actions.deleteStressProfile(profile.name) }))))),
    );
  }

  return { update };
}
