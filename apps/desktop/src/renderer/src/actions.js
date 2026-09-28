import { api } from './api.js';
import { h } from './dom.js';
import { formatDuration } from './format.js';
import { state, set, setBusy, emit } from './store.js';
import { toast, toastError, toastSuccess } from './toast.js';
import { btn, confirmDialog, modal, prompt } from './ui.js';

const bridge = () => window.hitspec;

/* -------------------------------------------------------------------------- */
/* bootstrap                                                                   */
/* -------------------------------------------------------------------------- */

export async function bootstrap() {
  const [settings, appInfo, status] = await Promise.all([
    bridge().getSettings(),
    bridge().getAppInfo(),
    bridge().getStatus(),
  ]);
  set({ settings, appInfo, status });
  applyTheme(settings.theme ?? 'nord');

  if (status?.state === 'ready' && status.workDir) {
    await loadWorkspace();
  }
  set({ bootstrapped: true });
}

export function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  state.settings = { ...state.settings, theme };
  emit();
  bridge().setSettings({ theme }).catch(() => {});
}

export async function handleStatus(status) {
  const previous = state.status?.state;
  set({ status });
  if (status.state === 'ready' && previous !== 'ready') {
    await loadWorkspace();
  }
  if (status.state === 'error' && status.error) {
    toast(status.error, { severity: 'error', title: 'API server problem', timeout: 0 });
  }
}

/* -------------------------------------------------------------------------- */
/* workspace                                                                   */
/* -------------------------------------------------------------------------- */

export async function loadWorkspace() {
  if (state.status?.state !== 'ready') return;
  setBusy('workspace', true);
  try {
    const [workspace, files, environments, config, systemInfo] = await Promise.all([
      api.workspace(),
      api.files(),
      api.environments(),
      api.config().catch(() => null),
      api.systemInfo().catch(() => null),
    ]);
    state.workspace = workspace;
    state.files = files ?? [];
    state.environments = environments ?? [];
    state.config = config;
    state.systemInfo = systemInfo;
    state.activeEnv = workspace.environment || environments?.[0]?.name || 'dev';

    const stillThere = state.selectedFile && state.files.some((f) => f.relativePath === state.selectedFile);
    if (!stillThere) {
      state.selectedFile = state.files[0]?.relativePath ?? null;
      state.parsed = null;
      state.selectedRequest = null;
      state.source = { path: null, text: '', saved: '', dirty: false, loading: false };
    }
    setBusy('workspace', false);
    emit();
    if (state.selectedFile) await openFile(state.selectedFile, { keepSelection: true });
  } catch (err) {
    setBusy('workspace', false);
    toastError(err, { title: 'Could not load workspace' });
  }
}

export async function openWorkspaceDialog() {
  try {
    const status = await bridge().openWorkspace();
    if (!status) return; // cancelled
    if (status.state === 'error') throw new Error(status.error ?? 'could not start API server');
    toastSuccess(`Opened ${status.workDir}`);
    await loadWorkspace();
  } catch (err) {
    toastError(err, { title: 'Could not open workspace' });
  }
}

export async function openWorkspacePath(dirPath) {
  try {
    const status = await bridge().openWorkspacePath(dirPath);
    if (status?.state === 'error') throw new Error(status.error ?? 'could not start API server');
    await loadWorkspace();
  } catch (err) {
    toastError(err, { title: 'Could not open workspace' });
  }
}

export async function closeWorkspace() {
  const ok = await confirmDialog({
    title: 'Close workspace?',
    message: `The API server for ${state.status?.workDir ?? 'this workspace'} will be stopped. Unsaved editor changes are lost.`,
    confirmLabel: 'Close workspace',
  });
  if (!ok) return;
  await bridge().closeWorkspace();
  set({
    workspace: null,
    files: [],
    parsed: null,
    selectedFile: null,
    selectedRequest: null,
    lastRun: null,
    source: { path: null, text: '', saved: '', dirty: false, loading: false },
    view: 'workspace',
  });
}

export async function restartBackend() {
  try {
    setBusy('backend', true);
    const status = await bridge().restartBackend();
    if (status?.state === 'error') throw new Error(status.error ?? 'restart failed');
    toastSuccess('API server restarted');
    await loadWorkspace();
  } catch (err) {
    toastError(err, { title: 'Restart failed' });
  } finally {
    setBusy('backend', false);
  }
}

