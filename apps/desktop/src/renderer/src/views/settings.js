import * as actions from '../actions.js';
import { h, clear } from '../dom.js';
import { formatDateTime } from '../format.js';
import { btn, card, kv, stat, statGrid } from '../ui.js';

const THEMES = [
  { id: 'nord', label: 'Nord', colors: ['#2E3440', '#88C0D0', '#A3BE8C'] },
  { id: 'catppuccin', label: 'Catppuccin Mocha', colors: ['#1E1E2E', '#89B4FA', '#A6E3A1'] },
  { id: 'dracula', label: 'Dracula', colors: ['#282A36', '#BD93F9', '#50FA7B'] },
  { id: 'tokyonight', label: 'Tokyo Night', colors: ['#1A1B26', '#7AA2F7', '#9ECE6A'] },
  { id: 'gruvbox', label: 'Gruvbox Dark', colors: ['#282828', '#83A598', '#B8BB26'] },
];

const SHORTCUTS = [
  ['Run request', '⌘ / Ctrl + ↵'],
  ['Run file', '⌘ / Ctrl + ⇧ + ↵'],
  ['Save file', '⌘ / Ctrl + S'],
  ['Revert file', '⌘ / Ctrl + ⇧ + S'],
  ['New file', '⌘ / Ctrl + N'],
  ['Open workspace', '⌘ / Ctrl + O'],
  ['Command palette', '⌘ / Ctrl + K'],
  ['Restart API server', '⌘ / Ctrl + ⇧ + R'],
  ['Switch view 1–8', '⌘ / Ctrl + 1…8'],
  ['Edit source', 'e (in the file tree)'],
  ['Close dialog', 'Esc'],
];

