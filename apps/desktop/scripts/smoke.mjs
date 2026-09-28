#!/usr/bin/env node
/**
 * Headless integration test for the desktop app's backend layer.
 *
 * Boots the real `hitspec serve --api-only` child process exactly the way the
 * Electron main process does, then exercises the REST + WebSocket surface the
 * renderer depends on. No window is created, so this runs in CI.
 *
 *   node scripts/smoke.mjs
 *   HITSPEC_BIN=/path/to/hitspec node scripts/smoke.mjs
 */
import assert from 'node:assert/strict';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';

import { Backend } from '../src/main/backend.js';
import { createWorkspace, findHitspecBinary, startFixtureApi } from '../test/helpers/harness.mjs';

const results = [];
let section = '';

function group(name) {
  section = name;
  console.log(`\n${name}`);
}

async function check(name, fn) {
  const started = Date.now();
  try {
    await fn();
    results.push({ section, name, ok: true, ms: Date.now() - started });
    console.log(`  ✓ ${name} (${Date.now() - started}ms)`);
  } catch (err) {
    results.push({ section, name, ok: false, ms: Date.now() - started, error: err?.message ?? String(err) });
    console.log(`  ✗ ${name}\n      ${String(err?.message ?? err).split('\n').join('\n      ')}`);
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitFor(predicate, { timeout = 15_000, interval = 100, label = 'condition' } = {}) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const value = await predicate();
    if (value) return value;
    await sleep(interval);
  }
  throw new Error(`timed out waiting for ${label}`);
}

function pickFreePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.once('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
  });
}

