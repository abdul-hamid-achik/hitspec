/** Pure formatting helpers shared by every view. */

export function formatBytes(bytes) {
  const n = Number(bytes);
  if (!Number.isFinite(n) || n <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)));
  const value = n / 1024 ** i;
  return `${i === 0 ? value : value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[i]}`;
}

export function formatDuration(ms) {
  const n = Number(ms);
  if (!Number.isFinite(n)) return '—';
  if (n === 0) return '0 ms';
  if (n < 1) return `${Math.round(n * 1000)} µs`;
  if (n < 1000) return `${n < 10 ? n.toFixed(1) : Math.round(n)} ms`;
  if (n < 60_000) return `${(n / 1000).toFixed(n < 10_000 ? 2 : 1)} s`;
  const mins = Math.floor(n / 60_000);
  const secs = Math.round((n % 60_000) / 1000);
  return `${mins}m ${secs}s`;
}

export function formatNumber(n, digits = 0) {
  const value = Number(n);
  if (!Number.isFinite(value)) return '—';
  return value.toLocaleString('en-US', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

export function formatRate(rps) {
  const n = Number(rps);
  if (!Number.isFinite(n)) return '—';
  if (n === 0) return '0';
  if (n < 10) return n.toFixed(2);
  return formatNumber(Math.round(n));
}

export function formatPercent(ratio, digits = 1) {
  const n = Number(ratio);
  if (!Number.isFinite(n)) return '—';
  // The Go API reports success/error rates as a 0..1 ratio (see
  // packages/stress/metrics.go), so scale rather than guess.
  return `${(n * 100).toFixed(digits)}%`;
}

/** CSS class bucket for an HTTP status code. */
export function statusTone(code) {
  const n = Number(code);
  if (!Number.isFinite(n) || n === 0) return '';
  const bucket = Math.floor(n / 100);
  return bucket >= 2 && bucket <= 5 ? `s${bucket}` : '';
}

export function relativeTime(iso, now = Date.now()) {
  if (!iso) return '—';
  const then = new Date(iso).getTime();
  if (!Number.isFinite(then)) return '—';
  const diff = now - then;
  const abs = Math.abs(diff);
  const suffix = diff >= 0 ? 'ago' : 'from now';
  const units = [
    ['second', 1000],
    ['minute', 60_000],
    ['hour', 3_600_000],
    ['day', 86_400_000],
  ];
  if (abs < 5000) return diff >= 0 ? 'just now' : 'in a moment';
  for (let i = units.length - 1; i >= 0; i--) {
    const [label, size] = units[i];
    if (abs >= size || i === 0) {
      const value = Math.round(abs / size);
      return `${value} ${label}${value === 1 ? '' : 's'} ${suffix}`;
    }
  }
  return '—';
}

export function formatClock(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return String(iso);
  return d.toLocaleTimeString('en-GB', { hour12: false });
}

export function formatDateTime(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return String(iso);
  return d.toLocaleString('en-GB', { hour12: false });
}

export function truncate(text, max = 120) {
  const s = String(text ?? '');
  return s.length > max ? `${s.slice(0, max - 1)}…` : s;
}

/** Pretty-print JSON; returns null when the input is not valid JSON. */
export function prettyJSON(text, indent = 2) {
  if (typeof text !== 'string' || !text.trim()) return null;
  try {
    return JSON.stringify(JSON.parse(text), null, indent);
  } catch {
    return null;
  }
}

export function isJSON(text) {
  return prettyJSON(text) !== null;
}

/** Best-effort body rendering: pretty JSON, otherwise the raw text. */
export function renderBody(body) {
  if (body === null || body === undefined) return '';
  const text = typeof body === 'string' ? body : JSON.stringify(body);
  return prettyJSON(text) ?? text;
}

export function methodClass(method) {
  const m = String(method ?? '').toUpperCase();
  if (m === 'DELETE') return 'delete';
  return m.toLowerCase();
}

/** Normalise a server error payload into a display string. */
export function errorMessage(err) {
  if (!err) return 'Unknown error';
  if (typeof err === 'string') return err;
  if (err.message) return err.message;
  if (err.error) return typeof err.error === 'string' ? err.error : JSON.stringify(err.error);
  return JSON.stringify(err);
}

/** Case-insensitive substring filter used by the sidebar and palette. */
export function matches(haystack, needle) {
  if (!needle) return true;
  return String(haystack ?? '').toLowerCase().includes(String(needle).toLowerCase());
}

/** Simple subsequence fuzzy score; higher is better, -1 means no match. */
export function fuzzyScore(text, query) {
  const t = String(text ?? '').toLowerCase();
  const q = String(query ?? '').toLowerCase().trim();
  if (!q) return 0;
  let score = 0;
  let ti = 0;
  let streak = 0;
  for (const ch of q) {
    const idx = t.indexOf(ch, ti);
    if (idx === -1) return -1;
    streak = idx === ti ? streak + 1 : 0;
    score += 1 + streak * 2 + (idx === 0 ? 3 : 0);
    ti = idx + 1;
  }
  return score;
}
