import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { resolveBinary } from '../../src/main/binary.js';

const here = path.dirname(fileURLToPath(import.meta.url));
const desktopRoot = path.resolve(here, '..', '..');
const fixtureRoot = path.join(desktopRoot, 'test', 'fixtures', 'workspace');

/**
 * A tiny in-process API for the integration tests: enough behaviour for the
 * fixture `.http` files (health probe, list/create/get/delete users, 404s) and
 * a request log the assertions can inspect.
 */
export async function startFixtureApi() {
  let nextId = 3;
  const users = new Map([
    [1, { id: 1, name: 'Ada', email: 'ada@example.com' }],
    [2, { id: 2, name: 'Grace', email: 'grace@example.com' }],
  ]);
  const requests = [];

  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    requests.push({ method: req.method, path: url.pathname, at: Date.now() });

    const send = (status, payload, headers = {}) => {
      const body = payload === undefined ? '' : JSON.stringify(payload);
      res.writeHead(status, {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(body),
        ...headers,
      });
      res.end(body);
    };

    if (url.pathname === '/health') {
      return send(200, { status: 'ok', service: 'hitspec-fixture', version: '1.0.0', uptime: process.uptime() });
    }

    if (url.pathname === '/users' && req.method === 'GET') {
      return send(200, [...users.values()]);
    }

    if (url.pathname === '/users' && req.method === 'POST') {
      let raw = '';
      req.on('data', (chunk) => { raw += chunk; });
      req.on('end', () => {
        let parsed = {};
        try {
          parsed = JSON.parse(raw || '{}');
        } catch {
          return send(400, { error: 'invalid json' });
        }
        const user = { id: nextId++, name: parsed.name ?? 'unnamed', email: parsed.email ?? null };
        users.set(user.id, user);
        return send(201, user);
      });
      return undefined;
    }

    const match = /^\/users\/(\d+)$/.exec(url.pathname);
    if (match) {
      const id = Number(match[1]);
      if (req.method === 'DELETE') {
        const existed = users.delete(id);
        res.writeHead(existed ? 204 : 404, { 'Content-Length': '0' });
        return res.end();
      }
      const user = users.get(id);
      return user ? send(200, user) : send(404, { error: 'not found', id });
    }

    return send(404, { error: 'not found', path: url.pathname });
  });

  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();

  return {
    port,
    url: `http://127.0.0.1:${port}`,
    requests,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}

/**
 * Copy the fixture workspace into a temp directory with the API port baked in.
 * A throwaway copy keeps `hitspec.yaml` writes (environments, stress profiles)
 * out of the repository.
 */
export function createWorkspace({ port, prefix = 'hitspec-desktop-' } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));

  const copy = (from, to) => {
    const stat = fs.statSync(from);
    if (stat.isDirectory()) {
      fs.mkdirSync(to, { recursive: true });
      for (const entry of fs.readdirSync(from)) copy(path.join(from, entry), path.join(to, entry));
      return;
    }
    fs.mkdirSync(path.dirname(to), { recursive: true });
    const text = fs.readFileSync(from, 'utf8').replaceAll('__PORT__', String(port ?? ''));
    fs.writeFileSync(to, text, 'utf8');
  };
  copy(fixtureRoot, dir);

  return {
    dir,
    file: (...segments) => path.join(dir, ...segments),
    cleanup: () => fs.rmSync(dir, { recursive: true, force: true }),
  };
}

/** Locate the Go CLI the same way the packaged app does. */
export async function findHitspecBinary() {
  const resolved = await resolveBinary({
    platform: process.platform,
    isPackaged: false,
    appRoot: desktopRoot,
    env: process.env,
  });
  if (resolved.error) throw new Error(resolved.error);
  return resolved;
}

export { desktopRoot, fixtureRoot };
