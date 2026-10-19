import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';

import {
  looksLikeHitspecServe, readPidFile, reapStaleServer, removePidFile, writePidFile,
} from '../../src/main/stale.js';

const posix = { skip: process.platform === 'win32' && 'needs POSIX processes' };
const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-stale-'));

test('looksLikeHitspecServe only matches a real API server command line', () => {
  assert.equal(looksLikeHitspecServe('/Applications/hitspec Studio.app/Contents/Resources/hitspec serve --api-only --host 127.0.0.1 --port 5'), true);
  assert.equal(looksLikeHitspecServe('C:\\app\\resources\\hitspec.exe serve --api-only'), true);
  assert.equal(looksLikeHitspecServe('hitspec.exe'), true, 'tasklist image name');
  assert.equal(looksLikeHitspecServe('/usr/bin/vim notes.txt'), false);
  assert.equal(looksLikeHitspecServe('/opt/homebrew/bin/hitspec mcp serve --workspace /x'), false);
  assert.equal(looksLikeHitspecServe('/bin/hitspec run tests/'), false);
  assert.equal(looksLikeHitspecServe(null), false);
});

test('pid file round trip; removal respects ownership', () => {
  const file = path.join(tmp(), 'serve.pid.json');
  writePidFile(file, { pid: 4242, port: 1 });
  assert.equal(readPidFile(file).pid, 4242);
  removePidFile(file, 1);
  assert.ok(readPidFile(file), 'a newer child owns the file, leave it');
  removePidFile(file, 4242);
  assert.equal(readPidFile(file), null);
  fs.writeFileSync(file, 'garbage');
  assert.equal(readPidFile(file), null);
});

test('reap kills a live stale hitspec server and clears the file', async () => {
  const file = path.join(tmp(), 'serve.pid.json');
  writePidFile(file, { pid: 31337 });
  const alive = new Set([31337]);
  const signals = [];
  const res = await reapStaleServer(file, {
    isAlive: (pid) => alive.has(pid),
    getCommand: async () => '/x/hitspec serve --api-only --port 9',
    kill: (pid, sig) => { signals.push(sig); if (sig === 'SIGTERM') alive.delete(pid); },
  });
  assert.deepEqual(res, { reaped: true, pid: 31337 });
  assert.deepEqual(signals, ['SIGTERM']);
  assert.equal(readPidFile(file), null);
});

test('reap escalates to SIGKILL when the server ignores SIGTERM', async () => {
  const file = path.join(tmp(), 'serve.pid.json');
  writePidFile(file, { pid: 31338 });
  const alive = new Set([31338]);
  const signals = [];
  await reapStaleServer(file, {
    graceMs: 120,
    isAlive: (pid) => alive.has(pid),
    getCommand: async () => '/x/hitspec serve --api-only',
    kill: (pid, sig) => { signals.push(sig); if (sig === 'SIGKILL') alive.delete(pid); },
  });
  assert.deepEqual(signals, ['SIGTERM', 'SIGKILL']);
});

test('reap never signals a process that reused the pid or is already gone', async () => {
  for (const [alive, command, reason] of [
    [true, '/usr/bin/vim', /reused/],
    [true, null, /reused/],
    [false, '/x/hitspec serve --api-only', /not running/],
  ]) {
    const file = path.join(tmp(), 'serve.pid.json');
    writePidFile(file, { pid: 555 });
    const signals = [];
    const res = await reapStaleServer(file, {
      isAlive: () => alive,
      getCommand: async () => command,
      kill: (_p, sig) => signals.push(sig),
    });
    assert.equal(res.reaped, false);
    assert.match(res.reason, reason);
    assert.deepEqual(signals, []);
    assert.equal(readPidFile(file), null, 'stale record is cleaned up');
  }
  assert.deepEqual(await reapStaleServer(path.join(tmp(), 'missing.json')), { reaped: false, reason: 'no pidfile' });
});

test('reap really kills an orphaned process (real pid, real ps lookup)', posix, async () => {
  const dir = tmp();
  // A stand-in whose command line looks like `<bin>/hitspec serve --api-only`.
  const bin = path.join(dir, 'hitspec');
  fs.writeFileSync(bin, '#!/usr/bin/env node\nsetInterval(() => {}, 1000);\n', { mode: 0o755 });
  const child = spawn(bin, ['serve', '--api-only', '--port', '1'], { stdio: 'ignore' });
  const exited = new Promise((resolve) => child.once('exit', (code, signal) => resolve(signal ?? code)));
  try {
    const file = path.join(dir, 'serve.pid.json');
    writePidFile(file, { pid: child.pid });
    // give the shebang interpreter a moment to exec so ps shows the full command
    await new Promise((r) => setTimeout(r, 300));
    const res = await reapStaleServer(file, { graceMs: 3000 });
    assert.equal(res.reaped, true, JSON.stringify(res));
    assert.equal(await Promise.race([exited, new Promise((r) => setTimeout(() => r('still running'), 3000))]), 'SIGTERM');
  } finally {
    child.kill('SIGKILL');
  }
});
