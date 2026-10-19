import assert from 'node:assert/strict';
import { test } from 'node:test';

import { diskChanged, openFileDecision } from '../../src/renderer/src/guard.js';

test('a clean buffer or a forced open never asks', () => {
  assert.equal(openFileDecision({ dirty: false, currentPath: 'a', targetPath: 'b' }), 'proceed');
  assert.equal(openFileDecision({ dirty: true, currentPath: 'a', targetPath: 'b', force: true }), 'proceed');
});

test('switching files with unsaved edits asks first', () => {
  assert.equal(openFileDecision({ dirty: true, currentPath: 'a.http', targetPath: 'b.http' }), 'confirm');
  assert.equal(openFileDecision({ dirty: true, currentPath: null, targetPath: 'b.http' }), 'confirm');
});

test('an automatic reload of the file being edited is skipped, not applied', () => {
  assert.equal(openFileDecision({ dirty: true, currentPath: 'a', targetPath: 'a', keepSelection: true }), 'skip');
  // explicit re-open of the same file (e.g. clicking it) is not an auto reload
  assert.equal(openFileDecision({ dirty: true, currentPath: 'a', targetPath: 'a' }), 'proceed');
});

test('diskChanged detects external modification and tolerates unknowns', () => {
  assert.equal(diskChanged('GET /a\n', 'GET /a\n'), false);
  assert.equal(diskChanged('GET /a\n', 'GET /b\n'), true);
  assert.equal(diskChanged(undefined, 'x'), false, 'no baseline: do not nag');
  assert.equal(diskChanged('x', null), false, 'unreadable file: let the save report the error');
});
