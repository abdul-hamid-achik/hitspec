import * as actions from './actions.js';
import { h, clear, withPreservedFocus } from './dom.js';
import { icon } from './icons.js';
import { openPalette } from './palette.js';
import { openPopover } from './popover.js';
import { relativeTime } from './format.js';
import { set, state, subscribe } from './store.js';
import { toast } from './toast.js';
import { btn, closeTopOverlay, codeBlock, modal, pill } from './ui.js';
import { createSidebar } from './views/sidebar.js';
import { VIEWS, createWelcomeView, viewById } from './views/index.js';

const topbarEl = document.getElementById('topbar');
const sidebarEl = document.getElementById('sidebar');
const contentEl = document.getElementById('content');
const statusbarEl = document.getElementById('statusbar');

/* -------------------------------------------------------------------------- */
/* shell (built once, then updated in place so focus is never stolen)          */
/* -------------------------------------------------------------------------- */

const workspaceChip = h('button', { class: 'workspace-chip nodrag', title: 'Open a workspace folder' },
  icon('folder'),
  h('span', { class: 'name', text: 'Open workspace…' }),
  icon('chevron', { cls: 'chev' }));
workspaceChip.addEventListener('click', () => openWorkspaceSwitcher());

/* View rail (activity bar). The visible label lives in a sr-only span so the
   buttons stay accessible and testable without cluttering the bar. */
const railEl = document.getElementById('rail');
const railItems = new Map();
railEl.append(
  ...VIEWS.map((view) => {
    const item = h('button', {
      class: 'rail-item',
      type: 'button',
      title: `${view.label}  ⌘${view.num}`,
      ariaLabel: view.label,
      onClick: () => actions.setView(view.id),
    }, icon(view.icon), h('span', { class: 'sr-only', text: view.label }));
    railItems.set(view.id, item);
    return item;
  }),
);

const envSelect = h('select', { class: 'select nodrag', ariaLabel: 'Active environment', title: 'Active environment' });
envSelect.addEventListener('change', () => actions.setActiveEnvironment(envSelect.value));
const envEdit = h('button', {
  class: 'btn icon nodrag',
  type: 'button',
  title: 'Edit environment variables',
  ariaLabel: 'Edit environment variables',
  onClick: () => actions.editEnvironmentDialog(state.activeEnv),
}, icon('pencil'));
const envGroup = h('div', { class: 'env-group nodrag' }, h('div', { class: 'select-wrap' }, envSelect), envEdit);

const paletteBtn = h('button', {
  class: 'btn icon nodrag',
  type: 'button',
  title: 'Command palette (⌘K)',
  ariaLabel: 'Command palette',
  onClick: () => showPalette(),
}, icon('search'), h('span', { class: 'kbd hide-narrow', text: '⌘K' }));

const statusDot = h('span', { class: 'dot' });
const statusLabel = h('span', { text: 'idle' });
const statusGroup = h('div', { class: 'status-group nodrag' }, statusDot, statusLabel);

topbarEl.append(
  h('div', { class: 'brand' }, h('span', { class: 'brand-mark', text: 'h' }), h('span', { class: 'hide-narrow', text: 'Studio' })),
  workspaceChip,
  h('div', { class: 'spacer' }),
  envGroup,
  paletteBtn,
  statusGroup,
);

/* statusbar */
const sbWorkspace = h('button', { class: 'item', title: 'Reveal workspace in file manager' });
const sbFile = h('span', { class: 'item mono' });
const sbDirty = h('span', { class: 'item' });
const sbCounts = h('span', { class: 'item mono' });
const sbSocket = h('span', { class: 'item' });
const sbVersion = h('span', { class: 'item mono' });
sbWorkspace.addEventListener('click', () => window.hitspec.openRoot().catch(() => {}));

statusbarEl.append(
  sbWorkspace,
  sbCounts,
  h('div', { class: 'spacer' }),
  sbFile,
  sbDirty,
  sbSocket,
  sbVersion,
);

/* -------------------------------------------------------------------------- */
/* render                                                                      */
/* -------------------------------------------------------------------------- */

let sidebar = null;
let currentView = null;
let currentViewId = null;
let envSignature = '';

function activeViewId(s) {
  if (s.status?.state !== 'ready' || !s.workspace) return 'welcome';
  return s.view;
}

