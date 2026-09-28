'use strict';

const { contextBridge, ipcRenderer } = require('electron');

/**
 * The renderer talks to hitspec exclusively through this bridge: no Node
 * integration, no direct network access, and the API token never leaves the
 * main process.
 */

function subscribe(channel, callback) {
  const listener = (_event, payload) => {
    try {
      callback(payload);
    } catch (err) {
      console.error(`[hitspec] listener for ${channel} failed:`, err);
    }
  };
  ipcRenderer.on(channel, listener);
  return () => ipcRenderer.removeListener(channel, listener);
}

const invoke = (channel, ...args) => ipcRenderer.invoke(channel, ...args);

contextBridge.exposeInMainWorld('hitspec', {
  // --- status & info ---
  getStatus: () => invoke('hitspec:status:get'),
  getAppInfo: () => invoke('hitspec:app:info'),
  getSettings: () => invoke('hitspec:settings:get'),
  setSettings: (patch) => invoke('hitspec:settings:set', patch),
  getLogs: (limit) => invoke('hitspec:backend:logs', limit),

  // --- REST proxy to `hitspec serve --api-only` ---
  api: (req) => invoke('hitspec:api', req),

  // --- workspace lifecycle ---
  openWorkspace: () => invoke('hitspec:workspace:open'),
  openWorkspacePath: (dirPath) => invoke('hitspec:workspace:openPath', dirPath),
  closeWorkspace: () => invoke('hitspec:workspace:close'),
  restartBackend: () => invoke('hitspec:backend:restart'),
  getRecents: () => invoke('hitspec:workspace:recents'),
  removeRecent: (dirPath) => invoke('hitspec:workspace:removeRecent', dirPath),

  // --- OS integration ---
  reveal: (relPath) => invoke('hitspec:workspace:reveal', relPath),
  openInEditor: (relPath) => invoke('hitspec:workspace:openInEditor', relPath),
  openRoot: () => invoke('hitspec:workspace:openRoot'),
  saveFileDialog: (opts) => invoke('hitspec:dialog:saveFile', opts),
  writeFile: (filePath, content) => invoke('hitspec:fs:writeFile', { path: filePath, content }),
  copy: (text) => invoke('hitspec:clipboard:write', text),
  quit: () => invoke('hitspec:app:quit'),

  // --- pushed events ---
  onEvent: (cb) => subscribe('hitspec:event', cb),
  onStatus: (cb) => subscribe('hitspec:status', cb),
  onSocket: (cb) => subscribe('hitspec:socket', cb),
  onLog: (cb) => subscribe('hitspec:log', cb),
  onMenu: (cb) => subscribe('hitspec:menu', cb),
  onWorkspace: (cb) => subscribe('hitspec:workspace', cb),
  onToast: (cb) => subscribe('hitspec:toast', cb),
});
