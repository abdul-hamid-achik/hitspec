import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';

import { Backend, describeSpawnError } from '../../src/main/backend.js';
import { readPidFile } from '../../src/main/stale.js';

// The stand-in server is an extensionless shebang script.
const opts = { skip: process.platform === 'win32' && 'needs a POSIX shebang executable' };

/**
 * A stand-in for `hitspec serve --api-only` that applies the same request
 * policy as the Go server (Host allowlist, no foreign Origin, bearer token,
 * JSON Content-Type) and records what it saw, so these tests pin down the
 * contract between the desktop app and the Go server without needing a Go
 * toolchain.
 */
const FAKE_SERVER = String.raw`#!/usr/bin/env node
const http = require('node:http');
const crypto = require('node:crypto');
const fs = require('node:fs');

const argv = process.argv.slice(2);
if (process.env.FAKE_REJECT_PARENT_PID && argv.includes('--parent-pid')) {
  process.stderr.write('Error: unknown flag: --parent-pid\\n');
  process.exit(1);
}
const port = Number(argv[argv.indexOf('--port') + 1]);
const token = process.env.HITSPEC_API_TOKEN;
const logFile = process.env.FAKE_LOG;
const log = (rec) => fs.appendFileSync(logFile, JSON.stringify(rec) + '\n');
log({ kind: 'start', argv, envToken: token ?? null, dyld: process.env.DYLD_INSERT_LIBRARIES ?? null, nodeOptions: process.env.NODE_OPTIONS ?? null, custom: process.env.HARMLESS_KEY ?? null });

const hostOk = (h) => h === '127.0.0.1:' + port || h === 'localhost:' + port || h === '[::1]:' + port;
const authOk = (req, url) => {
  const m = /^bearer (.+)$/i.exec(req.headers.authorization || '');
  return (m ? m[1] : url.searchParams.get('token')) === token;
};

const server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://x');
  let body = '';
  req.on('data', (c) => (body += c));
  req.on('end', () => {
    const rec = { kind: 'http', method: req.method, path: url.pathname, host: req.headers.host,
      origin: req.headers.origin ?? null, contentType: req.headers['content-type'] ?? null, hasBody: body.length > 0 };
    log(rec);
    const send = (code, obj) => { res.writeHead(code, { 'content-type': 'application/json' }); res.end(JSON.stringify(obj)); };
    if (!hostOk(req.headers.host)) return send(403, { error: 'host' });
    if (req.headers.origin) return send(403, { error: 'origin' });
    if (!authOk(req, url)) return send(401, { error: 'token' });
    const raw = url.pathname.startsWith('/api/v1/files/');
    if (body.length > 0 && !raw && !/^application\/json(;|$)/i.test(req.headers['content-type'] || '')) {
      return send(415, { error: 'content-type' });
    }
    send(200, { version: process.env.FAKE_VERSION || 'fake', echo: raw ? body : body ? JSON.parse(body) : null });
  });
});

server.on('upgrade', (req, socket) => {
  const url = new URL(req.url, 'http://x');
  log({ kind: 'ws', host: req.headers.host, origin: req.headers.origin ?? null, tokenInQuery: url.searchParams.get('token') === token });
  if (!hostOk(req.headers.host) || req.headers.origin || !authOk(req, url)) {
    socket.end('HTTP/1.1 403 Forbidden\r\n\r\n');
    return;
  }
  const accept = crypto.createHash('sha1').update(req.headers['sec-websocket-key'] + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').digest('base64');
  socket.write('HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ' + accept + '\r\n\r\n');
});

server.listen(port, '127.0.0.1');
process.on('SIGTERM', () => process.exit(0));
`;

function fakeBackend() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-backend-'));
  const bin = path.join(dir, 'hitspec');
  const logFile = path.join(dir, 'log.jsonl');
  fs.writeFileSync(bin, FAKE_SERVER, { mode: 0o755 });
  fs.writeFileSync(logFile, '');
  const backend = new Backend({
    resolveBinary: async () => ({ path: bin, source: 'test' }),
    log: { log() {}, warn() {}, error() {} },
  });
  const records = () =>
    fs.readFileSync(logFile, 'utf8').split('\n').filter(Boolean).map((l) => JSON.parse(l));
  const start = (extraEnv = {}) => backend.start(dir, { childEnv: { FAKE_LOG: logFile, ...extraEnv } });
  return { backend, records, start, dir, bin };
}

async function waitFor(fn, ms = 3000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (fn()) return true;
    await new Promise((r) => setTimeout(r, 25));
  }
  return false;
}

