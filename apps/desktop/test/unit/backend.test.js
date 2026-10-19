import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';

import { Backend } from '../../src/main/backend.js';

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
const port = Number(argv[argv.indexOf('--port') + 1]);
const token = process.env.HITSPEC_API_TOKEN;
const logFile = process.env.FAKE_LOG;
const log = (rec) => fs.appendFileSync(logFile, JSON.stringify(rec) + '\n');
log({ kind: 'start', argv, envToken: token ?? null });

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
    send(200, { version: 'fake', echo: raw ? body : body ? JSON.parse(body) : null });
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
  const start = () => backend.start(dir, { childEnv: { FAKE_LOG: logFile } });
  return { backend, records, start };
}

async function waitFor(fn, ms = 3000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (fn()) return true;
    await new Promise((r) => setTimeout(r, 25));
  }
  return false;
}

test('the API token goes to the child through HITSPEC_API_TOKEN, never argv', async () => {
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

test('requests satisfy the server Host/Origin/Content-Type policy', async () => {
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

test('the WebSocket upgrade has a loopback Host, no Origin and the token', async () => {
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

test('each start generates a fresh token', async () => {
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