/* -------------------------------------------------------------------------- */
/* files                                                                       */
/* -------------------------------------------------------------------------- */

export async function openFile(relPath, { keepSelection = false } = {}) {
  if (!relPath) return;
  const previousRequest = keepSelection ? state.selectedRequest : null;
  setBusy('file', true);
  state.source = { ...state.source, loading: true };
  emit();
  try {
    const [parsed, raw] = await Promise.all([api.file(relPath), api.fileRaw(relPath)]);
    state.parsed = parsed;
    state.selectedFile = relPath;
    state.source = { path: relPath, text: raw ?? '', saved: raw ?? '', dirty: false, loading: false };
    state.lastRun = null;
    state.selectedResult = null;
    const names = (parsed.requests ?? []).map((r) => r.name);
    state.selectedRequest =
      previousRequest && names.includes(previousRequest) ? previousRequest : (names[0] ?? null);
    setBusy('file', false);
    emit();
  } catch (err) {
    state.source = { ...state.source, loading: false };
    setBusy('file', false);
    emit();
    toastError(err, { title: `Could not open ${relPath}` });
  }
}

export function selectRequest(name) {
  set({ selectedRequest: name, detailTab: 'request' });
}

export function setSourceText(text) {
  state.source = { ...state.source, text, dirty: text !== state.source.saved };
  emit();
}

export async function saveSource() {
  if (!state.source.path) return;
  if (!state.source.dirty) {
    toast('Nothing to save', { severity: 'info', timeout: 1800 });
    return;
  }
  setBusy('save', true);
  try {
    const result = await api.saveFile(state.source.path, state.source.text);
    state.source = { ...state.source, saved: state.source.text, dirty: false };
    if (result?.requests) {
      state.parsed = result;
      const names = result.requests.map((r) => r.name);
      if (!names.includes(state.selectedRequest)) state.selectedRequest = names[0] ?? null;
    } else {
      state.parsed = await api.file(state.source.path);
    }
    if (result?.warning) toast(result.warning, { severity: 'warning', title: 'Saved with a parse warning' });
    await refreshFileCounts();
    emit();
  } catch (err) {
    toastError(err, { title: 'Save failed' });
  } finally {
    setBusy('save', false);
  }
}

export async function revertSource() {
  if (!state.source.path) return;
  if (state.source.dirty) {
    const ok = await confirmDialog({
      title: 'Revert file?',
      message: 'Unsaved changes in the editor will be discarded.',
      confirmLabel: 'Revert',
      danger: true,
    });
    if (!ok) return;
  }
  await openFile(state.source.path);
}

async function refreshFileCounts() {
  try {
    const [workspace, files] = await Promise.all([api.workspace(), api.files()]);
    state.workspace = workspace;
    state.files = files ?? [];
    emit();
  } catch {
    // counts are cosmetic; ignore
  }
}

export async function newFileDialog() {
  const name = await prompt({
    title: 'New hitspec file',
    label: 'Relative path inside the workspace',
    value: 'requests/new.http',
    confirmLabel: 'Create',
  });
  if (!name) return;
  try {
    setBusy('newFile', true);
    await api.createFile(name, DEFAULT_FILE_TEMPLATE);
    await refreshFileCounts();
    // POST /files appends .http when the extension is missing, so resolve the
    // created file against the refreshed list rather than trusting the input.
    const base = name.split('/').pop();
    const created =
      state.files.find((f) => f.relativePath === name) ??
      state.files.find((f) => f.name === base || f.name === `${base}.http`);
    await openFile(created?.relativePath ?? name);
    toastSuccess(`Created ${created?.relativePath ?? name}`);
  } catch (err) {
    toastError(err, { title: 'Could not create file' });
  } finally {
    setBusy('newFile', false);
  }
}

const DEFAULT_FILE_TEMPLATE = `@baseUrl = https://api.example.com

### Health check
# @name health
GET {{baseUrl}}/health

>>>
expect status 200
<<<
`;