test('the API token goes to the child through HITSPEC_API_TOKEN, never argv', opts, async () => {
  const { backend, records, start } = fakeBackend();
  try {
    await start();
    const started = records().find((r) => r.kind === 'start');
    assert.ok(started, 'fake server should have started');
    assert.equal(started.envToken, backend.token);
    assert.ok(backend.token.length >= 20);
    assert.ok(!started.argv.includes('--api-token'), 'argv must not carry --api-token');
    assert.ok(
      !started.argv.some((a) => a.includes(backend.token)),
      'the token must not appear anywhere in argv (it is visible in ps)',
    );
    assert.deepEqual(started.argv.slice(0, 2), ['serve', '--api-only']);
    assert.equal(started.argv[started.argv.indexOf('--host') + 1], '127.0.0.1');
  } finally {
    await backend.stop();
  }
});

test('the child is told to exit with this process (--parent-pid)', opts, async () => {
  const { backend, records, start } = fakeBackend();
  try {
    await start();
    const started = records().find((r) => r.kind === 'start');
    assert.equal(started.argv[started.argv.indexOf('--parent-pid') + 1], String(process.pid));
  } finally {
    await backend.stop();
  }
});

test('a CLI that predates --parent-pid is started without it, and remembered', opts, async () => {
  const { backend, records, start } = fakeBackend();
  try {
    await start({ FAKE_REJECT_PARENT_PID: '1' });
    const starts = records().filter((r) => r.kind === 'start');
    assert.equal(starts.length, 1, 'only the retry without the flag gets as far as logging');
    assert.ok(!starts[0].argv.includes('--parent-pid'));
    assert.equal(backend.state, 'ready');
    assert.equal(backend.noParentPidFor, backend.binary);
    // the next start does not try the flag again (it would only fail once more)
    await start({ FAKE_REJECT_PARENT_PID: '1' });
    assert.equal(records().filter((r) => r.kind === 'start').length, 2);
  } finally {
    await backend.stop();
  }
});

test('requests satisfy the server Host/Origin/Content-Type policy', opts, async () => {
  const { backend, records, start } = fakeBackend();
  try {
    await start();
    assert.equal(backend.state, 'ready');

    const json = await backend.request({ method: 'POST', path: '/execute', body: { file: 'a.http' } });
    assert.equal(json.ok, true, json.error);
    assert.deepEqual(json.data.echo, { file: 'a.http' });

    const raw = await backend.request({ method: 'PUT', path: '/files/a.http', body: '### r\nGET /\n', raw: true });
    assert.equal(raw.ok, true, raw.error);

    const bodyless = await backend.request({ method: 'POST', path: '/stress/stop' });
    assert.equal(bodyless.ok, true, bodyless.error);

    const http = records().filter((r) => r.kind === 'http');
    assert.ok(http.length >= 4);
    for (const r of http) {
      assert.equal(r.host, `127.0.0.1:${backend.port}`, 'Host must be the literal loopback address and port');
      assert.equal(r.origin, null, 'the main process must not send an Origin header');
    }
    const post = http.find((r) => r.path === '/api/v1/execute');
    assert.match(post.contentType, /^application\/json/);
    const put = http.find((r) => r.path === '/api/v1/files/a.http');
    assert.match(put.contentType, /^text\/plain/);
    const stop = http.find((r) => r.path === '/api/v1/stress/stop');
    assert.equal(stop.hasBody, false);
  } finally {
    await backend.stop();
  }
});

test('the WebSocket upgrade has a loopback Host, no Origin and the token', opts, async () => {
  const { backend, records, start } = fakeBackend();
  try {
    await start();
    assert.ok(await waitFor(() => records().some((r) => r.kind === 'ws')), 'expected a WebSocket upgrade');
    const ws = records().find((r) => r.kind === 'ws');
    assert.equal(ws.host, `127.0.0.1:${backend.port}`);
    assert.equal(ws.origin, null, 'the ws client must not send an Origin header');
    assert.equal(ws.tokenInQuery, true);
    assert.ok(await waitFor(() => backend.ws?.readyState === 1), 'upgrade should have been accepted');
  } finally {
    await backend.stop();
  }
});

test('each start generates a fresh token', opts, async () => {
  const { backend, start } = fakeBackend();
  try {
    await start();
    const first = backend.token;
    await start();
    assert.notEqual(backend.token, first);
  } finally {
    await backend.stop();
  }
});

