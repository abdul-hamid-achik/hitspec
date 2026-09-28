/** Central renderer state + a minimal pub/sub. */

export const state = {
  bootstrapped: false,
  appInfo: null,
  status: { state: 'idle', error: null, workDir: null, port: null, version: '', binary: null, binarySource: null },
  socketConnected: false,
  settings: { theme: 'nord' },

  view: 'workspace',
  workspace: null,
  files: [],
  environments: [],
  activeEnv: '',
  config: null,
  systemInfo: null,

  filter: { files: '', requests: '' },
  expandedDirs: {},

  selectedFile: null,
  parsed: null,
  selectedRequest: null,
  source: { path: null, text: '', saved: '', dirty: false, loading: false },

  detailTab: 'request',
  responseTab: 'body',
  compactBody: false,

  running: { active: false, execId: null, file: null, total: 0, index: 0, current: null },
  lastRun: null,
  selectedResult: null,
  busy: {},

  history: { runs: [], total: 0, limit: 30, offset: 0, selected: null, loading: false },
  stress: {
    running: false,
    status: null,
    result: null,
    series: [],
    profiles: [],
    form: { files: [], duration: '10s', rate: 50, vus: 0, maxVUs: 0 },
  },
  mock: { status: null, requests: [], form: { files: [], port: 3000, delay: '' } },
  record: { status: null, form: { targetUrl: '', port: 8081, deduplicate: true } },
  contract: { files: [], results: null, form: { files: [], providerUrl: '' } },
  importDraft: { kind: 'curl', input: '', specPath: '', baseUrl: '', result: null, target: '' },

  logs: [],
  events: [],
};

const subscribers = new Set();

export function subscribe(fn) {
  subscribers.add(fn);
  return () => subscribers.delete(fn);
}

export function emit() {
  for (const fn of [...subscribers]) {
    try {
      fn(state);
    } catch (err) {
      console.error('[hitspec] subscriber failed:', err);
    }
  }
}

/** Shallow-merge a patch into state and notify subscribers. */
export function set(patch) {
  Object.assign(state, patch);
  emit();
  return state;
}

/** Mutate a nested slice in place, then notify. */
export function update(key, mutator) {
  const next = mutator({ ...state[key] });
  state[key] = next;
  emit();
  return next;
}

export function setBusy(name, value) {
  state.busy = { ...state.busy, [name]: value };
  emit();
}

export function selectFile(relPath) {
  state.selectedFile = relPath;
  state.selectedRequest = null;
  state.lastRun = null;
  state.selectedResult = null;
  state.detailTab = 'request';
  emit();
}

export function isReady() {
  return state.status?.state === 'ready';
}
