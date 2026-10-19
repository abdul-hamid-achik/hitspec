import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  buildStartOptions, createSecureHandle, DirtyGuard, isTrustedSender, requestAllowShell,
  sanitizeChildEnv, sanitizeSettingsPatch, ShellTrust, validateApiRequest,
} from '../../src/main/policy.js';

/* ---- sender verification -------------------------------------------------- */

const mainFrame = { url: 'hitspec://app/index.html', parent: null };
const okEvent = () => ({ senderFrame: mainFrame, sender: { mainFrame } });

test('isTrustedSender accepts only the app origin top frame', () => {
  assert.equal(isTrustedSender(okEvent()), true);
});

test('isTrustedSender rejects foreign origins, sub-frames and missing frames', () => {
  const frame = (url, parent = null) => ({ url, parent });
  const ev = (f, main = f) => ({ senderFrame: f, sender: { mainFrame: main } });
  assert.equal(isTrustedSender({}), false);
  assert.equal(isTrustedSender({ senderFrame: null }), false);
  assert.equal(isTrustedSender(ev(frame('https://evil.example/'))), false);
  assert.equal(isTrustedSender(ev(frame('file:///tmp/x.html'))), false);
  assert.equal(isTrustedSender(ev(frame('hitspec://evil/index.html'))), false);
  assert.equal(isTrustedSender(ev(frame('hitspec://app.evil.example/'))), false);
  assert.equal(isTrustedSender(ev(frame('not a url'))), false);
  assert.equal(isTrustedSender(ev(frame('hitspec://app/x', mainFrame), mainFrame)), false, 'sub-frame');
  assert.equal(isTrustedSender({ senderFrame: mainFrame, sender: { mainFrame: { url: 'other' } } }), false);
  assert.equal(isTrustedSender(okEvent(), { isKnownSender: () => false }), false, 'unknown webContents');
});

test('createSecureHandle blocks untrusted senders before the handler runs', async () => {
  const handlers = new Map();
  const ipcMain = { handle: (channel, fn) => handlers.set(channel, fn) };
  let calls = 0;
  const handle = createSecureHandle(ipcMain, (e) => e.trusted === true);
  handle('x', (_e, a, b) => { calls++; return a + b; });
  assert.equal(await handlers.get('x')({ trusted: true }, 1, 2), 3);
  assert.throws(() => handlers.get('x')({ trusted: false }, 1, 2), /untrusted sender/);
  assert.equal(calls, 1);
});

/* ---- REST allowlist ------------------------------------------------------- */

test('validateApiRequest accepts every call the renderer api module makes', async () => {
  const seen = [];
  globalThis.window = { hitspec: { api: async (req) => { seen.push(req); return { ok: true, data: null }; } } };
  try {
    const { api } = await import('../../src/renderer/src/api.js');
    for (const fn of Object.values(api)) await fn('my file ü.http', 'name', 'dev');
    // Nested files keep their separators; each segment is percent-encoded.
    await api.file('sub dir/ü file.http');
    await api.saveFile('sub dir/ü file.http', 'GET /');
    await api.deleteFile('a/b/c.http');
  } finally {
    delete globalThis.window;
  }
  assert.ok(seen.length > 40, `expected to exercise the whole api surface, got ${seen.length}`);
  for (const req of seen) {
    const checked = validateApiRequest(req);
    assert.equal(checked.ok, true, `${req.method} ${req.path}: ${checked.error}`);
  }
});

