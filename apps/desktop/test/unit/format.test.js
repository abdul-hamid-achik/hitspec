import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  errorMessage, formatBytes, formatDuration, formatNumber, formatPercent, formatRate,
  fuzzyScore, matches, methodClass, prettyJSON, relativeTime, statusTone, truncate,
} from '../../src/renderer/src/format.js';

test('formatBytes scales through the units', () => {
  assert.equal(formatBytes(0), '0 B');
  assert.equal(formatBytes(512), '512 B');
  assert.equal(formatBytes(1024), '1.00 KB');
  assert.equal(formatBytes(1536), '1.50 KB');
  assert.equal(formatBytes(5 * 1024 * 1024), '5.00 MB');
  assert.equal(formatBytes(Number.NaN), '0 B');
});

test('formatDuration picks a sensible unit', () => {
  assert.equal(formatDuration(0), '0 ms');
  assert.equal(formatDuration(0.4), '400 µs');
  assert.equal(formatDuration(12), '12 ms');
  assert.equal(formatDuration(250), '250 ms');
  assert.equal(formatDuration(1500), '1.50 s');
  assert.equal(formatDuration(90_000), '1m 30s');
  assert.equal(formatDuration('nope'), '—');
});

test('formatPercent treats Go rates as 0..1 ratios', () => {
  assert.equal(formatPercent(1), '100.0%');
  assert.equal(formatPercent(0.995), '99.5%');
  assert.equal(formatPercent(0), '0.0%');
});

test('formatRate and formatNumber stay readable', () => {
  assert.equal(formatRate(0), '0');
  assert.equal(formatRate(4.25), '4.25');
  assert.equal(formatRate(1234.5), '1,235');
  assert.equal(formatNumber(1234567), '1,234,567');
});

test('statusTone buckets status codes', () => {
  assert.equal(statusTone(200), 's2');
  assert.equal(statusTone(301), 's3');
  assert.equal(statusTone(404), 's4');
  assert.equal(statusTone(503), 's5');
  assert.equal(statusTone(0), '');
});

test('prettyJSON returns null for non-JSON', () => {
  assert.equal(prettyJSON('{"a":1}'), '{\n  "a": 1\n}');
  assert.equal(prettyJSON('not json'), null);
  assert.equal(prettyJSON(''), null);
});

test('relativeTime describes the past', () => {
  const now = Date.parse('2026-09-27T12:00:00Z');
  assert.equal(relativeTime('2026-09-27T11:59:57Z', now), 'just now');
  assert.equal(relativeTime('2026-09-27T11:55:00Z', now), '5 minutes ago');
  assert.equal(relativeTime('2026-09-27T10:00:00Z', now), '2 hours ago');
  assert.equal(relativeTime('2026-09-26T12:00:00Z', now), '1 day ago');
  assert.equal(relativeTime(null), '—');
});

test('truncate keeps short strings intact', () => {
  assert.equal(truncate('abc', 10), 'abc');
  assert.equal(truncate('abcdefghij', 5), 'abcd…');
});

test('methodClass maps HTTP verbs to css classes', () => {
  assert.equal(methodClass('get'), 'get');
  assert.equal(methodClass('DELETE'), 'delete');
  assert.equal(methodClass(''), '');
});

test('errorMessage unwraps every payload shape', () => {
  assert.equal(errorMessage('boom'), 'boom');
  assert.equal(errorMessage({ message: 'bad' }), 'bad');
  assert.equal(errorMessage({ error: 'worse' }), 'worse');
  assert.equal(errorMessage(null), 'Unknown error');
});

test('matches is a case-insensitive substring filter', () => {
  assert.equal(matches('api/users.http', 'USERS'), true);
  assert.equal(matches('api/users.http', 'posts'), false);
  assert.equal(matches('anything', ''), true);
});

test('fuzzyScore rewards prefixes and contiguity', () => {
  const prefix = fuzzyScore('runFile', 'run');
  const scattered = fuzzyScore('runFile', 'rfl');
  assert.ok(prefix > scattered, `${prefix} should beat ${scattered}`);
  assert.equal(fuzzyScore('runFile', 'zzz'), -1);
  assert.equal(fuzzyScore('runFile', ''), 0);
});