export async function deleteCurrentFile() {
  const relPath = state.selectedFile;
  if (!relPath) return;
  const ok = await confirmDialog({
    title: `Delete ${relPath}?`,
    message: 'The file is removed from disk. This cannot be undone from here.',
    confirmLabel: 'Delete file',
    danger: true,
  });
  if (!ok) return;
  try {
    await api.deleteFile(relPath);
    state.selectedFile = null;
    state.parsed = null;
    state.source = { path: null, text: '', saved: '', dirty: false, loading: false };
    toastSuccess(`Deleted ${relPath}`);
    await loadWorkspace();
  } catch (err) {
    toastError(err, { title: 'Delete failed' });
  }
}

export function revealCurrentFile() {
  if (!state.selectedFile) return;
  bridge().reveal(state.selectedFile).catch((err) => toastError(err));
}

export function openCurrentFileInEditor() {
  if (!state.selectedFile) return;
  bridge().openInEditor(state.selectedFile).catch((err) => toastError(err));
}

/* -------------------------------------------------------------------------- */
/* execution                                                                   */
/* -------------------------------------------------------------------------- */

function summarise(result) {
  if (!result) return '';
  const parts = [`${result.passed} passed`];
  if (result.failed) parts.push(`${result.failed} failed`);
  if (result.skipped) parts.push(`${result.skipped} skipped`);
  parts.push(formatDuration(result.duration));
  return parts.join(' · ');
}

export async function runRequest() {
  if (!state.selectedFile || !state.selectedRequest) {
    toast('Select a request to run', { severity: 'warning' });
    return;
  }
  const file = state.selectedFile;
  const name = state.selectedRequest;
  state.running = { active: true, execId: null, file, total: 1, index: 0, current: name };
  state.detailTab = 'response';
  setBusy('run', true);
  emit();
  try {
    const result = await api.execute(file, name, state.activeEnv);
    state.lastRun = result;
    state.selectedResult = result.results?.[0] ?? null;
    state.detailTab = 'response';
    const failed = (result.results ?? []).some((r) => !r.passed && !r.skipped);
    toast(summarise(result), {
      severity: failed ? 'error' : 'success',
      title: `${name} — ${failed ? 'failed' : 'passed'}`,
    });
  } catch (err) {
    toastError(err, { title: `Run failed: ${name}` });
  } finally {
    state.running = { active: false, execId: null, file: null, total: 0, index: 0, current: null };
    setBusy('run', false);
    emit();
  }
}

export async function runFile() {
  if (!state.selectedFile) {
    toast('Select a file to run', { severity: 'warning' });
    return;
  }
  const file = state.selectedFile;
  const total = state.parsed?.requests?.length ?? 0;
  state.running = { active: true, execId: null, file, total, index: 0, current: null };
  state.detailTab = 'response';
  setBusy('runFile', true);
  emit();
  try {
    const result = await api.runFile(file, state.activeEnv);
    state.lastRun = result;
    const firstFailure = (result.results ?? []).find((r) => !r.passed && !r.skipped);
    state.selectedResult = firstFailure ?? result.results?.[0] ?? null;
    const failed = result.failed > 0;
    toast(summarise(result), {
      severity: failed ? 'error' : 'success',
      title: `${file} — ${failed ? 'failed' : 'passed'}`,
    });
  } catch (err) {
    toastError(err, { title: `Run failed: ${file}` });
  } finally {
    state.running = { active: false, execId: null, file: null, total: 0, index: 0, current: null };
    setBusy('runFile', false);
    emit();
  }
}

export function selectResult(result) {
  set({ selectedResult: result, responseTab: 'body' });
}

/* -------------------------------------------------------------------------- */
/* environments & config                                                       */
/* -------------------------------------------------------------------------- */

export async function setActiveEnvironment(name) {
  if (!name || name === state.activeEnv) return;
  const previous = state.activeEnv;
  state.activeEnv = name;
  emit();
  try {
    await api.setActiveEnvironment(name);
    toast(`Environment: ${name}`, { severity: 'info', timeout: 1800 });
  } catch (err) {
    state.activeEnv = previous;
    emit();
    toastError(err, { title: 'Could not switch environment' });
  }
}

export async function reloadEnvironments() {
  try {
    state.environments = (await api.environments()) ?? [];
    emit();
  } catch (err) {
    toastError(err);
  }
}

