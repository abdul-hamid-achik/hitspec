import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { app, BrowserWindow, dialog, net, protocol, screen, session, shell } from 'electron';

import { Backend } from './backend.js';
import { resolveBinary } from './binary.js';
import { registerIpc } from './ipc.js';
import { buildMenu } from './menu.js';
import { buildStartOptions, DirtyGuard, ShellTrust } from './policy.js';
import { Settings } from './settings.js';
import { reapStaleServer } from './stale.js';

app.setName('hitspec Studio');

// Tests (and portable setups) can redirect the settings store; must happen
// before the first app.getPath('userData') call below.
if (process.env.HITSPEC_DESKTOP_USER_DATA) {
  app.setPath('userData', path.resolve(process.env.HITSPEC_DESKTOP_USER_DATA));
}

const SCHEME = 'hitspec';
const here = path.dirname(fileURLToPath(import.meta.url));
const appRoot = path.resolve(here, '..', '..');
const rendererDir = path.join(appRoot, 'src', 'renderer');

// Must run before app ready so the scheme behaves like http(s).
protocol.registerSchemesAsPrivileged([
  {
    scheme: SCHEME,
    privileges: { standard: true, secure: true, supportFetchAPI: true, stream: true, codeCache: true },
  },
]);

const startedAt = new Date().toISOString();
const settings = new Settings(path.join(app.getPath('userData'), 'settings.json'));
settings.load();

/** Workspaces the user approved for --allow-shell in this session (never persisted). */
const shellTrust = new ShellTrust();

/** Everything `backend.start` may receive; nothing here is renderer-writable. */
const startOptionsFor = (dir) => buildStartOptions(settings.get('backend'), { allowShell: shellTrust.has(dir) });

const pidFile = path.join(app.getPath('userData'), 'serve.pid.json');

const dirtyGuard = new DirtyGuard({
  confirmDiscard: (context) => {
    const win = getMainWindow();
    const options = {
      type: 'warning',
      buttons: ['Cancel', 'Discard changes'],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
      title: 'Unsaved changes',
      message: 'You have unsaved changes in the source editor.',
      detail: `Discard them and ${context ?? 'continue'}?`,
    };
    const result = win ? dialog.showMessageBoxSync(win, options) : dialog.showMessageBoxSync(options);
    return result === 1;
  },
});

const backend = new Backend({
  pidFile,
  resolveBinary: () =>
    resolveBinary({
      platform: process.platform,
      isPackaged: app.isPackaged,
      resourcesPath: process.resourcesPath,
      appRoot,
      env: process.env,
    }),
});

/** @type {Set<BrowserWindow>} */
const windows = new Set();

function broadcast(channel, payload) {
  for (const win of windows) {
    if (!win.isDestroyed()) win.webContents.send(channel, payload);
  }
}

backend.on('status', (status) => broadcast('hitspec:status', status));
backend.on('event', (event) => broadcast('hitspec:event', event));
backend.on('socket', (info) => broadcast('hitspec:socket', info));
backend.on('log', (line) => broadcast('hitspec:log', line));

/** Queued workspace open requests that arrive before the window exists. */
let pendingWorkspace = null;

function clampToDisplay(bounds) {
  const displays = screen.getAllDisplays();
  const fits = displays.some((d) => {
    const { x, y, width, height } = d.workArea;
    return bounds.x + 80 < x + width && bounds.x + bounds.width - 80 > x &&
      bounds.y + 40 < y + height && bounds.y + bounds.height - 40 > y;
  });
  return fits ? bounds : null;
}

