/**
 * Dependency-free syntax highlighting for `.http` sources and JSON payloads.
 * Both functions return an HTML string with every input character escaped, so
 * the result is safe to inject into the editor's highlight layer.
 */

const METHODS = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|TRACE|CONNECT)(?=\s|$)/;
const HEADER = /^([A-Za-z0-9!#$%&'*+\-.^_`|~]+)[ \t]*:(.*)$/;
const VAR_DEF = /^@([\w.$-]+)[ \t]*=[ \t]*(.*)$/;
const ASSERT = /^expect\b/;
// Longest first so `length >=` wins over `length` and `!contains` over `contains`.
const OPERATORS = [
  '!contains', '!exists', '!includes', '!in', 'contains', 'startsWith', 'endsWith',
  'matches', 'exists', 'includes', 'length', 'schema', 'snapshot', 'each', 'type', 'in',
  'length >=', 'length <=', 'length >', 'length <',
  '==', '!=', '>=', '<=', '>', '<',
].sort((a, b) => b.length - a.length);

export function escapeHtml(text) {
  return String(text ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function span(text, cls) {
  return cls ? `<span class="${cls}">${escapeHtml(text)}</span>` : escapeHtml(text);
}

/** Split a run of text on `{{interpolations}}` and colour each part. */
function withVars(text, cls) {
  const re = /\{\{[^}\n]*\}\}/g;
  let out = '';
  let last = 0;
  let m;
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) out += span(text.slice(last, m.index), cls);
    out += span(m[0], 'tok-var');
    last = m.index + m[0].length;
  }
  out += span(text.slice(last), cls);
  return out || span('', cls);
}

const JSON_TOKEN = /("(?:\\.|[^"\\])*"\s*:|"(?:\\.|[^"\\])*"|\btrue\b|\bfalse\b|\bnull\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;

/** Highlight a JSON fragment (works line-by-line or on a whole document). */
export function highlightJson(text) {
  const src = String(text ?? '');
  let out = '';
  let last = 0;
  let m;
  JSON_TOKEN.lastIndex = 0;
  while ((m = JSON_TOKEN.exec(src)) !== null) {
    if (m.index > last) out += escapeHtml(src.slice(last, m.index));
    const token = m[0];
    if (/:$/.test(token.trimEnd())) {
      const key = token.slice(0, token.lastIndexOf('"') + 1);
      const rest = token.slice(key.length);
      out += `${span(key, 'tok-key')}${escapeHtml(rest)}`;
    } else if (token.startsWith('"')) {
      out += span(token, 'tok-str');
    } else if (token === 'true' || token === 'false' || token === 'null') {
      out += span(token, 'tok-bool');
    } else {
      out += span(token, 'tok-num');
    }
    last = m.index + token.length;
  }
  out += escapeHtml(src.slice(last));
  return out;
}

function matchOperator(text) {
  for (const op of OPERATORS) {
    if (!text.startsWith(op)) continue;
    const after = text.slice(op.length);
    if (after === '' || /^\s/.test(after)) {
      const gap = /^\s*/.exec(after)[0];
      return { op, gap, rest: after.slice(gap.length) };
    }
  }
  return null;
}

function highlightAssertion(line) {
  const head = /^(\s*)expect(\s+)(.*)$/.exec(line);
  if (!head) return withVars(line);
  const [, indent, gap, rest] = head;
  let out = span(indent) + span('expect', 'tok-op') + span(gap);

  const subject = /^(\S+)(\s*)(.*)$/.exec(rest);
  if (!subject) return out + span(rest);
  out += span(subject[1], 'tok-key') + span(subject[2]);

  const operator = matchOperator(subject[3]);
  if (operator) {
    out += span(operator.op, 'tok-op') + span(operator.gap);
    return out + withVars(operator.rest, 'tok-str');
  }
  return out + withVars(subject[3], 'tok-str');
}

function highlightCapture(line) {
  const parts = line.split(/^(\s*)(\S+)(\s+from\s+)(.+)$/);
  if (parts.length < 6) return withVars(line);
  const [, indent, name, keyword, source] = parts;
  return span(indent) + span(name, 'tok-var') + span(keyword, 'tok-op') + withVars(source, 'tok-key');
}

/**
 * Highlight a `.http` document.
 * @param {string} source
 * @returns {string} escaped HTML, one output line per input line
 */
export function highlightHttp(source) {
  const lines = String(source ?? '').split('\n');
  let state = 'idle'; // idle | headers | body | assert | capture

  const out = lines.map((line) => {
    if (/^\s*###/.test(line)) {
      state = 'idle';
      return span(line, 'tok-sep');
    }
    if (/^\s*>>>\s*capture/i.test(line)) {
      state = 'capture';
      return span(line, 'tok-sep');
    }
    if (/^\s*>>>/.test(line)) {
      state = 'assert';
      return span(line, 'tok-sep');
    }
    if (/^\s*<<</.test(line)) {
      state = 'idle';
      return span(line, 'tok-sep');
    }
    if (/^\s*#\s*@/.test(line)) return span(line, 'tok-meta');
    if (/^\s*#/.test(line)) return span(line, 'tok-com');

    const varDef = line.trimStart().match(VAR_DEF);
    if (varDef && (state === 'idle' || state === 'headers')) {
      const indent = line.slice(0, line.length - line.trimStart().length);
      return `${span(indent)}${span(`@${varDef[1]}`, 'tok-var')}${span(' = ', 'tok-op')}${withVars(varDef[2], 'tok-str')}`;
    }

    const method = line.trimStart().match(METHODS);
    if (method) {
      const indent = line.slice(0, line.length - line.trimStart().length);
      const rest = line.trimStart().slice(method[0].length);
      state = 'headers';
      return `${span(indent)}${span(method[0], 'tok-method')}${withVars(rest, 'tok-url')}`;
    }

    if (state === 'assert') {
      return ASSERT.test(line.trimStart()) ? highlightAssertion(line) : withVars(line, 'tok-str');
    }
    if (state === 'capture') return highlightCapture(line);

    if (state !== 'body') {
      const header = line.trimStart().match(HEADER);
      if (header && line.trim() !== '') {
        const indent = line.slice(0, line.length - line.trimStart().length);
        state = 'headers';
        return `${span(indent)}${span(header[1], 'tok-hdr')}${span(':', 'tok-op')}${withVars(header[2], 'tok-str')}`;
      }
    }

    if (line.trim() === '') {
      if (state === 'headers') state = 'body';
      return '';
    }

    state = 'body';
    const looksJson = /[{"[\d-]/.test(line.trim()[0] ?? '');
    return looksJson ? highlightJson(line) : withVars(line);
  });

  // A trailing newline in the textarea produces one more visual line than the
  // <pre> highlight layer, so pad it to keep the gutter aligned.
  return `${out.join('\n')}\n`;
}