/** Editable key/value grid for one environment; persists through hitspec.yaml. */
export function editEnvironmentDialog(name) {
  const env = state.environments.find((e) => e.name === name) ?? { name, variables: {} };
  let rows = Object.entries(env.variables ?? {}).map(([key, value]) => ({ key, value: String(value) }));
  if (!rows.length) rows = [{ key: '', value: '' }];

  const listEl = h('div', { class: 'stack-sm' });

  const renderRows = () => {
    listEl.replaceChildren(
      ...rows.map((row, index) =>
        h(
          'div',
          { class: 'row' },
          h('input', {
            class: 'input mono',
            placeholder: 'name',
            value: row.key,
            style: { flex: '0 0 38%' },
            onInput: (e) => { rows[index].key = e.target.value; },
          }),
          h('input', {
            class: 'input mono',
            placeholder: 'value',
            value: row.value,
            style: { flex: '1 1 auto' },
            onInput: (e) => { rows[index].value = e.target.value; },
          }),
          btn('✕', {
            size: 'sm',
            variant: 'ghost',
            title: 'Remove variable',
            onClick: () => { rows.splice(index, 1); renderRows(); },
          }),
        ),
      ),
    );
  };
  renderRows();

  const close = modal({
    title: `Environment: ${name}`,
    wide: true,
    body: h(
      'div',
      { class: 'stack' },
      h('p', {
        class: 'muted',
        style: { margin: 0 },
        text: 'Saved to hitspec.yaml under environments. Changes apply to the next run.',
      }),
      listEl,
      btn('+ Add variable', {
        size: 'sm',
        onClick: () => { rows.push({ key: '', value: '' }); renderRows(); },
      }),
    ),
    actions: [
      btn('New environment…', {
        onClick: () => {
          close();
          newEnvironmentDialog();
        },
      }),
      btn('Cancel', { onClick: () => close() }),
      btn('Save environment', {
        variant: 'primary',
        onClick: async () => {
          const variables = {};
          for (const row of rows) {
            const key = row.key.trim();
            if (key) variables[key] = row.value;
          }
          try {
            await api.putEnvironment(name, variables);
            await reloadEnvironments();
            toastSuccess(`Saved ${name}`);
            close();
          } catch (err) {
            toastError(err, { title: 'Could not save environment' });
          }
        },
      }),
    ],
  });
}

export async function newEnvironmentDialog() {
  const name = await prompt({ title: 'New environment', label: 'Name', placeholder: 'staging', confirmLabel: 'Create' });
  if (!name) return;
  try {
    await api.putEnvironment(name, {});
    await reloadEnvironments();
    await setActiveEnvironment(name);
    toastSuccess(`Created environment ${name}`);
  } catch (err) {
    toastError(err, { title: 'Could not create environment' });
  }
}

export async function saveConfig(patch) {
  try {
    state.config = await api.putConfig({ ...(state.config ?? {}), ...patch });
    emit();
    toastSuccess('Configuration saved');
  } catch (err) {
    toastError(err, { title: 'Could not save configuration' });
  }
}

/* -------------------------------------------------------------------------- */
/* export                                                                      */
/* -------------------------------------------------------------------------- */

export async function exportCurl(requestName = state.selectedRequest) {
  if (!state.selectedFile) return;
  try {
    const result = await api.exportCurl(state.selectedFile, requestName ?? undefined);
    const commands = result?.commands ?? [];
    if (!commands.length) {
      toast('Nothing to export', { severity: 'warning' });
      return;
    }
    const close = modal({
      title: `curl — ${requestName ?? state.selectedFile}`,
      wide: true,
      body: h(
        'div',
        { class: 'stack' },
        commands.map((command) =>
          h(
            'div',
            { class: 'row' },
            h('pre', { class: 'code plain grow', text: command }),
            btn('Copy', {
              size: 'sm',
              onClick: () => copyText(command),
            }),
          ),
        ),
      ),
      actions: [
        btn('Copy all', { onClick: () => copyText(commands.join('\n\n')) }),
        btn('Close', { variant: 'primary', onClick: () => close() }),
      ],
    });
  } catch (err) {
    toastError(err, { title: 'Export failed' });
  }
}

/* -------------------------------------------------------------------------- */
/* realtime events                                                             */
/* -------------------------------------------------------------------------- */

const MAX_EVENTS = 200;

