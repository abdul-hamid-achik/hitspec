/**
 * Typed-ish wrappers around the `hitspec serve --api-only` REST surface.
 * All calls go through the main-process bridge, which owns the auth token.
 */

export class ApiError extends Error {
  constructor(result) {
    super(result?.error || `HTTP ${result?.status ?? 0}`);
    this.name = 'ApiError';
    this.status = result?.status ?? 0;
    this.payload = result?.data ?? null;
  }
}

function bridge() {
  if (!window.hitspec) throw new Error('hitspec bridge unavailable');
  return window.hitspec;
}

/** Percent-encode each path segment so spaces and unicode survive. */
export function encodePath(relPath) {
  return String(relPath ?? '')
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

async function call(method, path, options = {}) {
  const result = await bridge().api({ method, path, ...options });
  if (!result.ok) throw new ApiError(result);
  return result.data;
}

export const api = {
  // workspace & files
  workspace: () => call('GET', '/workspace'),
  files: () => call('GET', '/files'),
  file: (relPath) => call('GET', `/files/${encodePath(relPath)}`),
  fileRaw: (relPath) => call('GET', `/files/raw/${encodePath(relPath)}`, { raw: true }),
  saveFile: (relPath, content) =>
    call('PUT', `/files/${encodePath(relPath)}`, { body: content, raw: true }),
  createFile: (relPath, content) => call('POST', '/files', { body: { path: relPath, content } }),
  deleteFile: (relPath) => call('DELETE', `/files/${encodePath(relPath)}`),

  // execution
  execute: (file, requestName, environment) =>
    call('POST', '/execute', { body: { file, requestName, environment } }),
  runFile: (file, environment) => call('POST', '/run', { body: { file, environment } }),

  // environments & config
  environments: () => call('GET', '/environments'),
  environment: (name) => call('GET', `/environments/${encodeURIComponent(name)}`),
  setActiveEnvironment: (name) => call('PUT', '/environments/active', { body: { name } }),
  putEnvironment: (name, variables) =>
    call('PUT', `/environments/${encodeURIComponent(name)}`, { body: { name, variables } }),
  config: () => call('GET', '/config'),
  putConfig: (config) => call('PUT', '/config', { body: config }),

  // persistent history
  runs: (limit = 30, offset = 0) => call('GET', '/history/runs', { query: { limit, offset } }),
  runDetail: (id) => call('GET', `/history/runs/${encodeURIComponent(id)}`),
  deleteRun: (id) => call('DELETE', `/history/runs/${encodeURIComponent(id)}`),
  clearRuns: () => call('DELETE', '/history/runs'),
  resultsByRequest: (requestName, filePath, limit = 20) =>
    call('GET', '/history/results', { query: { requestName, filePath, limit } }),
  legacyHistory: () => call('GET', '/history'),

  // stress
  stressStart: (payload) => call('POST', '/stress/start', { body: payload }),
  stressStop: () => call('POST', '/stress/stop'),
  stressStatus: () => call('GET', '/stress/status'),
  stressResult: () => call('GET', '/stress/result'),
  stressProfiles: () => call('GET', '/stress/profiles'),
  createStressProfile: (payload) => call('POST', '/stress/profiles', { body: payload }),
  updateStressProfile: (name, payload) =>
    call('PUT', `/stress/profiles/${encodeURIComponent(name)}`, { body: payload }),
  deleteStressProfile: (name) => call('DELETE', `/stress/profiles/${encodeURIComponent(name)}`),

  // mock
  mockStart: (payload) => call('POST', '/mock/start', { body: payload }),
  mockStop: () => call('POST', '/mock/stop'),
  mockRoutes: () => call('GET', '/mock/routes'),

  // recording proxy
  recordStart: (payload) => call('POST', '/record/start', { body: payload }),
  recordStop: () => call('POST', '/record/stop'),
  recordStatus: () => call('GET', '/record/status'),
  recordExport: () => call('POST', '/record/export'),
  recordClear: () => call('DELETE', '/record/clear'),

  // contract
  contractVerify: (payload) => call('POST', '/contract/verify', { body: payload }),
  contractFiles: () => call('GET', '/contract/files'),

  // import / export
  importCurl: (payload) => call('POST', '/import/curl', { body: payload }),
  importInsomnia: (payload) => call('POST', '/import/insomnia', { body: payload }),
  importOpenAPI: (payload) => call('POST', '/import/openapi', { body: payload }),
  exportCurl: (file, requestName) => call('POST', '/export/curl', { body: { file, requestName } }),

  // system
  systemInfo: () => call('GET', '/system/info'),
};