test('childEnv cannot inject loader or interpreter variables into the child', opts, async () => {
  const { backend, records, start } = fakeBackend();
  try {
    await start({
      DYLD_INSERT_LIBRARIES: '/tmp/evil.dylib',
      NODE_OPTIONS: '--require /tmp/evil.js',
      HARMLESS_KEY: 'kept',
    });
    const started = records().find((r) => r.kind === 'start');
    assert.equal(started.dyld, null, 'DYLD_INSERT_LIBRARIES must be dropped');
    assert.equal(started.nodeOptions, null, 'NODE_OPTIONS must be dropped');
    assert.equal(started.custom, 'kept', 'ordinary keys still pass');
  } finally {
    await backend.stop();
  }
});

test('start() rejects promptly with a clear message when the binary is not executable (EACCES)', opts, async () => {
  const { backend, bin } = fakeBackend();
  fs.chmodSync(bin, 0o644);
  const states = [];
  backend.on('status', (s) => states.push(s.state));
  const t0 = Date.now();
  await assert.rejects(() => backend.start(path.dirname(bin)), (err) => {
    assert.match(err.message, /Could not start the hitspec CLI at /);
    assert.match(err.message, /EACCES/);
    assert.match(err.message, /chmod \+x/);
    return true;
  });
  assert.ok(Date.now() - t0 < 5000, 'must not wait for the 30s readiness timeout');
  assert.equal(backend.state, 'error');
  assert.match(backend.error, /EACCES/);
  assert.ok(states.includes('error'));
  assert.equal(backend.child, null);
});

test('start() rejects promptly when the binary does not exist (ENOENT)', opts, async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-backend-'));
  const backend = new Backend({ resolveBinary: async () => ({ path: path.join(dir, 'missing'), source: 'test' }) });
  await assert.rejects(() => backend.start(dir), /Could not start the hitspec CLI at .*ENOENT/s);
  assert.equal(backend.state, 'error');
});

test('start() reports a child that exits before it is ready, with its output', opts, async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-backend-'));
  const bin = path.join(dir, 'hitspec');
  fs.writeFileSync(bin, '#!/bin/sh\necho boom-from-child >&2\nexit 3\n', { mode: 0o755 });
  const backend = new Backend({ resolveBinary: async () => ({ path: bin, source: 'test' }) });
  await assert.rejects(() => backend.start(dir), /exited before becoming ready \(code 3.*boom-from-child/s);
  assert.equal(backend.state, 'error');
});

test('describeSpawnError gives actionable hints per failure', () => {
  const bin = '/Applications/hitspec Studio.app/Contents/Resources/hitspec';
  assert.match(describeSpawnError({ code: 'EACCES', message: 'spawn EACCES' }, bin, 'linux'), /chmod \+x/);
  assert.match(describeSpawnError({ code: 'EPERM', message: 'spawn EPERM' }, bin, 'darwin'), /xattr -dr com\.apple\.quarantine/);
  assert.match(describeSpawnError({ code: 'EBADARCH', message: 'bad cpu' }, bin, 'darwin'), /different CPU architecture/);
  assert.match(describeSpawnError({ code: 'ENOENT', message: 'spawn ENOENT' }, bin, 'linux'), /does not exist/);
});

test('the child pid is recorded while it runs and removed after stop', opts, async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-backend-'));
  const pidFile = path.join(dir, 'serve.pid.json');
  const { backend: _unused, bin, dir: wd } = fakeBackend();
  const backend = new Backend({ resolveBinary: async () => ({ path: bin, source: 'test' }), pidFile });
  try {
    await backend.start(wd, { childEnv: { FAKE_LOG: path.join(wd, 'log.jsonl') } });
    const rec = readPidFile(pidFile);
    assert.equal(rec.pid, backend.child.pid);
    assert.equal(rec.port, backend.port);
  } finally {
    await backend.stop();
  }
  assert.equal(readPidFile(pidFile), null);
});

test('an old CLI version is flagged in the status snapshot, a current or dev one is not', opts, async () => {
  for (const [version, expectWarning] of [['2.19.1', true], ['2.20.0', false], ['v2.21.3', false], ['dev', false]]) {
    const { backend, start } = fakeBackend();
    try {
      await start({ FAKE_VERSION: version });
      // the fake answers every GET with {version}, but system/info reads data.version
      assert.equal(backend.version, version);
      assert.equal(Boolean(backend.snapshot().warning), expectWarning, version);
      assert.ok(backend.snapshot().startedAt, 'startedAt is recorded when the server becomes ready');
    } finally {
      await backend.stop();
    }
  }
});