export function handleBackendEvent(message) {
  if (!message || typeof message !== 'object') return;
  const { type, payload } = message;

  state.events = [{ type, payload, at: message.timestamp ?? new Date().toISOString() }, ...state.events].slice(0, MAX_EVENTS);

  switch (type) {
    case 'file_changed': {
      const relPath = payload?.path;
      const operation = payload?.operation ?? 'changed';
      if (operation === 'deleted' && relPath === state.selectedFile) {
        toast(`${relPath} was deleted outside the app`, { severity: 'warning' });
        state.selectedFile = null;
        state.parsed = null;
      }
      // Reload counts, and the open buffer only when it is clean.
      refreshFileCounts().then(() => {
        if (relPath === state.selectedFile && !state.source.dirty) {
          return openFile(relPath, { keepSelection: true });
        }
        return undefined;
      });
      break;
    }
    case 'request_progress': {
      if (payload?.status === 'started') {
        state.running = {
          ...state.running,
          active: true,
          execId: payload.execId ?? state.running.execId,
          total: payload.total ?? state.running.total,
          index: (payload.index ?? 0) + 1,
          current: payload.requestName ?? null,
        };
      } else if (payload?.status === 'completed') {
        state.running = {
          ...state.running,
          index: (payload.index ?? state.running.index) + 1,
          current: null,
        };
      }
      break;
    }
    case 'execution_complete': {
      if (payload?.result && !state.lastRun) state.lastRun = payload.result;
      state.running = { ...state.running, active: false, current: null };
      break;
    }
    case 'error': {
      if (payload?.error) toast(payload.error, { severity: 'error', title: 'Execution error' });
      state.running = { ...state.running, active: false, current: null };
      break;
    }
    case 'environment_changed': {
      if (payload?.name) state.activeEnv = payload.name;
      break;
    }
    case 'stress_update': {
      applyStressUpdate(payload);
      break;
    }
    case 'mock_request': {
      if (payload?.event === 'request') {
        state.mock = {
          ...state.mock,
          requests: [
            { ...payload, at: payload.timestamp ?? new Date().toISOString() },
            ...state.mock.requests,
          ].slice(0, 200),
        };
      } else if (payload?.event === 'stopped') {
        state.mock = { ...state.mock, status: { running: false } };
      }
      break;
    }
    default:
      break;
  }
  emit();
}

function applyStressUpdate(payload) {
  if (!payload) return;
  const stats = payload.stats ?? null;
  const point = {
    at: payload.timestamp ?? new Date().toISOString(),
    elapsed: payload.elapsed ?? 0,
    rps: stats?.rps ?? 0,
    p95: stats?.p95Ms ?? 0,
    errors: stats?.errors ?? 0,
    total: stats?.total ?? 0,
  };
  state.stress = {
    ...state.stress,
    running: Boolean(payload.running),
    status: { running: Boolean(payload.running), elapsed: payload.elapsed ?? 0, stats },
    series: [...state.stress.series, point].slice(-600),
  };
  if (payload.completed) {
    api.stressResult()
      .then((result) => { state.stress = { ...state.stress, result }; emit(); })
      .catch(() => {});
    toast('Stress test finished', { severity: 'success' });
  }
}

/* -------------------------------------------------------------------------- */
/* history                                                                     */
/* -------------------------------------------------------------------------- */

export async function loadRuns({ reset = false } = {}) {
  const limit = state.history.limit;
  const offset = reset ? 0 : state.history.offset;
  state.history = { ...state.history, loading: true, offset };
  emit();
  try {
    const page = await api.runs(limit, offset);
    state.history = {
      ...state.history,
      loading: false,
      runs: offset === 0 ? (page.runs ?? []) : [...state.history.runs, ...(page.runs ?? [])],
      total: page.total ?? 0,
    };
  } catch (err) {
    state.history = { ...state.history, loading: false };
    toastError(err, { title: 'Could not load history' });
  }
  emit();
}

export async function loadMoreRuns() {
  state.history = { ...state.history, offset: state.history.offset + state.history.limit };
  await loadRuns();
}

export async function openRun(id) {
  try {
    setBusy('runDetail', true);
    const detail = await api.runDetail(id);
    state.history = { ...state.history, selected: detail };
    emit();
  } catch (err) {
    toastError(err, { title: 'Could not load run' });
  } finally {
    setBusy('runDetail', false);
  }
}

export function closeRun() {
  state.history = { ...state.history, selected: null };
  emit();
}

