import { h, clear } from './dom.js';
import { matches } from './format.js';

/** Push state into a form control without stealing focus or moving the caret. */
export function syncInput(input, value) {
  const next = String(value ?? '');
  if (input.value !== next && document.activeElement !== input) input.value = next;
}

/**
 * Scrollable checkbox list of workspace files.
 * An empty selection means "every file", which the Go endpoints also assume.
 */
export function filePicker({ files, selected, onChange, height = '180px' }) {
  const search = h('input', { class: 'input', type: 'search', placeholder: 'Filter files…', ariaLabel: 'Filter files' });
  const list = h('div', { class: 'tree scroll', style: { height, border: '1px solid var(--border-soft)', borderRadius: 'var(--radius)' } });
  const summary = h('span', { class: 'muted', style: { fontSize: '11px' } });
  let chosen = new Set(selected ?? []);

  function paint() {
    const filter = search.value;
    const rows = files.filter((f) => matches(f.relativePath, filter));
    clear(list);
    if (!rows.length) {
      list.append(h('div', { class: 'muted', style: { padding: '10px' }, text: 'No files match.' }));
    }
    for (const file of rows) {
      const box = h('input', { type: 'checkbox', checked: chosen.has(file.relativePath) });
      box.addEventListener('change', () => {
        if (box.checked) chosen.add(file.relativePath);
        else chosen.delete(file.relativePath);
        summary.textContent = label();
        onChange([...chosen]);
      });
      list.append(
        h('label', { class: 'tree-row', style: { cursor: 'pointer' } },
          box,
          h('span', { class: 'label mono', text: file.relativePath }),
          file.requestCount ? h('span', { class: 'badge-count', text: String(file.requestCount) }) : null),
      );
    }
    summary.textContent = label();
  }

  function label() {
    return chosen.size ? `${chosen.size} selected` : `all ${files.length} files`;
  }

  search.addEventListener('input', paint);
  paint();

  return {
    el: h('div', { class: 'stack-sm' },
      h('div', { class: 'row' }, search, summary),
      list),
    selection: () => [...chosen],
  };
}

/**
 * Inline SVG line chart. No charting dependency: two series (throughput and
 * p95 latency) drawn on a shared time axis with independent scales.
 *
 * @param {{at:string, rps:number, p95:number, errors:number}[]} series
 */
export function lineChart(series, { height = 130, width = 720 } = {}) {
  const svg = h('div', { class: 'chart-wrap' });
  const points = (series ?? []).slice(-240);

  if (points.length < 2) {
    return h('div', { class: 'muted', style: { padding: '18px', textAlign: 'center' }, text: 'Waiting for metrics…' });
  }

  const pad = { top: 8, right: 8, bottom: 16, left: 34 };
  const innerW = width - pad.left - pad.right;
  const innerH = height - pad.top - pad.bottom;
  const maxRps = Math.max(1, ...points.map((p) => p.rps ?? 0));
  const maxP95 = Math.max(1, ...points.map((p) => p.p95 ?? 0));

  const x = (i) => pad.left + (i / (points.length - 1)) * innerW;
  const yRps = (v) => pad.top + innerH - (v / maxRps) * innerH;
  const yP95 = (v) => pad.top + innerH - (v / maxP95) * innerH;

  const path = (accessor, scale) =>
    points.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${scale(accessor(p) ?? 0).toFixed(1)}`).join(' ');

  const area = `${path((p) => p.rps, yRps)} L${x(points.length - 1).toFixed(1)},${(pad.top + innerH).toFixed(1)} L${x(0).toFixed(1)},${(pad.top + innerH).toFixed(1)} Z`;

  const NS = 'http://www.w3.org/2000/svg';
  const make = (tag, attrs = {}, text) => {
    const node = document.createElementNS(NS, tag);
    for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, String(value));
    if (text !== undefined) node.textContent = text;
    return node;
  };

  const chart = make('svg', { class: 'chart', viewBox: `0 0 ${width} ${height}`, preserveAspectRatio: 'none' });
  for (let i = 0; i <= 3; i++) {
    const gy = pad.top + (innerH / 3) * i;
    chart.appendChild(make('line', { class: 'grid-line', x1: pad.left, x2: width - pad.right, y1: gy, y2: gy }));
    chart.appendChild(make('text', { class: 'axis', x: 2, y: gy + 3 }, Math.round(maxRps * (1 - i / 3)).toString()));
  }
  chart.appendChild(make('path', { class: 'area-rps', d: area }));
  chart.appendChild(make('path', { class: 'line-rps', d: path((p) => p.rps, yRps) }));
  chart.appendChild(make('path', { class: 'line-p95', d: path((p) => p.p95, yP95) }));
  chart.appendChild(make('text', { class: 'axis', x: pad.left, y: height - 4 }, 'start'));
  chart.appendChild(make('text', { class: 'axis', x: width - pad.right - 34, y: height - 4 }, 'now'));

  svg.append(chart,
    h('div', { class: 'legend', style: { marginTop: '6px' } },
      h('span', {}, h('i', { style: { background: 'var(--accent)' } }), `rps (peak ${maxRps.toFixed(0)})`),
      h('span', {}, h('i', { style: { background: 'var(--warning)' } }), `p95 ms (peak ${maxP95.toFixed(0)})`)));
  return svg;
}
