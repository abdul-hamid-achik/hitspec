import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';

import { DEFAULTS, Settings } from '../../src/main/settings.js';

function tempSettings() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-settings-'));
  return { dir, store: new Settings(path.join(dir, 'settings.json')) };
}

test('load() falls back to defaults when the file is missing', () => {
  const { store } = tempSettings();
  const data = store.load();
  assert.equal(data.theme, DEFAULTS.theme);
  assert.deepEqual(data.recents, []);
  assert.equal(data.backend.watch, true);
});

test('load() survives a corrupt file', () => {
  const { dir, store } = tempSettings();
  fs.writeFileSync(path.join(dir, 'settings.json'), '{ not json', 'utf8');
  const data = store.load();
  assert.equal(data.theme, DEFAULTS.theme);
});

test('set() persists atomically and merges nested backend options', () => {
  const { dir, store } = tempSettings();
  store.load();
  store.set('theme', 'dracula');
  store.set({ backend: { readOnly: true } });

  const file = path.join(dir, 'settings.json');
  const onDisk = JSON.parse(fs.readFileSync(file, 'utf8'));
  assert.equal(onDisk.theme, 'dracula');
  assert.equal(onDisk.backend.readOnly, true);
  // untouched keys survive the merge
  assert.equal(onDisk.backend.watch, true);
  assert.equal(store.get('theme'), 'dracula');
  assert.deepEqual(fs.readdirSync(dir).filter((f) => f.endsWith('.tmp')), []);
});

test('addRecent() dedupes, orders most-recent-first and caps the list', () => {
  const { store } = tempSettings();
  store.load();
  for (let i = 0; i < 15; i++) store.addRecent(`/tmp/ws-${i}`);
  const recents = store.get('recents');
  assert.equal(recents.length, 12);
  assert.equal(recents[0].path, path.resolve('/tmp/ws-14'));
  assert.equal(store.get('lastWorkspace'), path.resolve('/tmp/ws-14'));

  store.addRecent('/tmp/ws-3');
  assert.equal(store.get('recents')[0].path, path.resolve('/tmp/ws-3'));
  assert.equal(store.get('recents').filter((r) => r.path === path.resolve('/tmp/ws-3')).length, 1);
});

test('removeRecent() promotes the next entry to lastWorkspace', () => {
  const { store } = tempSettings();
  store.load();
  store.addRecent('/tmp/one');
  store.addRecent('/tmp/two');
  assert.equal(store.get('lastWorkspace'), path.resolve('/tmp/two'));

  store.removeRecent('/tmp/two');
  assert.deepEqual(store.get('recents').map((r) => r.path), [path.resolve('/tmp/one')]);
  assert.equal(store.get('lastWorkspace'), path.resolve('/tmp/one'));
});