export async function deleteRun(id) {
  const ok = await confirmDialog({ title: 'Delete this run?', message: 'The stored run and its results are removed.', confirmLabel: 'Delete', danger: true });
  if (!ok) return;
  try {
    await api.deleteRun(id);
    state.history = { ...state.history, runs: state.history.runs.filter((r) => r.id !== id), selected: null };
    emit();
    toastSuccess('Run deleted');
  } catch (err) {
    toastError(err);
  }
}

export async function clearRuns() {
  const ok = await confirmDialog({
    title: 'Delete all run history?',
    message: 'Every stored run in the local SQLite history database will be removed.',
    confirmLabel: 'Delete all',
    danger: true,
  });
  if (!ok) return;
  try {
    await api.clearRuns();
    state.history = { ...state.history, runs: [], total: 0, selected: null };
    emit();
    toastSuccess('History cleared');
  } catch (err) {
    toastError(err);
  }
}

/* -------------------------------------------------------------------------- */
/* stress                                                                      */
/* -------------------------------------------------------------------------- */

export async function loadStress() {
  try {
    const [status, profiles] = await Promise.all([api.stressStatus(), api.stressProfiles().catch(() => [])]);
    state.stress = { ...state.stress, status, running: Boolean(status?.running), profiles: profiles ?? [] };
    if (!status?.running) {
      const result = await api.stressResult().catch(() => null);
      state.stress = { ...state.stress, result };
    }
    emit();
  } catch (err) {
    toastError(err, { title: 'Could not load stress status' });
  }
}

export function setStressForm(patch) {
  state.stress = { ...state.stress, form: { ...state.stress.form, ...patch } };
  emit();
}

export async function startStress() {
  const form = state.stress.form;
  const files = form.files?.length ? form.files : state.files.map((f) => f.relativePath);
  if (!files.length) {
    toast('No hitspec files to stress', { severity: 'warning' });
    return;
  }
  try {
    setBusy('stress', true);
    state.stress = { ...state.stress, series: [], result: null };
    await api.stressStart({
      files,
      duration: form.duration || '10s',
      rate: Number(form.rate) || 0,
      vus: Number(form.vus) || 0,
      maxVUs: Number(form.maxVUs) || 0,
      environment: state.activeEnv || undefined,
    });
    state.stress = { ...state.stress, running: true };
    emit();
    toast(`Stress test started for ${form.duration}`, { severity: 'info' });
  } catch (err) {
    toastError(err, { title: 'Could not start stress test' });
  } finally {
    setBusy('stress', false);
  }
}

export async function stopStress() {
  try {
    await api.stressStop();
    toast('Stopping stress test…', { severity: 'info', timeout: 2000 });
  } catch (err) {
    toastError(err);
  }
}

export async function saveStressProfile() {
  const name = await prompt({ title: 'Save stress profile', label: 'Profile name', placeholder: 'smoke-50rps', confirmLabel: 'Save' });
  if (!name) return;
  const form = state.stress.form;
  try {
    await api.createStressProfile({
      name,
      duration: form.duration,
      rate: Number(form.rate) || 0,
      vus: Number(form.vus) || 0,
      maxVUs: Number(form.maxVUs) || 0,
    });
    await loadStress();
    toastSuccess(`Saved profile ${name}`);
  } catch (err) {
    toastError(err, { title: 'Could not save profile' });
  }
}

export function applyStressProfile(profile) {
  setStressForm({
    duration: profile.duration || state.stress.form.duration,
    rate: profile.rate ?? 0,
    vus: profile.vus ?? 0,
    maxVUs: profile.maxVUs ?? 0,
  });
  toast(`Profile ${profile.name} applied`, { severity: 'info', timeout: 1800 });
}

export async function deleteStressProfile(name) {
  const ok = await confirmDialog({ title: `Delete profile ${name}?`, confirmLabel: 'Delete', danger: true });
  if (!ok) return;
  try {
    await api.deleteStressProfile(name);
    await loadStress();
    toastSuccess(`Deleted ${name}`);
  } catch (err) {
    toastError(err);
  }
}

/* -------------------------------------------------------------------------- */
/* mock server                                                                 */
/* -------------------------------------------------------------------------- */

export function setMockForm(patch) {
  state.mock = { ...state.mock, form: { ...state.mock.form, ...patch } };
  emit();
}

export async function loadMock() {
  try {
    state.mock = { ...state.mock, status: await api.mockRoutes() };
    emit();
  } catch (err) {
    toastError(err, { title: 'Could not load mock status' });
  }
}