test('validateApiRequest rejects paths and methods the UI never uses', () => {
  const bad = [
    { method: 'GET', path: '/ws' },
    { method: 'GET', path: '/admin' },
    { method: 'DELETE', path: '/workspace' },
    { method: 'POST', path: '/files/x' },
    { method: 'GET', path: '/files/../../etc/passwd' },
    { method: 'GET', path: '/files/%2e%2e/secret' },
    { method: 'GET', path: '/files/a%2Fb' },
    { method: 'GET', path: '/files/a%5Cb' },
    { method: 'GET', path: '/files//x' },
    { method: 'GET', path: '/files/x?token=1' },
    { method: 'GET', path: '/files/x#y' },
    { method: 'GET', path: 'workspace' },
    { method: 'GET', path: 'http://evil.example/workspace' },
    { method: 'GET', path: '/files/%zz' },
    { method: 'TRACE', path: '/workspace' },
    { method: 'GET' },
    null,
    'GET /workspace',
  ];
  for (const req of bad) assert.equal(validateApiRequest(req).ok, false, JSON.stringify(req));
});

test('validateApiRequest only forwards known, well-typed fields', () => {
  const checked = validateApiRequest({
    method: 'post', path: '/execute', body: { file: 'a.http' }, headers: { Authorization: 'x' }, token: 'y', timeout: 5000,
  });
  assert.equal(checked.ok, true);
  assert.deepEqual(checked.request, { method: 'POST', path: '/execute', body: { file: 'a.http' }, timeout: 5000 });

  assert.equal(validateApiRequest({ path: '/workspace', query: [1] }).ok, false);
  assert.equal(validateApiRequest({ path: '/history/runs', query: { limit: {} } }).ok, false);
  assert.equal(validateApiRequest({ method: 'PUT', path: '/files/a.http', body: 'x', raw: true, accept: 'text/plain\r\nX-Evil: 1' }).ok, false);
  assert.equal(validateApiRequest({ method: 'PUT', path: '/files/a.http', body: 'x', raw: true, accept: 'text/plain; charset=utf-8' }).ok, true);
  assert.equal(validateApiRequest({ path: '/workspace', timeout: -1 }).ok, false);
  assert.equal(validateApiRequest({ path: '/workspace', timeout: 10 ** 9 }).request.timeout, 600000);
});

/* ---- settings / childEnv ---------------------------------------------------- */

test('sanitizeSettingsPatch drops childEnv, allowShell, lastWorkspace and junk', () => {
  const clean = sanitizeSettingsPatch({
    theme: 'dracula',
    view: 'history',
    lastWorkspace: '/etc',
    recents: [{ path: '/etc' }],
    window: { width: 1000, height: 700, x: 1, y: 2, maximized: true, evil: 1 },
    backend: {
      watch: false,
      readOnly: true,
      logLevel: 'debug',
      allowShell: true,
      childEnv: { DYLD_INSERT_LIBRARIES: '/tmp/evil.dylib' },
      env: 'prod',
    },
    __proto__: { polluted: true },
  });
  assert.deepEqual(clean, {
    theme: 'dracula',
    view: 'history',
    window: { width: 1000, height: 700, x: 1, y: 2, maximized: true },
    backend: { watch: false, readOnly: true, logLevel: 'debug' },
  });
  assert.equal(sanitizeSettingsPatch({ theme: '../x', view: 'A B', backend: { logLevel: 'trace', watch: 'yes' } }).theme, undefined);
  assert.deepEqual(sanitizeSettingsPatch({ backend: { logLevel: 'trace', watch: 'yes' } }), { backend: {} });
  assert.deepEqual(sanitizeSettingsPatch(null), {});
  assert.deepEqual(sanitizeSettingsPatch([1]), {});
});

test('sanitizeChildEnv drops loader and interpreter hijack variables', () => {
  assert.deepEqual(
    sanitizeChildEnv({
      DYLD_INSERT_LIBRARIES: 'x', DYLD_LIBRARY_PATH: 'x', LD_PRELOAD: 'x', LD_LIBRARY_PATH: 'x',
      NODE_OPTIONS: 'x', ELECTRON_RUN_AS_NODE: '1', HITSPEC_API_TOKEN: 'x', PATH: '/evil', 'BAD KEY': 'x',
      HOME: '/tmp/h', USERPROFILE: 'C:\\h', N: 5,
    }),
    { HOME: '/tmp/h', USERPROFILE: 'C:\\h' },
  );
  assert.deepEqual(sanitizeChildEnv(null), {});
});