function updateTopbar(s) {
  const nameEl = workspaceChip.querySelector('.name');
  if (s.workspace) {
    nameEl.textContent = s.workspace.root.split('/').pop() || s.workspace.root;
    workspaceChip.title = `${s.workspace.root}\nClick to open another workspace`;
  } else {
    nameEl.textContent = s.status?.state === 'starting' ? 'Starting…' : 'Open workspace…';
    workspaceChip.title = s.status?.workDir
      ? `${s.status.workDir}\nClick to open another workspace`
      : 'Open a workspace folder';
  }

  const ready = s.status?.state === 'ready';
  for (const view of VIEWS) {
    const item = railItems.get(view.id);
    item.classList.toggle('active', ready && s.view === view.id);
    item.disabled = !ready && view.id !== 'settings';
  }

  const signature = (s.environments ?? []).map((e) => e.name).join('|');
  if (signature !== envSignature) {
    envSignature = signature;
    clear(envSelect);
    for (const env of s.environments ?? []) {
      envSelect.append(h('option', { value: env.name, text: env.name }));
    }
  }
  if (document.activeElement !== envSelect) envSelect.value = s.activeEnv ?? '';
  envSelect.disabled = !ready || !(s.environments ?? []).length;
  envEdit.disabled = !ready;
  // An empty disabled select reads as broken UI on the welcome screen.
  envGroup.classList.toggle('hidden', !ready);

  const tone = { ready: 'ok', starting: 'warn pulse', error: 'bad', stopped: '', idle: '' }[s.status?.state] ?? '';
  statusDot.className = ['dot', tone].filter(Boolean).join(' ');
  statusLabel.textContent = s.status?.state ?? 'idle';
  statusGroup.title = s.status?.error ?? (s.status?.port ? `127.0.0.1:${s.status.port}` : '');
}

function updateStatusbar(s) {
  clear(sbWorkspace);
  sbWorkspace.append(h('span', { class: 'mono', text: s.workspace?.root ?? 'no workspace' }));

  clear(sbCounts);
  if (s.workspace) {
    sbCounts.append(h('span', { text: `${s.files?.length ?? 0} files · ${s.workspace.totalRequests ?? 0} requests · env ${s.activeEnv || '—'}` }));
  }

  clear(sbFile);
  if (s.selectedFile) {
    const requestCount = s.parsed?.requests?.length ?? 0;
    sbFile.append(h('span', { text: `${s.selectedFile} (${requestCount})` }));
  }

  clear(sbDirty);
  if (s.source.dirty) sbDirty.append(pill('unsaved', 'warning'));
  if (s.running.active) sbDirty.append(pill(s.running.current ? `running ${s.running.current}` : 'running…', 'info'));

  clear(sbSocket);
  sbSocket.append(
    h('span', { class: ['dot', s.socketConnected ? 'ok' : ''].filter(Boolean).join(' ') }),
    h('span', { text: s.socketConnected ? 'live' : 'offline' }),
  );

  clear(sbVersion);
  sbVersion.append(h('span', { text: `studio ${s.appInfo?.version ?? ''}` }));
}

function render(s) {
  if (s.appInfo?.platform === 'darwin') document.documentElement.classList.add('platform-darwin');

  updateTopbar(s);
  updateStatusbar(s);

  const viewId = activeViewId(s);
  const showSidebar = viewId === 'workspace';
  sidebarEl.classList.toggle('hidden', !showSidebar);
  if (showSidebar) {
    if (!sidebar) sidebar = createSidebar(sidebarEl);
    sidebar.update(s);
  }

  if (currentViewId !== viewId) {
    currentView?.destroy?.();
    currentViewId = viewId;
    clear(contentEl);
    currentView = viewId === 'welcome'
      ? createWelcomeView(contentEl)
      : viewById(viewId).create(contentEl);
  }
  withPreservedFocus(() => currentView.update(s));
}

subscribe(render);

/* -------------------------------------------------------------------------- */
/* workspace (project) switcher                                                */
/* -------------------------------------------------------------------------- */