export async function startMock() {
  const form = state.mock.form;
  const files = form.files?.length ? form.files : state.files.map((f) => f.relativePath);
  if (!files.length) {
    toast('No hitspec files to mock', { severity: 'warning' });
    return;
  }
  try {
    setBusy('mock', true);
    const status = await api.mockStart({ files, port: Number(form.port) || 3000, delay: form.delay || undefined });
    state.mock = { ...state.mock, status, requests: [] };
    emit();
    toastSuccess(`Mock server listening on port ${status?.port ?? form.port}`);
  } catch (err) {
    toastError(err, { title: 'Could not start mock server' });
  } finally {
    setBusy('mock', false);
  }
}

export function clearMockLog() {
  state.mock = { ...state.mock, requests: [] };
  emit();
}

export async function stopMock() {
  try {
    await api.mockStop();
    state.mock = { ...state.mock, status: { running: false } };
    emit();
    toast('Mock server stopped', { severity: 'info', timeout: 2000 });
  } catch (err) {
    toastError(err);
  }
}

/* -------------------------------------------------------------------------- */
/* recording proxy                                                            */
/* -------------------------------------------------------------------------- */

export function setRecordForm(patch) {
  state.record = { ...state.record, form: { ...state.record.form, ...patch } };
  emit();
}

export async function loadRecord() {
  try {
    state.record = { ...state.record, status: await api.recordStatus() };
    emit();
  } catch (err) {
    toastError(err, { title: 'Could not load recorder status' });
  }
}

export async function startRecord() {
  const form = state.record.form;
  if (!form.targetUrl) {
    toast('A target URL is required', { severity: 'warning' });
    return;
  }
  try {
    setBusy('record', true);
    const result = await api.recordStart({
      targetUrl: form.targetUrl,
      port: Number(form.port) || 8081,
      deduplicate: Boolean(form.deduplicate),
    });
    state.record = { ...state.record, status: { running: true, port: result?.port, targetUrl: form.targetUrl, count: 0 } };
    emit();
    toastSuccess(`Recording on port ${result?.port ?? form.port} → ${form.targetUrl}`);
  } catch (err) {
    toastError(err, { title: 'Could not start recording' });
  } finally {
    setBusy('record', false);
  }
}

export async function stopRecord() {
  try {
    await api.recordStop();
    await loadRecord();
    toast('Recording stopped', { severity: 'info', timeout: 2000 });
  } catch (err) {
    toastError(err);
  }
}

export async function exportRecordings() {
  try {
    const result = await api.recordExport();
    const content = result?.content ?? '';
    if (!content.trim()) {
      toast('No recordings to export yet', { severity: 'warning' });
      return;
    }
    const target = await bridge().saveFileDialog({
      title: 'Export recordings',
      defaultPath: 'recorded.http',
      filters: [{ name: 'hitspec file', extensions: ['http', 'hitspec'] }],
    });
    if (!target) return;
    const written = await bridge().writeFile(target, content);
    toastSuccess(`Wrote ${written.bytes} bytes to ${written.path}`);
  } catch (err) {
    toastError(err, { title: 'Export failed' });
  }
}

export async function clearRecordings() {
  try {
    await api.recordClear();
    await loadRecord();
    toast('Recordings cleared', { severity: 'info', timeout: 1800 });
  } catch (err) {
    toastError(err);
  }
}

/* -------------------------------------------------------------------------- */
/* contract                                                                   */
/* -------------------------------------------------------------------------- */

export async function loadContractFiles() {
  try {
    const result = await api.contractFiles();
    state.contract = { ...state.contract, files: result?.files ?? [] };
    emit();
  } catch (err) {
    toastError(err, { title: 'Could not list contract files' });
  }
}

export function setContractForm(patch) {
  state.contract = { ...state.contract, form: { ...state.contract.form, ...patch } };
  emit();
}

export async function verifyContracts() {
  const form = state.contract.form;
  if (!form.providerUrl) {
    toast('A provider URL is required', { severity: 'warning' });
    return;
  }
  try {
    setBusy('contract', true);
    const results = await api.contractVerify({
      files: form.files?.length ? form.files : undefined,
      providerUrl: form.providerUrl,
      stateHandler: form.stateHandler || undefined,
    });
    state.contract = { ...state.contract, results: results ?? [] };
    emit();
    const failed = (results ?? []).some((r) => r.failed > 0);
    toast(`${(results ?? []).length} file(s) verified`, { severity: failed ? 'error' : 'success' });
  } catch (err) {
    toastError(err, { title: 'Contract verification failed' });
  } finally {
    setBusy('contract', false);
  }
}

