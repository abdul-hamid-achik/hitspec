import fs from 'node:fs';
import path from 'node:path';
import { app, clipboard, dialog, ipcMain, shell } from 'electron';

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
export function registerIpc({ backend, settings, getWindow, broadcast }) {
  /** Absolute paths the user picked in a save dialog; the only writable targets. */
  const approvedWritePaths = new Set();

  const requireReady = () => {
    if (backend.state !== 'ready') throw new Error(`backend is ${backend.state}`);
    return backend.workDir;
  };

  ipcMain.handle('hitspec:status:get', () => backend.snapshot());
  ipcMain.handle('hitspec:backend:logs', (_e, limit = 200) => backend.getLogs(limit));

  ipcMain.handle('hitspec:app:info', () => ({
    version: app.getVersion(),
    electron: process.versions.electron,
    node: process.versions.node,
    chrome: process.versions.chrome,
    platform: process.platform,
    arch: process.arch,
    dev: !app.isPackaged,
    backend: backend.snapshot(),
  }));

  ipcMain.handle('hitspec:api', async (_e, req) => {
    if (!req || typeof req.path !== 'string') {
      return { ok: false, status: 0, data: null, text: '', error: 'invalid api request' };
    }
    return backend.request(req);
  });

  ipcMain.handle('hitspec:settings:get', () => settings.all());

  ipcMain.handle('hitspec:settings:set', (_e, patch) => {
    if (!patch || typeof patch !== 'object') return settings.all();
    // Only allow known top-level keys so a renderer bug cannot inject junk.
    const allowed = ['theme', 'view', 'window', 'backend', 'lastWorkspace'];
    const clean = Object.fromEntries(Object.entries(patch).filter(([k]) => allowed.includes(k)));
    return settings.set(clean);
  });

  async function openWorkspaceDir(dirPath) {
    const resolved = path.resolve(dirPath);
    let st;
    try {
      st = await fs.promises.stat(resolved);
    } catch (err) {
      throw new Error(`cannot open ${resolved}: ${err?.message ?? err}`);
    }
    if (!st.isDirectory()) throw new Error(`${resolved} is not a directory`);

    await backend.start(resolved, settings.get('backend') ?? {});
    settings.addRecent(resolved);
    broadcast('hitspec:workspace', { path: resolved, name: path.basename(resolved) });
    return backend.snapshot();
  }

  ipcMain.handle('hitspec:workspace:open', async () => {
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

  ipcMain.handle('hitspec:workspace:openPath', async (_e, dirPath) => {
    if (typeof dirPath !== 'string' || !dirPath) throw new Error('path is required');
    return openWorkspaceDir(dirPath);
  });

  ipcMain.handle('hitspec:workspace:close', async () => {
    await backend.stop();
    settings.set('lastWorkspace', null);
    return backend.snapshot();
  });

  ipcMain.handle('hitspec:workspace:recents', () => settings.get('recents') ?? []);

  ipcMain.handle('hitspec:workspace:removeRecent', (_e, dirPath) => {
    if (typeof dirPath !== 'string' || !dirPath) return settings.get('recents') ?? [];
    return settings.removeRecent(dirPath);
  });

  ipcMain.handle('hitspec:backend:restart', async () => {
    const workDir = backend.workDir ?? settings.get('lastWorkspace');
    if (!workDir) throw new Error('no workspace to restart');
    return openWorkspaceDir(workDir);
  });

  ipcMain.handle('hitspec:workspace:reveal', (_e, relPath) => {
    const root = requireReady();
    const abs = safeJoin(root, relPath);
    if (!abs) throw new Error('path outside workspace');
    shell.showItemInFolder(abs);
    return abs;
  });

  ipcMain.handle('hitspec:workspace:openInEditor', (_e, relPath) => {
    const root = requireReady();
    const abs = safeJoin(root, relPath);
    if (!abs) throw new Error('path outside workspace');
    return shell.openPath(abs);
  });

  ipcMain.handle('hitspec:workspace:openRoot', () => {
    const root = requireReady();
    return shell.openPath(root);
  });

  ipcMain.handle('hitspec:dialog:saveFile', async (_e, opts = {}) => {
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

  ipcMain.handle('hitspec:fs:writeFile', async (_e, args = {}) => {
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

  ipcMain.handle('hitspec:clipboard:write', (_e, text) => {
    clipboard.writeText(typeof text === 'string' ? text : String(text ?? ''));
    return true;
  });

  ipcMain.handle('hitspec:app:quit', () => {
    app.quit();
    return true;
  });

  return { openWorkspaceDir, approvedWritePaths };
}