function openWorkspaceSwitcher() {
  window.hitspec.getRecents().then((recents) => {
    const list = recents ?? [];
    const current = state.workspace?.root ?? state.status?.workDir ?? null;

    const build = (refresh) => {
      const rows = list.map((entry) => {
        const active = entry.path === current;
        return h('div', { class: 'switcher-row' },
          h('button', {
            class: ['switcher-main', active ? 'active' : ''].filter(Boolean),
            type: 'button',
            title: entry.path,
            onClick: () => {
              close();
              if (!active) actions.openWorkspacePath(entry.path);
            },
          },
            icon('folder'),
            h('span', { class: 'meta' },
              h('span', { class: 'name', text: entry.name }),
              h('span', { class: 'path', text: entry.path })),
            active
              ? pill('active', 'accent')
              : h('span', { class: 'when', text: entry.openedAt ? relativeTime(entry.openedAt) : '' })),
          h('button', {
            class: 'btn sm ghost icon',
            type: 'button',
            title: 'Remove from recent projects',
            onClick: async (event) => {
              event.stopPropagation();
              await window.hitspec.removeRecent(entry.path);
              const index = list.findIndex((r) => r.path === entry.path);
              if (index >= 0) list.splice(index, 1);
              await actions.refreshSettings();
              refresh();
            },
          }, icon('close', { size: 12 })));
      });

      return h('div', {},
        h('div', { class: 'section-title', style: { padding: '6px 10px 4px' }, text: 'Recent projects' }),
        rows.length ? h('div', {}, rows) : h('div', { class: 'palette-empty', text: 'No recent projects yet.' }),
        h('div', { class: 'popover-foot' },
          btn('Open folder…', { size: 'sm', variant: 'primary', onClick: () => { close(); actions.openWorkspaceDialog(); } }),
          current
            ? btn('Close workspace', { size: 'sm', variant: 'danger', onClick: () => { close(); actions.closeWorkspace(); } })
            : null));
    };

    const close = openPopover(workspaceChip, build, { width: 360 });
  });
}

/* -------------------------------------------------------------------------- */
/* palette                                                                     */
/* -------------------------------------------------------------------------- */

const THEMES = ['nord', 'catppuccin', 'dracula', 'tokyonight', 'gruvbox'];

function showPalette() {
  const items = [];
  const s = state;

  for (const view of VIEWS) {
    items.push({ section: 'Go to', title: view.label, hint: `⌘${view.num}`, run: () => actions.setView(view.id) });
  }

  items.push(
    { section: 'Workspace', title: 'Open workspace…', hint: '⌘O', run: () => actions.openWorkspaceDialog() },
    { section: 'Workspace', title: 'Rescan files', run: () => actions.loadWorkspace() },
    { section: 'Workspace', title: 'Reveal workspace in file manager', run: () => window.hitspec.openRoot() },
    { section: 'Workspace', title: 'Close workspace', run: () => actions.closeWorkspace() },
    { section: 'Workspace', title: 'Restart API server', hint: '⌘⇧R', run: () => actions.restartBackend() },
  );

  for (const entry of s.settings?.recents ?? []) {
    if (entry.path === (s.workspace?.root ?? null)) continue;
    items.push({
      section: 'Switch project',
      title: entry.name,
      hint: entry.openedAt ? relativeTime(entry.openedAt) : undefined,
      keywords: entry.path,
      run: () => actions.openWorkspacePath(entry.path),
    });
  }

  if (s.selectedFile) {
    items.push(
      { section: 'File', title: 'Run request', hint: '⌘↵', keywords: s.selectedRequest ?? '', run: () => actions.runRequest() },
      { section: 'File', title: 'Run whole file', hint: '⌘⇧↵', keywords: s.selectedFile, run: () => actions.runFile() },
      { section: 'File', title: 'Edit source', hint: 'e', run: () => currentView?.focusEditor?.() },
      { section: 'File', title: 'Save file', hint: '⌘S', run: () => actions.saveSource() },
      { section: 'File', title: 'Revert file', run: () => actions.revertSource() },
      { section: 'File', title: 'Export request as curl', run: () => actions.exportCurl() },
      { section: 'File', title: 'Reveal file in file manager', run: () => actions.revealCurrentFile() },
      { section: 'File', title: 'Open file in external editor', run: () => actions.openCurrentFileInEditor() },
      { section: 'File', title: 'New file…', hint: '⌘N', run: () => actions.newFileDialog() },
      { section: 'File', title: 'Delete file…', run: () => actions.deleteCurrentFile() },
    );
  }

  for (const env of s.environments ?? []) {
    items.push({
      section: 'Environment',
      title: `Switch to ${env.name}`,
      hint: env.name === s.activeEnv ? 'active' : undefined,
      run: () => actions.setActiveEnvironment(env.name),
    });
  }
  items.push({ section: 'Environment', title: 'Edit environment variables', run: () => actions.editEnvironmentDialog(s.activeEnv) });
  items.push({ section: 'Environment', title: 'New environment…', run: () => actions.newEnvironmentDialog() });

  for (const file of s.files ?? []) {
    items.push({ section: 'Files', title: file.relativePath, hint: `${file.requestCount} req`, run: () => actions.openFile(file.relativePath) });
  }

  for (const request of s.parsed?.requests ?? []) {
    items.push({
      section: `Requests in ${s.selectedFile}`,
      title: request.name || request.url,
      hint: request.method,
      keywords: request.url,
      run: () => { actions.selectRequest(request.name); actions.setView('workspace'); },
    });
  }

  for (const theme of THEMES) {
    items.push({ section: 'Theme', title: theme, hint: theme === s.settings?.theme ? 'active' : undefined, run: () => actions.applyTheme(theme) });
  }

  items.push(
    { section: 'Help', title: 'Keyboard shortcuts', hint: 'F1', run: () => showShortcuts() },
    { section: 'Help', title: '.http syntax reference', run: () => showSyntax() },
    { section: 'Help', title: 'Quit hitspec Studio', run: () => window.hitspec.quit() },
  );

  openPalette(items);
}