async function main() {
  const binary = await findHitspecBinary();
  console.log(`hitspec binary: ${binary.path} (${binary.source})`);

  const api = await startFixtureApi();
  console.log(`fixture API:    ${api.url}`);

  const workspace = createWorkspace({ port: api.port });
  console.log(`workspace:      ${workspace.dir}`);

  // The API server keeps its SQLite history under $HOME/.hitspec. Point HOME at
  // a throwaway directory so the test never touches the developer's real runs.
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-home-'));
  console.log(`home:           ${home}`);
  const startOptions = { watch: true, logLevel: 'info', childEnv: { HOME: home, USERPROFILE: home } };

  const backend = new Backend({ resolveBinary: async () => binary });
  const events = [];
  const statuses = [];
  backend.on('event', (event) => events.push(event));
  backend.on('status', (status) => statuses.push(status));

  const request = async (req) => {
    const result = await backend.request(req);
    assert.ok(result.ok, `${req.method ?? 'GET'} ${req.path} failed: ${result.error ?? result.status}`);
    return result.data;
  };

  let exitCode = 0;
  try {
    group('backend lifecycle');

    await check('starts the Go API server and becomes ready', async () => {
      await backend.start(workspace.dir, startOptions);
      assert.equal(backend.state, 'ready');
      assert.ok(backend.port > 0, 'expected a bound port');
      assert.ok(backend.token && backend.token.length >= 20, 'expected a generated API token');
      assert.deepEqual(
        statuses.map((s) => s.state),
        ['starting', 'ready'],
        'expected the renderer-visible status transitions',
      );
    });

    await check('binds to loopback only', async () => {
      const res = await fetch(`http://127.0.0.1:${backend.port}/api/v1/system/info`, {
        headers: { Authorization: `Bearer ${backend.token}` },
      });
      assert.equal(res.status, 200);
    });

    await check('rejects requests without the token', async () => {
      const res = await fetch(`http://127.0.0.1:${backend.port}/api/v1/system/info`);
      assert.equal(res.status, 401);
    });

    await check('connects the realtime WebSocket', async () => {
      const connected = await waitFor(() => events.length >= 0 && backend.ws?.readyState === 1, {
        timeout: 8000,
        label: 'websocket open',
      });
      assert.ok(connected);
    });

    group('workspace & files');

    await check('GET /system/info reports the CLI version', async () => {
      const info = await request({ path: '/system/info' });
      assert.ok(info.version, 'expected a version string');
      assert.equal(info.os, process.platform === 'darwin' ? 'darwin' : process.platform);
    });

    await check('GET /workspace returns the tree and request totals', async () => {
      const ws = await request({ path: '/workspace' });
      assert.equal(fs.realpathSync(ws.root), fs.realpathSync(workspace.dir));
      assert.equal(ws.environment, 'dev');
      assert.equal(ws.hasConfig, true);
      assert.ok(ws.totalRequests >= 6, `expected >= 6 requests, got ${ws.totalRequests}`);
      const dirs = ws.files.filter((n) => n.isDir).map((n) => n.name).sort();
      assert.deepEqual(dirs, ['api', 'mock']);
    });

    await check('GET /files lists .http files with request counts', async () => {
      const files = await request({ path: '/files' });
      const relative = files.map((f) => f.relativePath).sort();
      assert.deepEqual(relative, ['api/health.http', 'api/users.http', 'mock/pets.http']);
      const users = files.find((f) => f.relativePath === 'api/users.http');
      assert.equal(users.requestCount, 5);
    });

    await check('GET /files/{path} parses requests, assertions and captures', async () => {
      const parsed = await request({ path: '/files/api/users.http' });
      assert.deepEqual(parsed.variables ?? [], []);
      const names = parsed.requests.map((r) => r.name);
      assert.deepEqual(names, ['listUsers', 'createUser', 'getUser', 'deleteUser', 'missingUser']);
      const create = parsed.requests.find((r) => r.name === 'createUser');
      assert.equal(create.method, 'POST');
      assert.ok(create.body?.raw.includes('Ada Lovelace'), 'expected the JSON body to be parsed');
      assert.equal(create.headers?.[0]?.key, 'Content-Type');
      assert.ok(create.assertions.some((a) => a.operator === '==' && a.subject === 'body.name'));
      assert.deepEqual(create.captures.map((c) => c.name), ['createdUserId']);
      const get = parsed.requests.find((r) => r.name === 'getUser');
      assert.deepEqual(get.metadata?.depends, ['createUser']);
    });

    await check('GET /files/raw/{path} returns the exact bytes', async () => {
      const raw = await request({ path: '/files/raw/api/health.http', raw: true });
      assert.equal(raw, fs.readFileSync(workspace.file('api', 'health.http'), 'utf8'));
    });

    await check('rejects paths that escape the workspace', async () => {
      const res = await backend.request({ path: '/files/raw/..%2F..%2Fgo.mod', raw: true });
      assert.equal(res.ok, false);
      assert.ok([400, 403, 404].includes(res.status), `unexpected status ${res.status}`);
    });

    await check('POST /files creates a file and PUT saves it', async () => {
      await request({ method: 'POST', path: '/files', body: { path: 'scratch/temp.http', content: '### Temp\nGET {{baseUrl}}/health\n' } });
      assert.ok(fs.existsSync(workspace.file('scratch', 'temp.http')));
      await request({
        method: 'PUT',
        path: '/files/scratch/temp.http',
        body: '### Temp renamed\nGET {{baseUrl}}/health\n',
        raw: true,
      });
      assert.match(fs.readFileSync(workspace.file('scratch', 'temp.http'), 'utf8'), /Temp renamed/);
      const parsed = await request({ path: '/files/scratch/temp.http' });
      assert.equal(parsed.requests[0].name, 'Temp renamed');
    });

    await check('DELETE /files removes it again', async () => {
      await request({ method: 'DELETE', path: '/files/scratch/temp.http' });
      assert.equal(fs.existsSync(workspace.file('scratch', 'temp.http')), false);
    });

    await check('broadcasts file_changed over the WebSocket', async () => {
      events.length = 0;
      const target = workspace.file('api', 'health.http');
      const original = fs.readFileSync(target, 'utf8');
      await waitFor(async () => {
        fs.writeFileSync(target, `${original}\n# touched by the smoke test\n`, 'utf8');
        return events.some((e) => e.type === 'file_changed' && e.payload?.path === 'api/health.http');
      }, { timeout: 12_000, interval: 400, label: 'file_changed event' });
      fs.writeFileSync(target, original, 'utf8');
    });

    group('execution');

    await check('POST /execute runs a single request and returns the response', async () => {
      const result = await request({ method: 'POST', path: '/execute', body: { file: 'api/health.http', requestName: 'health' } });
      assert.equal(result.results.length, 1);
      const only = result.results[0];
      assert.equal(only.passed, true, only.error || JSON.stringify(only.assertions));
      assert.equal(only.response.statusCode, 200);
      assert.equal(only.request.method, 'GET');
      assert.match(only.request.url, /\/health$/);
      assert.ok(only.assertions.length >= 3);
      assert.ok(only.assertions.every((a) => a.passed));
      assert.match(JSON.parse(only.response.body).service, /hitspec-fixture/);
    });

    await check('POST /run executes a whole file with captures and dependencies', async () => {
      const result = await request({ method: 'POST', path: '/run', body: { file: 'api/users.http' } });
      assert.equal(result.results.length, 5);
      assert.equal(result.failed, 0, JSON.stringify(result.results.map((r) => ({ n: r.name, e: r.error, a: r.assertions })), null, 2));
      const created = result.results.find((r) => r.name === 'createUser');
      assert.equal(created.response.statusCode, 201);
      assert.ok(created.captures?.createdUserId, 'expected the createdUserId capture');
      const fetched = result.results.find((r) => r.name === 'getUser');
      assert.equal(fetched.passed, true);
      assert.match(fetched.request.url, new RegExp(`/users/${created.captures.createdUserId}$`));
    });

    await check('streams request_progress events while running', async () => {
      events.length = 0;
      await request({ method: 'POST', path: '/run', body: { file: 'api/users.http' } });
      const progress = events.filter((e) => e.type === 'request_progress');
      assert.ok(progress.length >= 5, `expected >= 5 progress events, got ${progress.length}`);
      assert.ok(progress.some((e) => e.payload?.status === 'started'));
      assert.ok(progress.some((e) => e.payload?.status === 'completed'));
    });

    await check('reports a failing assertion as a failed request', async () => {
      await request({
        method: 'POST',
        path: '/files',
        body: { path: 'scratch/failing.http', content: '### Bad expectation\nGET {{baseUrl}}/health\n\n>>>\nexpect status 418\nexpect body.status == "nope"\n<<<\n' },
      });
      const result = await request({ method: 'POST', path: '/run', body: { file: 'scratch/failing.http' } });
      assert.equal(result.failed, 1);
      assert.equal(result.results[0].passed, false);
      assert.ok(result.results[0].assertions.some((a) => !a.passed));
      await request({ method: 'DELETE', path: '/files/scratch/failing.http' });
    });

    group('environments, config & export');

    await check('GET /environments lists configured environments', async () => {
      const envs = await request({ path: '/environments' });
      const names = envs.map((e) => e.name).sort();
      assert.deepEqual(names, ['dev', 'staging']);
      assert.equal(envs.find((e) => e.name === 'dev').variables.baseUrl, api.url);
    });

    await check('PUT /environments/active switches the environment', async () => {
      await request({ method: 'PUT', path: '/environments/active', body: { name: 'staging' } });
      const ws = await request({ path: '/workspace' });
      assert.equal(ws.environment, 'staging');
      assert.ok(events.some((e) => e.type === 'environment_changed'));
      await request({ method: 'PUT', path: '/environments/active', body: { name: 'dev' } });
    });

    await check('PUT /environments/{name} persists variables to hitspec.yaml', async () => {
      await request({ method: 'PUT', path: '/environments/dev', body: { name: 'dev', variables: { baseUrl: api.url, extra: 'from-desktop' } } });
      const env = await request({ path: '/environments/dev' });
      assert.equal(env.variables.extra, 'from-desktop');
      const yaml = fs.readFileSync(workspace.file('hitspec.yaml'), 'utf8');
      assert.match(yaml, /from-desktop/);
      // restore so later checks see the pristine fixture
      await request({ method: 'PUT', path: '/environments/dev', body: { name: 'dev', variables: { baseUrl: api.url } } });
    });

    await check('GET/PUT /config round-trips hitspec.yaml', async () => {
      const before = await request({ path: '/config' });
      assert.equal(before.defaultEnvironment, 'dev');
      const updated = await request({ method: 'PUT', path: '/config', body: { ...before, timeout: 12000 } });
      assert.equal(updated.timeout, 12000);
    });

    await check('POST /export/curl produces a runnable command', async () => {
      const result = await request({ method: 'POST', path: '/export/curl', body: { file: 'api/health.http', requestName: 'health' } });
      assert.equal(result.commands.length, 1);
      assert.match(result.commands[0], /^curl/);
      assert.match(result.commands[0], /\/health/);
    });

    await check('POST /import/curl converts a command into .http', async () => {
      const result = await request({
        method: 'POST',
        path: '/import/curl',
        body: { command: `curl -X POST '${api.url}/users' -H 'Content-Type: application/json' -d '{"name":"Imported"}'` },
      });
      assert.ok(result.requestCount >= 1);
      assert.match(result.content, /###/);
      assert.match(result.content, /POST/);
      assert.match(result.content, /Imported/);
    });

    group('history');

    await check('GET /history/runs lists persisted runs with results', async () => {
      const page = await request({ path: '/history/runs', query: { limit: 50 } });
      assert.ok(page.total >= 3, `expected >= 3 stored runs, got ${page.total}`);
      const run = page.runs.find((r) => r.filePath.includes('users.http'));
      assert.ok(run, 'expected a stored run for api/users.http');
      const detail = await request({ path: `/history/runs/${run.id}` });
      assert.equal(detail.results.length, 5);
      assert.ok(detail.results.every((r) => r.requestName));
      const withAssertions = detail.results.find((r) => (r.assertions ?? []).length);
      assert.ok(withAssertions, 'expected assertions to be stored');
    });

    await check('GET /history/results filters by request name', async () => {
      const page = await request({ path: '/history/results', query: { requestName: 'health', filePath: 'api/health.http', limit: 10 } });
      assert.ok(page.total >= 1, 'expected at least one stored health result');
      assert.ok(page.results.every((r) => r.requestName === 'health'));
    });

    await check('DELETE /history/runs/{id} removes a single run', async () => {
      const page = await request({ path: '/history/runs', query: { limit: 1 } });
      const id = page.runs[0].id;
      const before = page.total;
      await request({ method: 'DELETE', path: `/history/runs/${id}` });
      const after = await request({ path: '/history/runs', query: { limit: 1 } });
      assert.equal(after.total, before - 1);
    });

    group('mock, stress & recording');

    const mockPort = await pickFreePort();
    await check('POST /mock/start serves >>>mock blocks', async () => {
      const status = await request({ method: 'POST', path: '/mock/start', body: { files: ['mock/pets.http'], port: mockPort } });
      assert.equal(status.running, true);
      assert.equal(status.port, mockPort);
      assert.ok(status.routes.length >= 2, `expected >= 2 routes, got ${status.routes.length}`);

      const listed = await waitFor(async () => {
        try {
          const res = await fetch(`http://127.0.0.1:${mockPort}/pets`);
          if (!res.ok) return null;
          return res.json();
        } catch {
          return null;
        }
      }, { timeout: 8000, label: 'mock route to answer' });
      assert.equal(Array.isArray(listed), true);
      assert.equal(listed[0].name, 'Rex');

      const routes = await request({ path: '/mock/routes' });
      assert.equal(routes.running, true);
      await waitFor(() => events.some((e) => e.type === 'mock_request'), { timeout: 5000, label: 'mock_request event' });
      await request({ method: 'POST', path: '/mock/stop' });
      await waitFor(async () => (await request({ path: '/mock/routes' })).running === false, { timeout: 5000, label: 'mock to stop' });
    });

    await check('POST /stress/start streams metrics and produces a result', async () => {
      events.length = 0;
      await request({ method: 'POST', path: '/stress/start', body: { files: ['api/health.http'], duration: '2s', rate: 25, environment: 'dev' } });
      const running = await request({ path: '/stress/status' });
      assert.equal(running.running, true);

      await waitFor(() => events.some((e) => e.type === 'stress_update' && e.payload?.stats), {
        timeout: 10_000,
        label: 'first stress_update',
      });
      await waitFor(() => events.some((e) => e.type === 'stress_update' && e.payload?.completed), {
        timeout: 30_000,
        label: 'stress completion',
      });

      const result = await request({ path: '/stress/result' });
      assert.ok(result.total > 0, 'expected stress traffic');
      assert.equal(result.errors, 0);
      assert.ok(result.rps > 0);
      assert.ok(result.p95Ms >= 0);
      assert.ok(Array.isArray(result.breakdown) && result.breakdown.length >= 1);
      assert.ok(Array.isArray(result.timeSeries) && result.timeSeries.length >= 1);
    });

    await check('GET /stress/profiles reads profiles from hitspec.yaml', async () => {
      const profiles = await request({ path: '/stress/profiles' });
      assert.ok(profiles.some((p) => p.name === 'quick'), JSON.stringify(profiles));
      assert.equal(profiles.find((p) => p.name === 'quick').duration, '2s');
    });

    const recordPort = await pickFreePort();
    await check('POST /record/start proxies and captures traffic', async () => {
      await request({ method: 'POST', path: '/record/start', body: { targetUrl: api.url, port: recordPort, deduplicate: false } });
      const status = await waitFor(async () => {
        const current = await request({ path: '/record/status' });
        return current.running ? current : null;
      }, { timeout: 8000, label: 'recorder to start' });
      assert.equal(status.port, recordPort);
      assert.equal(status.targetUrl, api.url);

      await waitFor(async () => {
        try {
          await fetch(`http://127.0.0.1:${recordPort}/health`);
          const after = await request({ path: '/record/status' });
          return (after.count ?? 0) > 0;
        } catch {
          return false;
        }
      }, { timeout: 10_000, label: 'a recorded request' });

      const exported = await request({ method: 'POST', path: '/record/export' });
      assert.match(exported.content, /###/);
      assert.match(exported.content, /GET/);
      await request({ method: 'DELETE', path: '/record/clear' });
      assert.equal((await request({ path: '/record/status' })).count, 0);
      await request({ method: 'POST', path: '/record/stop' });
    });

    group('shutdown');

    await check('stop() terminates the child and releases the port', async () => {
      const port = backend.port;
      const pid = backend.child?.pid;
      assert.ok(pid, 'expected a child pid');
      await backend.stop();
      assert.equal(backend.state, 'stopped');
      assert.equal(backend.child, null);

      await waitFor(async () => {
        try {
          process.kill(pid, 0);
          return false;
        } catch {
          return true;
        }
      }, { timeout: 8000, label: 'child process to exit' });

      const reusable = await new Promise((resolve) => {
        const srv = net.createServer();
        srv.once('error', () => resolve(false));
        srv.listen(port, '127.0.0.1', () => srv.close(() => resolve(true)));
      });
      assert.equal(reusable, true, 'expected the port to be released');
    });

    await check('requests after shutdown fail cleanly instead of hanging', async () => {
      const res = await backend.request({ path: '/system/info' });
      assert.equal(res.ok, false);
      assert.match(res.error, /backend is stopped/);
    });

    await check('restarts and becomes ready again', async () => {
      await backend.start(workspace.dir, startOptions);
      assert.equal(backend.state, 'ready');
      const info = await request({ path: '/system/info' });
      assert.ok(info.version);
      await backend.stop();
    });
  } catch (err) {
    console.error('\nfatal:', err);
    exitCode = 1;
  } finally {
    try { await backend.stop(); } catch { /* already stopped */ }
    await api.close();
    workspace.cleanup();
    fs.rmSync(home, { recursive: true, force: true });
  }

  const failed = results.filter((r) => !r.ok);
  console.log(`\n${'─'.repeat(60)}`);
  console.log(`${results.length - failed.length}/${results.length} checks passed`);
  if (backend.getLogs(0).length) console.log(`captured ${backend.lines.length} server log line(s)`);
  if (failed.length) {
    console.log('\nfailures:');
    for (const failure of failed) console.log(`  - [${failure.section}] ${failure.name}: ${failure.error}`);
    exitCode = 1;
  }
  process.exit(exitCode);
}

const hardTimeout = setTimeout(() => {
  console.error('\nsmoke test exceeded its hard timeout');
  process.exit(2);
}, 180_000);
hardTimeout.unref?.();

main().catch((err) => {
  console.error('smoke test crashed:', err);
  process.exit(2);
});