async function createWindow() {
  const saved = settings.get('window') ?? {};
  const width = Math.max(940, saved.width ?? 1360);
  const height = Math.max(600, saved.height ?? 860);
  const base = { width, height, x: saved.x ?? undefined, y: saved.y ?? undefined };
  const bounds = saved.x != null && saved.y != null ? clampToDisplay(base) : null;

  const win = new BrowserWindow({
    ...(bounds ?? base),
    minWidth: 940,
    minHeight: 600,
    show: false,
    backgroundColor: '#2E3440',
    title: 'hitspec Studio',
    titleBarStyle: process.platform === 'darwin' ? 'hiddenInset' : 'default',
    webPreferences: {
      preload: path.join(appRoot, 'src', 'preload', 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      spellcheck: false,
      webSecurity: true,
      webviewTag: false,
      allowRunningInsecureContent: false,
      experimentalFeatures: false,
      enableBlinkFeatures: '',
    },
  });

  windows.add(win);
  const wcId = win.webContents.id;
  win.once('closed', () => {
    windows.delete(win);
    dirtyGuard.clear(wcId);
  });

  // Closing the window throws the editor buffer away: ask first.
  win.on('close', (event) => {
    if (!dirtyGuard.confirm(wcId, 'close the window')) event.preventDefault();
  });

  const persistBounds = () => {
    if (win.isDestroyed()) return;
    const maximized = win.isMaximized();
    const b = win.getNormalBounds();
    settings.set('window', { width: b.width, height: b.height, x: b.x, y: b.y, maximized });
  };
  win.on('resize', persistBounds);
  win.on('move', persistBounds);
  win.on('maximize', persistBounds);
  win.on('unmaximize', persistBounds);
  win.on('closed', persistBounds);

  // Keep navigation inside the app; hand external links to the OS browser.
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//i.test(url)) shell.openExternal(url).catch(() => {});
    return { action: 'deny' };
  });
  win.webContents.on('will-navigate', (event, url) => {
    if (!url.startsWith(`${SCHEME}://app/`)) event.preventDefault();
  });
  win.webContents.on('will-attach-webview', (event) => event.preventDefault());
  // The renderer blocks unload (reload / devtools refresh) while it has unsaved
  // edits; ask in a native dialog and let the unload through if the user agrees.
  win.webContents.on('will-prevent-unload', (event) => {
    if (dirtyGuard.confirm(wcId, 'reload the window')) event.preventDefault();
  });

  await win.loadURL(`${SCHEME}://app/index.html`);

  if (saved.maximized) win.maximize();
  win.once('ready-to-show', () => win.show());

  if (process.env.HITSPEC_DESKTOP_DEV === '1' || process.argv.includes('--devtools')) {
    win.webContents.openDevTools({ mode: 'detach' });
  }

  return win;
}

function getMainWindow() {
  for (const win of windows) {
    if (!win.isDestroyed() && win.isVisible()) return win;
  }
  return [...windows].find((w) => !w.isDestroyed()) ?? null;
}

/**
 * The page also carries a CSP <meta>, but headers add what a meta tag cannot
 * (`frame-ancestors`) and cover every asset served over the scheme.
 */
const CSP = [
  "default-src 'self'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' data:",
  "connect-src 'self'",
  "font-src 'self'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
  "frame-ancestors 'none'",
].join('; ');

function withSecurityHeaders(res) {
  const headers = new Headers(res.headers);
  headers.set('Content-Security-Policy', CSP);
  headers.set('X-Content-Type-Options', 'nosniff');
  headers.set('Referrer-Policy', 'no-referrer');
  return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
}

/** Serve renderer assets from disk over the custom scheme. */
function registerProtocol() {
  protocol.handle(SCHEME, async (request) => {
    const url = new URL(request.url);
    const rel = decodeURIComponent(url.pathname).replace(/^\/+/, '') || 'index.html';
    const target = path.resolve(rendererDir, rel);
    const inside = path.relative(rendererDir, target);
    if (inside === '..' || inside.startsWith(`..${path.sep}`)) {
      return new Response('not found', { status: 404 });
    }
    try {
      const stat = await fs.promises.stat(target);
      const file = stat.isDirectory() ? path.join(target, 'index.html') : target;
      return withSecurityHeaders(await net.fetch(pathToFileURL(file).href));
    } catch {
      return new Response('not found', { status: 404 });
    }
  });
}