/* -------------------------------------------------------------------------- */
/* help panels                                                                 */
/* -------------------------------------------------------------------------- */

const SHORTCUTS = [
  ['⌘/Ctrl + ↵', 'Run the selected request'],
  ['⌘/Ctrl + ⇧ + ↵', 'Run every request in the file'],
  ['⌘/Ctrl + S', 'Save the source editor'],
  ['⌘/Ctrl + ⇧ + S', 'Revert unsaved changes'],
  ['⌘/Ctrl + N', 'New file'],
  ['⌘/Ctrl + O', 'Open workspace'],
  ['⌘/Ctrl + K', 'Command palette'],
  ['⌘/Ctrl + ⇧ + R', 'Restart the local API server'],
  ['⌘/Ctrl + 1…8', 'Switch view'],
  ['e', 'Focus the source editor'],
  ['?', 'This dialog'],
  ['Esc', 'Close dialog / palette'],
];

function showShortcuts() {
  const close = modal({
    title: 'Keyboard shortcuts',
    body: h('dl', { class: 'help-grid' },
      h('dl', {}, SHORTCUTS.flatMap(([keys, label]) => [
        h('dt', {}, h('span', { class: 'kbd', text: keys })),
        h('dd', { text: label }),
      ]))),
    actions: [btn('Close', { variant: 'primary', onClick: () => close() })],
  });
}

const SYNTAX_REFERENCE = `@baseUrl = https://api.example.com
@token = {{$uuid()}}

### Create user
# @name createUser
# @tags smoke, users
# @timeout 5000
# @auth bearer {{token}}
POST {{baseUrl}}/users
Content-Type: application/json
Accept: application/json

{
  "name": "Ada",
  "email": "ada@example.com"
}

>>>
expect status 201
expect body.name == "Ada"
expect body.id exists
expect body.tags length >= 1
<<<

>>>capture
userId from body.id
<<<

### Delete user
# @depends createUser
DELETE {{baseUrl}}/users/{{userId}}

>>>
expect status 204
<<<
`;

function showSyntax() {
  const close = modal({
    title: '.http syntax reference',
    wide: true,
    body: h('div', { class: 'stack' },
      h('p', { class: 'muted', style: { margin: 0, lineHeight: 1.6 },
        text: 'Files are plain text and stay in git. The desktop app edits exactly these bytes — nothing is stored in a proprietary project format.' }),
      codeBlock(SYNTAX_REFERENCE, { lang: 'text', wrap: true })),
    actions: [
      btn('Copy example', { onClick: () => actions.copyText(SYNTAX_REFERENCE, 'Example copied') }),
      btn('Close', { variant: 'primary', onClick: () => close() }),
    ],
  });
}

/* -------------------------------------------------------------------------- */
/* menu commands from the main process                                         */
/* -------------------------------------------------------------------------- */

