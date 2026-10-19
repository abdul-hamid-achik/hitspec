import fs from 'node:fs';
import path from 'node:path';
import { app, BrowserWindow, clipboard, dialog, ipcMain, shell } from 'electron';

import {
  createSecureHandle, isTrustedSender, requestAllowShell, sanitizeSettingsPatch, validateApiRequest,
} from './policy.js';

/** Reject relative paths that escape the workspace root. */
function safeJoin(root, rel) {
  if (!root || !rel) return null;
  const abs = path.resolve(root, rel);
  const relFromRoot = path.relative(root, abs);
  if (relFromRoot === '..' || relFromRoot.startsWith(`..${path.sep}`)) return null;
  return abs;
}

/**
 * Wires the renderer-facing IPC surface. The renderer never touches the network
 * or the API token: every call is proxied here.
 */
export function registerIpc({
  backend, settings, getWindow, broadcast, shellTrust, dirtyGuard, startOptionsFor, startedAt = new Date().toISOString(),
}) {
  // Every handler is wrapped: only the app's own top-level page may call in.
  const handle = createSecureHandle(
    ipcMain,
    (event) => isTrustedSender(event, { isKnownSender: (wc) => Boolean(wc && BrowserWindow.fromWebContents(wc)) }),
  );

  /** Absolute paths the user picked in a save dialog; the only writable targets. */
  const approvedWritePaths = new Set();

  const requireReady = () => {
    if (backend.state !== 'ready') throw new Error(`backend is ${backend.state}`);
    return backend.workDir;
  };

  handle('hitspec:status:get', () => backend.snapshot());
  handle('hitspec:backend:logs', (_e, limit = 200) => backend.getLogs(limit));

  handle('hitspec:app:info', () => ({
    version: app.getVersion(),
    electron: process.versions.electron,
    node: process.versions.node,
    chrome: process.versions.chrome,
    platform: process.platform,
    arch: process.arch,
    dev: !app.isPackaged,
    startedAt,
    backend: backend.snapshot(),
  }));

  handle('hitspec:api', async (_e, req) => {
    // Not a generic passthrough: method + path must match what the UI uses.
    const checked = validateApiRequest(req);
    if (!checked.ok) return { ok: false, status: 0, data: null, text: '', error: checked.error };
    return backend.request(checked.request);
  });

  /** Settings as the renderer sees them: shell approval is session state, not a stored flag. */
  const publicSettings = () => {
    const all = settings.all();
    return { ...all, backend: { ...all.backend, allowShell: shellTrust.has(backend.workDir) } };
  };

  handle('hitspec:settings:get', () => publicSettings());

  handle('hitspec:settings:set', (_e, patch) => {
    // Whitelist keys *and* value shapes; childEnv/allowShell are not writable here.
    settings.set(sanitizeSettingsPatch(patch));
    return publicSettings();
  });

  handle('hitspec:backend:allowShell', (_e, enabled) =>
    requestAllowShell({
      enabled: enabled === true,
      dir: backend.workDir,
      trust: shellTrust,
      confirm: async (dir) => {
        const win = getWindow();
        const options = {
          type: 'warning',
          buttons: ['Cancel', 'Allow shell execution'],
          defaultId: 0,
          cancelId: 0,
          noLink: true,
          title: 'Allow shell execution?',
          message: `Let "${path.basename(dir)}" run shell commands?`,
          detail:
            `${dir}\n\n` +
            'With this on, .http files in this workspace can run arbitrary commands as your user. ' +
            'Only enable it for workspaces you wrote or fully trust.\n\n' +
            'The approval lasts until you quit hitspec Studio and takes effect when the API server restarts.',
        };
        const result = win ? await dialog.showMessageBox(win, options) : await dialog.showMessageBox(options);
        return result.response === 1;
      },
    }));

  handle('hitspec:dirty:set', (event, value) => {
    dirtyGuard.set(event.sender.id, value === true);
    return true;
  });

  async function openWorkspaceDir(dirPath, { guard = true } = {}) {
    // Switching workspaces discards the editor buffer: ask first (null = cancelled).
    if (guard && !dirtyGuard.confirm(undefined, 'open another workspace')) return null;
    const resolved = path.resolve(dirPath);
    let st;
    try {
      st = await fs.promises.stat(resolved);
    } catch (err) {
      throw new Error(`cannot open ${resolved}: ${err?.message ?? err}`);
    }
    if (!st.isDirectory()) throw new Error(`${resolved} is not a directory`);

    await backend.start(resolved, startOptionsFor(resolved));
    settings.addRecent(resolved);
    broadcast('hitspec:workspace', { path: resolved, name: path.basename(resolved) });
    return backend.snapshot();
  }

  handle('hitspec:workspace:open', async () => {
    const win = getWindow();
    const result = await dialog.showOpenDialog(win, {
      title: 'Open a hitspec workspace',
      buttonLabel: 'Open Workspace',
      properties: ['openDirectory', 'createDirectory'],
      defaultPath: settings.get('lastWorkspace') ?? app.getPath('home'),
    });
    if (result.canceled || result.filePaths.length === 0) return null;
    return openWorkspaceDir(result.filePaths[0]);
  });

  handle('hitspec:workspace:openPath', async (_e, dirPath) => {
    if (typeof dirPath !== 'string' || !dirPath) throw new Error('path is required');
    return openWorkspaceDir(dirPath);
  });

  handle('hitspec:workspace:close', async () => {
    await backend.stop();
    settings.set('lastWorkspace', null);
    return backend.snapshot();
  });

  handle('hitspec:workspace:recents', () => settings.get('recents') ?? []);

  handle('hitspec:workspace:removeRecent', (_e, dirPath) => {
    if (typeof dirPath !== 'string' || !dirPath) return settings.get('recents') ?? [];
    return settings.removeRecent(dirPath);
  });

  handle('hitspec:backend:restart', async () => {
    const workDir = backend.workDir ?? settings.get('lastWorkspace');
    if (!workDir) throw new Error('no workspace to restart');
    return openWorkspaceDir(workDir, { guard: false });
  });

  handle('hitspec:workspace:reveal', (_e, relPath) => {
    const root = requireReady();
    const abs = safeJoin(root, relPath);
    if (!abs) throw new Error('path outside workspace');
    shell.showItemInFolder(abs);
    return abs;
  });

  handle('hitspec:workspace:openInEditor', (_e, relPath) => {
    const root = requireReady();
    const abs = safeJoin(root, relPath);
    if (!abs) throw new Error('path outside workspace');
    return shell.openPath(abs);
  });

  handle('hitspec:workspace:openRoot', () => {
    const root = requireReady();
    return shell.openPath(root);
  });

  handle('hitspec:dialog:saveFile', async (_e, opts = {}) => {
    const win = getWindow();
    const result = await dialog.showSaveDialog(win, {
      title: opts.title ?? 'Save file',
      defaultPath: opts.defaultPath ?? 'export.http',
      filters: opts.filters ?? [{ name: 'hitspec file', extensions: ['http', 'hitspec'] }],
    });
    if (result.canceled || !result.filePath) return null;
    approvedWritePaths.add(path.resolve(result.filePath));
    return result.filePath;
  });

  handle('hitspec:fs:writeFile', async (_e, args = {}) => {
    const target = typeof args.path === 'string' ? path.resolve(args.path) : null;
    if (!target) throw new Error('path is required');
    if (!approvedWritePaths.has(target)) {
      throw new Error('refusing to write to a path that was not chosen in a save dialog');
    }
    if (typeof args.content !== 'string') throw new Error('content must be a string');
    await fs.promises.mkdir(path.dirname(target), { recursive: true });
    await fs.promises.writeFile(target, args.content, 'utf8');
    return { path: target, bytes: Buffer.byteLength(args.content, 'utf8') };
  });

  handle('hitspec:clipboard:write', (_e, text) => {
    clipboard.writeText(typeof text === 'string' ? text : String(text ?? ''));
    return true;
  });

  handle('hitspec:app:quit', () => {
    app.quit();
    return true;
  });

  return { openWorkspaceDir, approvedWritePaths };

  // The main process opens workspaces too (a second launch, Finder "open with"),
  // and must go through the same guard and start sequence.
  return { openWorkspaceDir };
}
