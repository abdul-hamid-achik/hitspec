import assert from 'node:assert/strict';
import path from 'node:path';
import { test } from 'node:test';

import { binaryName, candidatePaths, resolveBinary } from '../../src/main/binary.js';

const APP_ROOT = '/repo/apps/desktop';

test('binaryName is platform aware', () => {
  assert.equal(binaryName('darwin'), 'hitspec');
  assert.equal(binaryName('linux'), 'hitspec');
  assert.equal(binaryName('win32'), 'hitspec.exe');
});

test('candidatePaths prefers HITSPEC_BIN then the bundled binary', () => {
  const candidates = candidatePaths({
    platform: 'darwin',
    isPackaged: true,
    resourcesPath: '/app/Resources',
    appRoot: APP_ROOT,
    env: { HITSPEC_BIN: '/custom/hitspec' },
  });
  assert.deepEqual(candidates.map((c) => c.source), ['HITSPEC_BIN', 'bundled', 'desktop-build', 'repo-bin']);
  assert.equal(candidates[0].path, '/custom/hitspec');
  assert.equal(candidates[1].path, path.join('/app/Resources', 'hitspec'));
});

test('candidatePaths skips the bundled binary in development', () => {
  const candidates = candidatePaths({
    platform: 'linux',
    isPackaged: false,
    appRoot: APP_ROOT,
    env: {},
  });
  assert.deepEqual(candidates.map((c) => c.source), ['desktop-build', 'repo-bin']);
  assert.equal(candidates[1].path, path.join('/repo', 'bin', 'hitspec'));
});

test('resolveBinary returns the first candidate that exists', async () => {
  const resolved = await resolveBinary({
    platform: 'darwin',
    isPackaged: false,
    appRoot: APP_ROOT,
    env: {},
    exists: async (p) => p === path.join(APP_ROOT, 'build', 'bin', 'hitspec'),
    which: async () => null,
  });
  assert.equal(resolved.source, 'desktop-build');
});

test('resolveBinary falls back to PATH', async () => {
  const resolved = await resolveBinary({
    platform: 'darwin',
    isPackaged: false,
    appRoot: APP_ROOT,
    env: {},
    exists: async () => false,
    which: async () => '/usr/local/bin/hitspec',
  });
  assert.equal(resolved.path, '/usr/local/bin/hitspec');
  assert.equal(resolved.source, 'PATH');
});

test('resolveBinary reports every location it tried', async () => {
  const result = await resolveBinary({
    platform: 'darwin',
    isPackaged: false,
    appRoot: APP_ROOT,
    env: { HITSPEC_BIN: '/nope/hitspec' },
    exists: async () => false,
    which: async () => null,
  });
  assert.ok(result.error, 'expected an error');
  assert.match(result.error, /HITSPEC_BIN/);
  assert.match(result.error, /task build/);
  assert.equal(result.tried.length, 3);
});