const MENU_COMMANDS = {
  'view:workspace': () => actions.setView('workspace'),
  'view:history': () => actions.setView('history'),
  'view:stress': () => actions.setView('stress'),
  'view:mock': () => actions.setView('mock'),
  'view:record': () => actions.setView('record'),
  'view:import': () => actions.setView('import'),
  'view:contract': () => actions.setView('contract'),
  'view:settings': () => actions.setView('settings'),
  'run:request': () => actions.runRequest(),
  'run:file': () => actions.runFile(),
  'file:save': () => actions.saveSource(),
  'file:revert': () => actions.revertSource(),
  'file:new': () => actions.newFileDialog(),
  'workspace:open': () => actions.openWorkspaceDialog(),
  'workspace:openPath': (payload) => actions.openWorkspacePath(payload),
  'workspace:close': () => actions.closeWorkspace(),
  'workspace:revealRoot': () => window.hitspec.openRoot(),
  'backend:restart': () => actions.restartBackend(),
  'palette:toggle': () => showPalette(),
  'help:shortcuts': () => showShortcuts(),
  'help:syntax': () => showSyntax(),
};

/* -------------------------------------------------------------------------- */
/* keyboard                                                                    */
/* -------------------------------------------------------------------------- */

function isTyping(target) {
  if (!target) return false;
  const tag = target.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable === true;
}

document.addEventListener('keydown', (event) => {
  const mod = event.metaKey || event.ctrlKey;
  const key = event.key.toLowerCase();

  if (mod && key === 'k') {
    event.preventDefault();
    showPalette();
    return;
  }
  if (mod && event.key === 'Enter') {
    event.preventDefault();
    if (event.shiftKey) actions.runFile();
    else actions.runRequest();
    return;
  }
  if (mod && key === 's') {
    event.preventDefault();
    if (event.shiftKey) actions.revertSource();
    else actions.saveSource();
    return;
  }
  if (mod && !event.shiftKey && /^[1-8]$/.test(event.key)) {
    event.preventDefault();
    actions.setView(VIEWS[Number(event.key) - 1].id);
    return;
  }

  if (isTyping(document.activeElement)) return;

  if (key === 'e' && state.selectedFile) {
    event.preventDefault();
    actions.setView('workspace');
    currentView?.focusEditor?.();
    return;
  }
  if (event.key === '?') {
    event.preventDefault();
    showShortcuts();
    return;
  }
  if (event.key === 'Escape') {
    closeTopOverlay();
  }
});

/* -------------------------------------------------------------------------- */
/* bridge wiring                                                               */
/* -------------------------------------------------------------------------- */

window.hitspec.onStatus((status) => { actions.handleStatus(status); });
window.hitspec.onSocket(({ connected }) => {
  set({ socketConnected: Boolean(connected) });
});
window.hitspec.onEvent((event) => {
  state.socketConnected = true;
  actions.handleBackendEvent(event);
});
window.hitspec.onLog((line) => { actions.appendLog(line); });
window.hitspec.onToast((payload) => {
  toast(payload?.message ?? '', { severity: payload?.severity ?? 'info', title: payload?.title });
});
window.hitspec.onWorkspace(async () => {
  await actions.refreshSettings();
  await actions.loadWorkspace();
});
window.hitspec.onMenu(({ command, payload } = {}) => {
  const handler = MENU_COMMANDS[command];
  if (!handler) {
    console.warn('[hitspec] unhandled menu command:', command);
    return;
  }
  handler(payload);
});

window.addEventListener('error', (event) => {
  toast(event.message || 'Unexpected renderer error', { severity: 'error', title: 'Renderer error', timeout: 0 });
});
window.addEventListener('unhandledrejection', (event) => {
  const message = event.reason?.message ?? String(event.reason ?? 'unknown');
  toast(message, { severity: 'error', title: 'Unhandled error' });
});

/* -------------------------------------------------------------------------- */
/* boot                                                                        */
/* -------------------------------------------------------------------------- */

render(state);

actions.bootstrap()
  .then(() => {
    if (state.appInfo?.platform === 'darwin') document.documentElement.classList.add('platform-darwin');
    const savedView = state.settings?.view;
    if (savedView && viewById(savedView) && state.status?.state === 'ready') actions.setView(savedView);
  })
  .catch((err) => {
    toast(err?.message ?? String(err), { severity: 'error', title: 'Startup failed', timeout: 0 });
  });
