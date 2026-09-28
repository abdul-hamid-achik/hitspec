import * as actions from '../actions.js';
import { h, clear } from '../dom.js';
import { relativeTime } from '../format.js';
import { btn } from '../ui.js';

/** Shown when no workspace is open: pick a folder, or resume a recent one. */
export function createWelcomeView(root) {
  const el = h('div', { class: 'welcome' });
  clear(root);
  root.append(el);

  function update(s) {
    clear(el);
    const recents = s.settings?.recents ?? [];

    el.append(
      h('div', { class: 'welcome-card' },
        h('div', { class: 'row', style: { gap: '10px' } },
          h('div', { class: 'brand-mark', style: { width: '30px', height: '30px', fontSize: '15px', borderRadius: '8px' }, text: 'h' }),
          h('h1', { text: 'hitspec Studio' })),
        h('p', { class: 'lede', text: 'A desktop client for plain-text HTTP API tests. Point it at a folder of .http files to browse, edit and run them — the same files your CI runs, no proprietary project format.' }),

        s.status?.error
          ? h('div', { class: 'card', style: { borderColor: 'var(--danger)', marginBottom: '18px' } },
              h('div', { class: 'card-head' }, h('h3', { class: 'danger', text: 'API server could not start' })),
              h('div', { class: 'card-body' },
                h('pre', { class: 'code plain', style: { maxHeight: '180px', color: 'var(--danger)' }, text: s.status.error }),
                h('div', { class: 'row', style: { marginTop: '10px' } },
                  btn('Retry', { variant: 'primary', onClick: () => actions.restartBackend() }),
                  btn('Copy error', { onClick: () => actions.copyText(s.status.error, 'Error copied') }))))
          : null,

        h('div', { class: 'row', style: { marginBottom: '18px' } },
          btn('Open workspace…', { variant: 'primary', kbd: '⌘O', onClick: () => actions.openWorkspaceDialog() }),
          s.status?.state === 'starting' ? h('span', { class: 'row' }, h('span', { class: 'spinner' }), h('span', { class: 'muted', text: 'Starting API server…' })) : null),

        recents.length
          ? h('div', { class: 'stack-sm' },
              h('div', { class: 'section-title', text: 'Recent workspaces' }),
              h('div', { class: 'recent-list' },
                recents.map((entry) =>
                  h('div', { class: 'recent' },
                    h('button', {
                      class: 'meta',
                      style: { textAlign: 'left', flex: '1 1 auto', minWidth: 0 },
                      title: entry.path,
                      onClick: () => actions.openWorkspacePath(entry.path),
                    },
                      h('span', { class: 'name', text: entry.name }),
                      h('span', { class: 'path', text: entry.path })),
                    h('span', { class: 'when', text: entry.openedAt ? relativeTime(entry.openedAt) : '' }),
                    btn('Open', { size: 'sm', onClick: () => actions.openWorkspacePath(entry.path) }),
                    btn('✕', { size: 'sm', variant: 'ghost', title: 'Remove from recents', onClick: () => removeRecent(entry.path) }))))
            )
          : h('div', { class: 'muted', text: 'No recent workspaces yet.' }),

        h('hr', { class: 'sep' }),
        h('div', { class: 'help-grid' },
          h('div', {},
            h('div', { class: 'section-title', text: 'Tips' }),
            h('ul', { class: 'muted', style: { margin: 0, paddingLeft: '18px', lineHeight: 1.9 } },
              h('li', { text: 'Drop a folder onto the Dock icon to open it as a workspace.' }),
              h('li', { text: 'Files edited elsewhere reload automatically — the app watches the workspace.' }),
              h('li', { text: '⌘K opens the command palette for everything below.' }))),
          h('div', {},
            h('div', { class: 'section-title', text: 'What you can do here' }),
            h('ul', { class: 'muted', style: { margin: 0, paddingLeft: '18px', lineHeight: 1.9 } },
              h('li', { text: 'Run single requests or whole files, with assertions and captures' }),
              h('li', { text: 'Switch and edit environments (written back to hitspec.yaml)' }),
              h('li', { text: 'Stress test, mock server, recording proxy, contract checks' }),
              h('li', { text: 'Browse the persistent SQLite run history' }))))),
    );
  }

  async function removeRecent(dirPath) {
    await window.hitspec.removeRecent(dirPath);
    await actions.refreshSettings();
  }

  return { update };
}
