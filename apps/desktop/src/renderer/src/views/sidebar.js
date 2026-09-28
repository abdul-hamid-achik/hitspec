import * as actions from '../actions.js';
import { h, clear } from '../dom.js';
import { matches } from '../format.js';
import { state, set, emit } from '../store.js';
import { btn, methodBadge } from '../ui.js';

/** Flatten the workspace tree into {node, depth} pairs honouring expansion. */
function visibleNodes(nodes, expanded, depth = 0, out = []) {
  for (const node of nodes ?? []) {
    out.push({ node, depth });
    if (node.isDir && expanded[node.path]) {
      visibleNodes(node.children, expanded, depth + 1, out);
    }
  }
  return out;
}

function nodeMatchesFilter(node, filter, expanded) {
  if (!filter) return true;
  if (!node.isDir) return matches(node.path, filter);
  const anyChild = (node.children ?? []).some((c) => nodeMatchesFilter(c, filter, expanded));
  return matches(node.name, filter) || anyChild;
}

function filterTree(nodes, filter) {
  if (!filter) return nodes ?? [];
  const out = [];
  for (const node of nodes ?? []) {
    if (node.isDir) {
      const children = filterTree(node.children, filter);
      if (children.length || matches(node.name, filter)) out.push({ ...node, children });
    } else if (matches(node.path, filter)) {
      out.push(node);
    }
  }
  return out;
}

export function createSidebar(root) {
  const fileSearch = h('input', { class: 'input', type: 'search', placeholder: 'Filter files…', ariaLabel: 'Filter files' });
  const requestSearch = h('input', { class: 'input', type: 'search', placeholder: 'Filter requests…', ariaLabel: 'Filter requests' });

  fileSearch.addEventListener('input', () => {
    state.filter = { ...state.filter, files: fileSearch.value };
    if (fileSearch.value) {
      // Auto-expand everything while filtering so matches are visible.
      state.expandedDirs = Object.fromEntries((state.workspace?.files ?? []).flatMap(collectDirs).map((p) => [p, true]));
    }
    emit();
  });
  requestSearch.addEventListener('input', () => {
    state.filter = { ...state.filter, requests: requestSearch.value };
    emit();
  });

  const fileCount = h('span', { class: 'count' });
  const fileList = h('div', { class: 'tree scroll' });
  const requestCount = h('span', { class: 'count' });
  const requestList = h('div', { class: 'tree scroll' });

  const filesPane = h(
    'div',
    { class: 'pane files' },
    h(
      'div',
      { class: 'pane-head' },
      h('span', { text: 'Files' }),
      fileCount,
      h('div', { class: 'row' },
        btn('+', { size: 'sm', variant: 'ghost', title: 'New file', onClick: () => actions.newFileDialog() }),
        btn('⟳', { size: 'sm', variant: 'ghost', title: 'Rescan workspace', onClick: () => actions.loadWorkspace() })),
    ),
    h('div', { class: 'search' }, fileSearch),
    fileList,
  );

  const requestsPane = h(
    'div',
    { class: 'pane requests' },
    h(
      'div',
      { class: 'pane-head' },
      h('span', { text: 'Requests' }),
      requestCount,
      btn('Run file', {
        size: 'sm',
        variant: 'ghost',
        title: 'Run every request in this file (⌘⇧↵)',
        onClick: () => actions.runFile(),
      }),
    ),
    h('div', { class: 'search' }, requestSearch),
    requestList,
  );

  clear(root);
  root.append(filesPane, requestsPane);

  function update(s) {
    if (fileSearch.value !== s.filter.files && document.activeElement !== fileSearch) {
      fileSearch.value = s.filter.files;
    }
    if (requestSearch.value !== s.filter.requests && document.activeElement !== requestSearch) {
      requestSearch.value = s.filter.requests;
    }

    const tree = filterTree(s.workspace?.files ?? [], s.filter.files);
    const nodes = visibleNodes(tree, s.filter.files ? expandAll(tree) : s.expandedDirs);
    fileCount.textContent = `${s.files?.length ?? 0}`;

    clear(fileList);
    if (!nodes.length) {
      fileList.append(
        h('div', { class: 'muted', style: { padding: '10px 8px' }, text: s.filter.files ? 'No files match the filter.' : 'No .http files in this workspace yet.' }),
      );
    }
    for (const { node, depth } of nodes) {
      fileList.append(renderNode(node, depth, s));
    }

    const requests = (s.parsed?.requests ?? []).filter(
      (r) => matches(r.name, s.filter.requests) || matches(r.url, s.filter.requests),
    );
    requestCount.textContent = `${requests.length}`;
    clear(requestList);
    if (!s.selectedFile) {
      requestList.append(h('div', { class: 'muted', style: { padding: '10px 8px' }, text: 'Select a file to see its requests.' }));
      return;
    }
    if (!requests.length) {
      requestList.append(h('div', { class: 'muted', style: { padding: '10px 8px' }, text: 'No requests in this file.' }));
      return;
    }
    const resultsByName = new Map((s.lastRun?.results ?? []).map((r) => [r.name, r]));
    for (const request of requests) {
      const result = resultsByName.get(request.name);
      const tone = result ? (result.passed ? 'passed' : result.skipped ? '' : 'failed') : '';
      requestList.append(
        h(
          'button',
          {
            class: ['req-row', request.name === s.selectedRequest ? 'active' : '', tone].filter(Boolean),
            title: `${request.method} ${request.url}`,
            onClick: () => actions.selectRequest(request.name),
            onDblclick: () => actions.runRequest(),
          },
          methodBadge(request.method),
          h('span', { class: 'req-name', text: request.name || request.url }),
          result ? h('span', { class: ['dot', result.passed ? 'ok' : 'bad'].filter(Boolean) }) : null,
          (request.tags ?? []).length
            ? h('span', { class: 'req-tags' }, request.tags.slice(0, 2).map((t) => h('span', { class: 'tag', text: t })))
            : null,
        ),
      );
    }
  }

  return { update };
}

