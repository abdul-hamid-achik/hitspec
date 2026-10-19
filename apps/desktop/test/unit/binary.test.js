import assert from 'node:assert/strict';
import path from 'node:path';
import { test } from 'node:test';

import { binaryName, candidatePaths, resolveBinary, targetId } from '../../src/main/binary.js';

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
  assert.deepEqual(candidates.map((c) => c.source), ['HITSPEC_BIN', 'bundled', 'desktop-build', 'desktop-build', 'repo-bin']);
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
  assert.deepEqual(candidates.map((c) => c.source), ['desktop-build', 'desktop-build', 'repo-bin']);
  assert.equal(candidates[2].path, path.join('/repo', 'bin', 'hitspec'));
});

test('targetId matches the electron-builder ${os}-${arch} spelling', () => {
  assert.equal(targetId('darwin', 'arm64'), 'mac-arm64');
  assert.equal(targetId('darwin', 'x64'), 'mac-x64');
  assert.equal(targetId('win32', 'x64'), 'win-x64');
  assert.equal(targetId('linux', 'x64'), 'linux-x64');
});

test('development lookup prefers the per-target build for the running CPU', () => {
  const dev = (arch) => candidatePaths({ platform: 'darwin', arch, isPackaged: false, appRoot: APP_ROOT, env: {} });
  assert.equal(dev('arm64')[0].path, path.join(APP_ROOT, 'build', 'bin', 'mac-arm64', 'hitspec'));
  assert.equal(dev('x64')[0].path, path.join(APP_ROOT, 'build', 'bin', 'mac-x64', 'hitspec'));
  // The flat copy is only a fallback behind the per-target one.
  assert.equal(dev('arm64')[1].path, path.join(APP_ROOT, 'build', 'bin', 'hitspec'));
});

test('a packaged app resolves the binary electron-builder placed in its resources', () => {
  const c = candidatePaths({ platform: 'win32', arch: 'x64', isPackaged: true, resourcesPath: 'C:\\app\\resources', appRoot: APP_ROOT, env: {} });
  assert.equal(path.basename(c[0].path), 'hitspec.exe');
  assert.equal(c[0].source, 'bundled');
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
  assert.equal(result.tried.length, 4); // HITSPEC_BIN, per-target build, flat build, repo bin
});
