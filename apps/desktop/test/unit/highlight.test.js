import assert from 'node:assert/strict';
import { test } from 'node:test';

import { escapeHtml, highlightHttp, highlightJson } from '../../src/renderer/src/highlight.js';

const SOURCE = [
  '@baseUrl = https://api.example.com',
  '',
  '### List users',
  '# @name listUsers',
  '# a plain comment',
  '',
  'GET {{baseUrl}}/users',
  'Accept: application/json',
  '',
  '{',
  '  "q": "ada & grace",',
  '  "limit": 10,',
  '  "active": true',
  '}',
  '',
  '>>>',
  'expect status 200',
  'expect body[0].name == "Ada"',
  'expect body length >= 1',
  '<<<',
  '',
  '>>>capture',
  'userId from body[0].id',
  '<<<',
].join('\n');

test('escapeHtml neutralises markup', () => {
  assert.equal(escapeHtml('<script>alert("x")</script>'), '&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;');
  assert.equal(escapeHtml("a & b 'c'"), 'a &amp; b &#39;c&#39;');
});

test('highlightHttp never emits raw markup from the source', () => {
  const html = highlightHttp('### evil\nGET http://x/<script>\n');
  assert.ok(!html.includes('<script>'), 'raw script tag leaked');
  assert.ok(html.includes('&lt;script&gt;'), 'expected escaped tag');
});

test('highlightHttp keeps one output line per input line', () => {
  const lines = SOURCE.split('\n').length;
  // +1 for the trailing padding line that keeps the gutter aligned.
  assert.equal(highlightHttp(SOURCE).split('\n').length, lines + 1);
});

test('highlightHttp colours the language constructs', () => {
  const html = highlightHttp(SOURCE);
  assert.match(html, /class="tok-var">@baseUrl<\/span>/);
  assert.match(html, /class="tok-sep">### List users<\/span>/);
  assert.match(html, /class="tok-meta"># @name listUsers<\/span>/);
  assert.match(html, /class="tok-com"># a plain comment<\/span>/);
  assert.match(html, /class="tok-method">GET<\/span>/);
  assert.match(html, /class="tok-var">\{\{baseUrl\}\}<\/span>/);
  assert.match(html, /class="tok-hdr">Accept<\/span>/);
  assert.match(html, /class="tok-op">expect<\/span>/);
  assert.match(html, /class="tok-key">status<\/span>/);
  assert.match(html, /class="tok-op">length &gt;=<\/span>/);
  assert.match(html, /class="tok-str">&quot;Ada&quot;<\/span>/);
  assert.match(html, /class="tok-var">userId<\/span>/);
  assert.match(html, /class="tok-op"> from <\/span>/);
});

test('highlightHttp treats JSON bodies as JSON', () => {
  const html = highlightHttp(SOURCE);
  assert.match(html, /class="tok-key">&quot;q&quot;<\/span>/);
  assert.match(html, /class="tok-str">&quot;ada &amp; grace&quot;<\/span>/);
  assert.match(html, /class="tok-num">10<\/span>/);
  assert.match(html, /class="tok-bool">true<\/span>/);
});

test('highlightHttp handles empty and null input', () => {
  assert.equal(highlightHttp(''), '\n');
  assert.equal(highlightHttp(null), '\n');
});

test('highlightJson classifies tokens and escapes the rest', () => {
  const html = highlightJson('{"a": [1, 2.5, null, false], "b<c": "x&y"}');
  assert.match(html, /class="tok-key">&quot;a&quot;<\/span>/);
  assert.match(html, /class="tok-num">2\.5<\/span>/);
  assert.match(html, /class="tok-bool">null<\/span>/);
  assert.match(html, /class="tok-bool">false<\/span>/);
  assert.match(html, /&quot;b&lt;c&quot;/);
  assert.match(html, /&quot;x&amp;y&quot;/);
  assert.ok(!html.includes('<c'), 'unescaped angle bracket leaked');
});

test('highlightJson leaves plain text untouched but escaped', () => {
  assert.equal(highlightJson('hello <world>'), 'hello &lt;world&gt;');
});