function collectDirs(nodes, out = []) {
  for (const node of nodes ?? []) {
    if (node.isDir) {
      out.push(node.path);
      collectDirs(node.children, out);
    }
  }
  return out;
}

function expandAll(nodes) {
  return Object.fromEntries(collectDirs(nodes).map((p) => [p, true]));
}

function renderNode(node, depth, s) {
  if (node.isDir) {
    const expanded = Boolean(s.expandedDirs[node.path]) || Boolean(s.filter.files);
    return h(
      'button',
      {
        class: ['tree-row', 'dir', expanded ? 'expanded' : ''].filter(Boolean),
        style: { paddingLeft: `${7 + depth * 12}px` },
        onClick: () => set({ expandedDirs: { ...s.expandedDirs, [node.path]: !expanded } }),
      },
      h('span', { class: 'caret', text: expanded ? '▾' : '▸' }),
      h('span', { class: 'icon', text: '▸' }),
      h('span', { class: 'label', text: node.name }),
      node.children?.length ? h('span', { class: 'badge-count', text: String(countRequests(node)) }) : null,
    );
  }

  const isActive = node.path === s.selectedFile;
  return h(
    'button',
    {
      class: ['tree-row', isActive ? 'active' : ''].filter(Boolean),
      style: { paddingLeft: `${7 + depth * 12}px` },
      title: node.path,
      onClick: () => actions.openFile(node.path),
    },
    h('span', { class: 'caret' }),
    h('span', { class: 'icon', text: '≡' }),
    h('span', { class: 'label', text: node.name }),
    node.requestCount ? h('span', { class: 'badge-count', text: String(node.requestCount) }) : null,
  );
}

function countRequests(node) {
  if (!node.isDir) return node.requestCount ?? 0;
  return (node.children ?? []).reduce((sum, child) => sum + countRequests(child), 0);
}