test('buildStartOptions never carries childEnv and takes allowShell only from trust', () => {
  const opts = buildStartOptions({ watch: false, readOnly: true, logLevel: 'warn', allowShell: true, childEnv: { A: '1' } });
  assert.deepEqual(opts, { watch: false, readOnly: true, logLevel: 'warn', allowShell: false });
  assert.equal('childEnv' in opts, false);
  assert.equal(buildStartOptions({ allowShell: true }, { allowShell: true }).allowShell, true);
  assert.equal(buildStartOptions({}, {}).logLevel, 'info');
  assert.equal(buildStartOptions({ logLevel: 'nonsense' }).logLevel, 'info');
  assert.equal(buildStartOptions({ env: 'staging' }).env, 'staging');
  assert.equal(buildStartOptions({ env: '--evil flag' }).env, undefined);
});

/* ---- allowShell confirmation ------------------------------------------------ */

test('enabling shell execution requires main-process confirmation per workspace', async () => {
  const trust = new ShellTrust();
  const asked = [];
  const confirm = (answer) => async (dir) => { asked.push(dir); return answer; };

  assert.deepEqual(await requestAllowShell({ enabled: true, dir: '/ws/a', trust, confirm: confirm(false) }), { allowed: false });
  assert.equal(trust.has('/ws/a'), false, 'a declined dialog grants nothing');

  assert.deepEqual(await requestAllowShell({ enabled: true, dir: '/ws/a', trust, confirm: confirm(true) }), { allowed: true });
  assert.equal(trust.has('/ws/a'), true);
  assert.equal(trust.has('/ws/b'), false, 'approval is per workspace');

  // Already approved in this session: no second prompt.
  await requestAllowShell({ enabled: true, dir: '/ws/a', trust, confirm: confirm(false) });
  assert.deepEqual(asked, ['/ws/a', '/ws/a']);

  // A different workspace asks again.
  await requestAllowShell({ enabled: true, dir: '/ws/b', trust, confirm: confirm(false) });
  assert.equal(asked.at(-1), '/ws/b');

  assert.deepEqual(await requestAllowShell({ enabled: false, dir: '/ws/a', trust, confirm: confirm(true) }), { allowed: false });
  assert.equal(trust.has('/ws/a'), false);
  await assert.rejects(() => requestAllowShell({ enabled: true, dir: null, trust, confirm: confirm(true) }), /open a workspace/);
  assert.deepEqual(
    await requestAllowShell({ enabled: true, dir: '/ws/c', trust, confirm: async () => 'yes' }),
    { allowed: false },
    'only a strict true counts as approval',
  );
});

/* ---- dirty guard -------------------------------------------------------------- */

test('DirtyGuard asks only when there are unsaved edits and clears on discard', () => {
  const asked = [];
  let answer = false;
  const guard = new DirtyGuard({ confirmDiscard: (ctx) => { asked.push(ctx); return answer; } });

  assert.equal(guard.confirm(1, 'close'), true, 'clean: no prompt');
  assert.deepEqual(asked, []);

  guard.set(1, true);
  assert.equal(guard.confirm(1, 'close'), false, 'cancel keeps the window open');
  assert.equal(guard.isDirty(1), true);

  answer = true;
  assert.equal(guard.confirm(1, 'close'), true);
  assert.equal(guard.isDirty(1), false, 'discard clears the flag so a second close does not ask again');
  assert.deepEqual(asked, ['close', 'close']);

  guard.set(2, true);
  guard.set(3, true);
  assert.equal(guard.isDirty(), true);
  assert.equal(guard.confirm(undefined, 'quit'), true);
  assert.equal(guard.isDirty(), false);

  guard.set(4, true);
  guard.set(4, false);
  assert.equal(guard.confirm(4, 'x'), true);
});