/* -------------------------------------------------------------------------- */
/* import                                                                     */
/* -------------------------------------------------------------------------- */

export function setImportDraft(patch) {
  state.importDraft = { ...state.importDraft, ...patch };
  emit();
}

export async function runImport() {
  const draft = state.importDraft;
  try {
    setBusy('import', true);
    let result;
    if (draft.kind === 'curl') {
      if (!draft.input.trim()) throw new Error('Paste a curl command first');
      result = await api.importCurl({ command: draft.input });
    } else if (draft.kind === 'insomnia') {
      if (!draft.input.trim()) throw new Error('Paste an Insomnia export first');
      result = await api.importInsomnia({ data: draft.input });
    } else {
      if (!draft.specPath.trim()) throw new Error('Provide an OpenAPI spec path or URL');
      result = await api.importOpenAPI({ specPath: draft.specPath.trim(), baseUrl: draft.baseUrl.trim() || undefined });
    }
    state.importDraft = { ...state.importDraft, result };
    emit();
    toastSuccess(`Imported ${result?.requestCount ?? 0} request(s)`);
  } catch (err) {
    toastError(err, { title: 'Import failed' });
  } finally {
    setBusy('import', false);
  }
}

export async function saveImport() {
  const content = state.importDraft?.result?.content;
  if (!content) {
    toast('Nothing to save — run an import first', { severity: 'warning' });
    return;
  }
  const target = await prompt({
    title: 'Save imported requests',
    label: 'Relative path inside the workspace',
    value: state.importDraft.target || 'imported.http',
    confirmLabel: 'Save',
  });
  if (!target) return;
  try {
    await api.createFile(target, content);
    state.importDraft = { ...state.importDraft, target, result: null, input: '' };
    await refreshFileCounts();
    await openFile(target);
    toastSuccess(`Saved ${target}`);
  } catch (err) {
    // createFile refuses to overwrite, so offer to append instead
    const overwrite = await confirmDialog({
      title: 'File already exists',
      message: `${target} exists. Append the imported requests to it instead?`,
      confirmLabel: 'Append',
    });
    if (!overwrite) {
      toastError(err);
      return;
    }
    try {
      const existing = await api.fileRaw(target);
      await api.saveFile(target, `${existing ?? ''}\n${content}`);
      await refreshFileCounts();
      await openFile(target);
      toastSuccess(`Appended to ${target}`);
    } catch (innerErr) {
      toastError(innerErr, { title: 'Could not save import' });
    }
  }
}

/* -------------------------------------------------------------------------- */
/* settings & logs                                                            */
/* -------------------------------------------------------------------------- */

export async function refreshSettings() {
  state.settings = await bridge().getSettings();
  emit();
}

export async function setBackendOption(key, value) {
  const backend = { ...(state.settings.backend ?? {}), [key]: value };
  const settings = await bridge().setSettings({ backend });
  state.settings = settings;
  emit();
  toast('Restart the API server to apply this change', {
    severity: 'info',
    timeout: 6000,
    action: { label: 'Restart now', onClick: () => restartBackend() },
  });
}

export async function loadLogs() {
  try {
    state.logs = (await bridge().getLogs(400)) ?? [];
    emit();
  } catch (err) {
    toastError(err);
  }
}

export function appendLog(line) {
  state.logs = [...state.logs, line].slice(-400);
  if (state.view === 'settings') emit();
}

export function setView(view) {
  if (state.view === view) return;
  set({ view });
  bridge().setSettings({ view }).catch(() => {});
  if (view === 'history') loadRuns({ reset: true });
  if (view === 'stress') loadStress();
  if (view === 'mock') loadMock();
  if (view === 'record') loadRecord();
  if (view === 'contract') loadContractFiles();
  if (view === 'settings') loadLogs();
}

export async function copyText(text, label = 'Copied to clipboard') {
  try {
    await bridge().copy(text);
    toast(label, { severity: 'success', timeout: 1500 });
  } catch (err) {
    toastError(err);
  }
}