async function startWithWorkspace(dirPath) {
  if (!dirPath) return;
  try {
    await backend.start(path.resolve(dirPath), startOptionsFor(path.resolve(dirPath)));
    settings.addRecent(path.resolve(dirPath));
    buildMenu({ getWindow: getMainWindow, settings });
    broadcast('hitspec:workspace', { path: path.resolve(dirPath) });
  } catch (err) {
    broadcast('hitspec:status', backend.snapshot());
    broadcast('hitspec:toast', { severity: 'error', message: err?.message ?? String(err) });
  }
}

function workspaceArgFrom(argv) {
  const appPath = path.resolve(app.getAppPath?.() ?? appRoot);
  for (const arg of argv.slice(1)) {
    if (!arg || arg.startsWith('-')) continue;
    if (!path.isAbsolute(arg)) continue;
    const resolved = path.resolve(arg);
    // `electron <appDir> <workspace>` — never mistake the app itself for a
    // workspace to open.
    if (resolved === appPath || resolved === appRoot) continue;
    try {
      if (fs.statSync(resolved).isDirectory()) return resolved;
    } catch {
      // not a path we can open
    }
  }
  return null;
}

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on('second-instance', (_event, argv) => {
    const win = getMainWindow();
    if (win) {
      if (win.isMinimized()) win.restore();
      win.focus();
    }
    const dir = workspaceArgFrom(argv);
    if (dir) pendingWorkspace = dir;
  });

  // macOS: opening a folder/file from Finder or `open -a`.
  app.on('open-file', (event, filePath) => {
    event.preventDefault();
    if (!filePath) return;
    pendingWorkspace = fs.existsSync(filePath) && fs.statSync(filePath).isDirectory()
      ? filePath
      : path.dirname(filePath);
  });

  app.whenReady().then(async () => {
    // Deny every Chromium permission prompt (camera, geolocation, ...): the UI needs none.
    session.defaultSession.setPermissionRequestHandler((_wc, _permission, callback) => callback(false));
    session.defaultSession.setPermissionCheckHandler(() => false);

    // A previous run that died without cleanup (SIGKILL, crash) may have left its API server running.
    const reaped = await reapStaleServer(pidFile).catch(() => ({ reaped: false }));
    if (reaped.reaped) console.warn(`[hitspec] stopped a stale API server left by a previous run (pid ${reaped.pid})`);

    registerProtocol();
    registerIpc({
      backend, settings, getWindow: getMainWindow, broadcast, shellTrust, dirtyGuard, startOptionsFor, startedAt,
    });
    buildMenu({ getWindow: getMainWindow, settings });

    await createWindow();

    const fromArgv = workspaceArgFrom(process.argv);
    const initial = pendingWorkspace ?? fromArgv ?? settings.get('lastWorkspace');
    pendingWorkspace = null;
    if (initial && fs.existsSync(initial)) {
      await startWithWorkspace(initial);
    } else {
      broadcast('hitspec:status', backend.snapshot());
    }

    app.on('activate', () => {
      if (windows.size === 0) createWindow();
    });
  });

  app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') app.quit();
  });

  app.on('before-quit', (event) => {
    // Quitting skips the windows' 'close' path (app.exit), so ask here too.
    if (!dirtyGuard.confirm(undefined, 'quit')) {
      event.preventDefault();
      return;
    }
    if (backend.state === 'idle' || backend.state === 'stopped') return;
    event.preventDefault();
    backend.stop().finally(() => app.exit(0));
  });

  // Best effort when we are asked to terminate or die with a JS error.
  // SIGKILL cannot be intercepted; the next launch reaps the child via the pidfile.
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    process.on(signal, () => {
      backend.stop().finally(() => app.exit(0));
    });
  }
  process.on('exit', () => backend.killSync());
}

process.on('uncaughtException', (err) => {
  console.error('[hitspec] uncaught exception:', err);
  broadcast('hitspec:toast', { severity: 'error', message: `Unexpected error: ${err?.message ?? err}` });
});
process.on('unhandledRejection', (reason) => {
  console.error('[hitspec] unhandled rejection:', reason);
});