export function createSettingsView(root) {
  const head = h('div', { class: 'view-head' });
  const body = h('div', { class: 'view-body' });
  const logBox = h('div', { class: 'scroll', style: { maxHeight: '280px', background: 'var(--bg)', borderRadius: 'var(--radius)', border: '1px solid var(--border-soft)' } });
  clear(root);
  root.append(h('div', { class: 'view' }, head, body));

  const logLevelSelect = h('select', { class: 'select', ariaLabel: 'Log level' },
    ['debug', 'info', 'warn', 'error'].map((level) => h('option', { value: level, text: level })));
  logLevelSelect.addEventListener('change', () => actions.setBackendOption('logLevel', logLevelSelect.value));

  const watchBox = h('input', { type: 'checkbox' });
  const readOnlyBox = h('input', { type: 'checkbox' });
  const allowShellBox = h('input', { type: 'checkbox' });
  watchBox.addEventListener('change', () => actions.setBackendOption('watch', watchBox.checked));
  readOnlyBox.addEventListener('change', () => actions.setBackendOption('readOnly', readOnlyBox.checked));
  allowShellBox.addEventListener('change', () => actions.setBackendOption('allowShell', allowShellBox.checked));

  let renderedLogCount = -1;

  function update(s) {
    const backend = s.settings?.backend ?? {};
    if (document.activeElement !== logLevelSelect) logLevelSelect.value = backend.logLevel ?? 'info';
    watchBox.checked = backend.watch !== false;
    readOnlyBox.checked = Boolean(backend.readOnly);
    allowShellBox.checked = Boolean(backend.allowShell);

    clear(head);
    head.append(
      h('h2', { text: 'Settings' }),
      h('div', { class: 'spacer' }),
      btn('Refresh', { size: 'sm', onClick: () => actions.loadLogs() }),
    );

    clear(body);
    body.append(
      h('div', { class: 'stack' },
        card('Appearance',
          h('div', { class: 'row wrap', style: { gap: '8px' } },
            THEMES.map((theme) =>
              h('button', {
                class: ['btn', theme.id === (s.settings?.theme ?? 'nord') ? 'primary' : ''].filter(Boolean),
                title: theme.label,
                onClick: () => actions.applyTheme(theme.id),
              },
                h('span', { class: 'row', style: { gap: '3px' } },
                  theme.colors.map((color) => h('i', { style: { width: '9px', height: '9px', borderRadius: '2px', background: color, display: 'inline-block' } }))),
                h('span', { text: theme.label })))),
          { sub: 'Matches the themes available in the terminal UI' }),

        card('Workspace',
          s.workspace
            ? h('div', { class: 'stack' },
                statGrid([
                  stat('Files', String(s.files?.length ?? 0)),
                  stat('Requests', String(s.workspace.totalRequests ?? 0), 'accent'),
                  stat('Environment', s.activeEnv || '—'),
                  stat('hitspec.yaml', s.workspace.hasConfig ? 'found' : 'missing', s.workspace.hasConfig ? 'success' : 'warning'),
                ]),
                kv([['Root', s.workspace.root]]),
                h('div', { class: 'row wrap' },
                  btn('Reveal in file manager', { size: 'sm', onClick: () => window.hitspec.openRoot() }),
                  btn('Rescan files', { size: 'sm', onClick: () => actions.loadWorkspace() }),
                  btn('Close workspace', { size: 'sm', variant: 'danger', onClick: () => actions.closeWorkspace() })))
            : h('div', { class: 'muted', text: 'No workspace is open.' })),

        card('API server',
          h('div', { class: 'stack' },
            statGrid([
              stat('State', s.status?.state ?? 'idle', s.status?.state === 'ready' ? 'success' : s.status?.state === 'error' ? 'danger' : ''),
              stat('Version', s.systemInfo?.version || s.status?.version || '—'),
              stat('Port', s.status?.port ? String(s.status.port) : '—'),
              stat('PID', s.status?.pid ? String(s.status.pid) : '—'),
            ]),
            kv([
              ['Binary', s.status?.binary ?? '—'],
              ['Resolved from', s.status?.binarySource ?? '—'],
              ['Go', s.systemInfo ? `${s.systemInfo.goVersion} · ${s.systemInfo.os}/${s.systemInfo.arch}` : '—'],
            ]),
            s.status?.error ? h('pre', { class: 'code plain', style: { color: 'var(--danger)', maxHeight: '160px' }, text: s.status.error }) : null,
            h('div', { class: 'row wrap' },
              h('label', { class: 'checkbox' }, watchBox, h('span', { text: 'Watch files' })),
              h('label', { class: 'checkbox' }, readOnlyBox, h('span', { text: 'Read-only (block writes)' })),
              h('label', { class: 'checkbox', title: 'Allows shell commands in .http files — only enable this for workspaces you trust' },
                allowShellBox, h('span', { text: 'Allow shell execution' })),
              h('div', { class: 'field', style: { width: '130px' } }, h('label', { text: 'Log level' }),
                h('div', { class: 'select-wrap' }, logLevelSelect))),
            h('div', { class: 'row' },
              btn('Restart API server', { variant: 'primary', busy: Boolean(s.busy.backend), onClick: () => actions.restartBackend() }),
              h('span', { class: 'muted', style: { fontSize: '11px' }, text: 'Applies the options above; runs are not affected otherwise.' }))),
          { sub: 'hitspec serve --api-only, spawned locally with a random port and token' }),

        card('Keyboard shortcuts',
          h('dl', {}, SHORTCUTS.flatMap(([label, keys]) => [
            h('dt', { style: { display: 'flex', gap: '6px', alignItems: 'center' } }, h('span', { class: 'kbd', text: keys })),
            h('dd', { class: 'muted', style: { margin: '0 0 6px' }, text: label }),
          ])),
          { bodyClass: 'card-body' }),

        card('Server logs',
          logBox,
          {
            bodyClass: '',
            actions: [
              btn('Copy', { size: 'sm', onClick: () => actions.copyText(s.logs.map((l) => l.text).join('\n'), 'Logs copied') }),
              btn('Refresh', { size: 'sm', onClick: () => actions.loadLogs() }),
            ],
          }),

        card('About',
          kv([
            ['App', `hitspec Studio ${s.appInfo?.version ?? ''}`],
            ['Electron', s.appInfo?.electron ?? '—'],
            ['Chromium', s.appInfo?.chrome ?? '—'],
            ['Node', s.appInfo?.node ?? '—'],
            ['Platform', s.appInfo ? `${s.appInfo.platform}/${s.appInfo.arch}` : '—'],
            ['Build', s.appInfo?.dev ? 'development' : 'packaged'],
            ['Started', formatDateTime(new Date().toISOString())],
          ])),
      ),
    );

    renderLogs(s);
  }

  function renderLogs(s) {
    const logs = s.logs ?? [];
    if (logs.length === renderedLogCount) return;
    const atBottom = logBox.scrollTop + logBox.clientHeight >= logBox.scrollHeight - 24;
    clear(logBox);
    if (!logs.length) {
      logBox.append(h('div', { class: 'muted', style: { padding: '12px' }, text: 'No log lines yet.' }));
    } else {
      logBox.append(...logs.map((line) =>
        h('div', { class: ['log-line', line.stream].filter(Boolean) },
          h('span', { class: 'lvl', text: line.level ?? line.stream }),
          h('span', { text: line.msg ?? line.text }))));
    }
    if (atBottom) logBox.scrollTop = logBox.scrollHeight;
    renderedLogCount = logs.length;
  }

  return { update };
}
