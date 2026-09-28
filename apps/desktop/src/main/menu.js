import { app, Menu, shell } from 'electron';

const DOCS_URL = 'https://github.com/abdul-hamid-achik/hitspec#readme';

/**
 * Builds the native application menu. Custom items forward a command name to the
 * renderer over `hitspec:menu`; the renderer owns the actual behaviour so
 * shortcuts and the command palette stay in sync.
 */
export function buildMenu({ getWindow, settings, onCommand }) {
  const send = (command, payload) => {
    const win = getWindow();
    if (!win || win.isDestroyed()) return;
    win.webContents.send('hitspec:menu', { command, payload });
    onCommand?.(command, payload);
  };

  const isMac = process.platform === 'darwin';

  /** @type {Electron.MenuItemConstructorOptions[]} */
  const viewItems = [
    { label: 'Requests', accelerator: 'CommandOrControl+1', click: () => send('view:workspace') },
    { label: 'History', accelerator: 'CommandOrControl+2', click: () => send('view:history') },
    { label: 'Stress', accelerator: 'CommandOrControl+3', click: () => send('view:stress') },
    { label: 'Mock Server', accelerator: 'CommandOrControl+4', click: () => send('view:mock') },
    { label: 'Recording Proxy', accelerator: 'CommandOrControl+5', click: () => send('view:record') },
    { label: 'Import', accelerator: 'CommandOrControl+6', click: () => send('view:import') },
    { label: 'Contracts', accelerator: 'CommandOrControl+7', click: () => send('view:contract') },
    { label: 'Settings', accelerator: 'CommandOrControl+8', click: () => send('view:settings') },
  ];

  const recents = settings.get('recents') ?? [];
  const recentSubmenu = recents.length
    ? recents.map((entry) => ({
        label: entry.name || entry.path,
        sublabel: entry.path,
        click: () => send('workspace:openPath', entry.path),
      }))
    : [{ label: 'No recent workspaces', enabled: false }];

  /** @type {Electron.MenuItemConstructorOptions[]} */
  const template = [
    ...(isMac
      ? [{
          label: app.name,
          submenu: [
            { role: 'about' },
            { type: 'separator' },
            { label: 'Settings…', accelerator: 'CommandOrControl+,', click: () => send('view:settings') },
            { type: 'separator' },
            { role: 'services' },
            { type: 'separator' },
            { role: 'hide' },
            { role: 'hideOthers' },
            { role: 'unhide' },
            { type: 'separator' },
            { role: 'quit' },
          ],
        }]
      : []),
    {
      label: 'File',
      submenu: [
        { label: 'Open Workspace…', accelerator: 'CommandOrControl+O', click: () => send('workspace:open') },
        { label: 'Open Recent', submenu: recentSubmenu },
        { label: 'Close Workspace', click: () => send('workspace:close') },
        { type: 'separator' },
        { label: 'Save File', accelerator: 'CommandOrControl+S', click: () => send('file:save') },
        { label: 'Revert File', accelerator: 'CommandOrControl+Shift+S', click: () => send('file:revert') },
        { type: 'separator' },
        { label: 'New File…', accelerator: 'CommandOrControl+N', click: () => send('file:new') },
        { label: 'Reveal in File Manager', click: () => send('workspace:revealRoot') },
        { type: 'separator' },
        isMac ? { role: 'close' } : { role: 'quit' },
      ],
    },
    {
      label: 'Run',
      submenu: [
        { label: 'Run Request', accelerator: 'CommandOrControl+Enter', click: () => send('run:request') },
        { label: 'Run File', accelerator: 'CommandOrControl+Shift+Enter', click: () => send('run:file') },
        { type: 'separator' },
        { label: 'Command Palette…', accelerator: 'CommandOrControl+K', click: () => send('palette:toggle') },
        { type: 'separator' },
        { label: 'Restart API Server', accelerator: 'CommandOrControl+Shift+R', click: () => send('backend:restart') },
      ],
    },
    {
      label: 'View',
      submenu: [
        ...viewItems,
        { type: 'separator' },
        { role: 'reload' },
        { role: 'forceReload' },
        { role: 'toggleDevTools' },
        { type: 'separator' },
        { role: 'resetZoom' },
        { role: 'zoomIn' },
        { role: 'zoomOut' },
        { type: 'separator' },
        { role: 'togglefullscreen' },
      ],
    },
    {
      label: 'Window',
      submenu: [
        { role: 'minimize' },
        { role: 'zoom' },
        ...(isMac ? [{ type: 'separator' }, { role: 'front' }, { type: 'separator' }, { role: 'window' }] : [{ role: 'close' }]),
      ],
    },
    {
      label: 'Help',
      submenu: [
        { label: 'Keyboard Shortcuts', accelerator: 'F1', click: () => send('help:shortcuts') },
        {
          label: 'hitspec Documentation',
          click: async () => {
            await shell.openExternal(DOCS_URL);
          },
        },
        { label: '.http Syntax Reference', click: () => send('help:syntax') },
      ],
    },
  ];

  const menu = Menu.buildFromTemplate(template);
  Menu.setApplicationMenu(menu);
  return menu;
}
