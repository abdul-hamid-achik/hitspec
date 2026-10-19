/**
 * Security policy for the renderer -> main boundary. Pure (no Electron
 * imports) so every rule is unit tested.
 *
 * The renderer is untrusted code from the point of view of the main process:
 * a single XSS in rendered response data would otherwise let it drive the
 * authenticated REST API, write arbitrary settings, or spawn the CLI with an
 * attacker-chosen environment.
 */

export const APP_SCHEME = 'hitspec';
export const APP_HOST = 'app';

/**
 * Is this IPC message from the app's own top-level renderer page?
 * `senderFrame` can be null/stale after navigation; treat that as untrusted.
 */
export function isTrustedSender(event, { isKnownSender = () => true } = {}) {
  const frame = event?.senderFrame;
  if (!frame || typeof frame.url !== 'string') return false;
  let url;
  try {
    url = new URL(frame.url);
  } catch {
    return false;
  }
  if (url.protocol !== `${APP_SCHEME}:` || url.host !== APP_HOST) return false;
  // Sub-frames (there are none today) must not inherit the main page's powers.
  if (frame.parent) return false;
  if (event.sender?.mainFrame && event.sender.mainFrame !== frame) return false;
  return isKnownSender(event.sender) === true;
}

/**
 * `ipcMain.handle` wrapper that refuses messages from untrusted senders
 * before the handler runs.
 */
export function createSecureHandle(ipcMain, isTrusted) {
  return (channel, handler) => {
    ipcMain.handle(channel, (event, ...args) => {
      if (!isTrusted(event)) {
        throw new Error(`blocked IPC call on "${channel}" from an untrusted sender`);
      }
      return handler(event, ...args);
    });
  };
}

/* -------------------------------------------------------------------------- */
/* REST proxy allowlist                                                        */
/* -------------------------------------------------------------------------- */

const SEG = '[^/]+';
// [method, path regex]. Mirrors src/renderer/src/api.js; a new renderer call
// needs a line here on purpose.
const API_ROUTES = [
  ['GET', /^\/workspace$/],
  ['GET', /^\/files$/],
  ['POST', /^\/files$/],
  ['GET', /^\/files\/.+$/],
  ['PUT', /^\/files\/.+$/],
  ['DELETE', /^\/files\/.+$/],
  ['POST', /^\/execute$/],
  ['POST', /^\/run$/],
  ['GET', /^\/environments$/],
  ['GET', new RegExp(`^/environments/${SEG}$`)],
  ['PUT', /^\/environments\/active$/],
  ['PUT', new RegExp(`^/environments/${SEG}$`)],
  ['GET', /^\/config$/],
  ['PUT', /^\/config$/],
  ['GET', /^\/history$/],
  ['GET', /^\/history\/runs$/],
  ['DELETE', /^\/history\/runs$/],
  ['GET', new RegExp(`^/history/runs/${SEG}$`)],
  ['DELETE', new RegExp(`^/history/runs/${SEG}$`)],
  ['GET', /^\/history\/results$/],
  ['POST', /^\/stress\/(start|stop)$/],
  ['GET', /^\/stress\/(status|result|profiles)$/],
  ['POST', /^\/stress\/profiles$/],
  ['PUT', new RegExp(`^/stress/profiles/${SEG}$`)],
  ['DELETE', new RegExp(`^/stress/profiles/${SEG}$`)],
  ['POST', /^\/mock\/(start|stop)$/],
  ['GET', /^\/mock\/routes$/],
  ['POST', /^\/record\/(start|stop|export)$/],
  ['GET', /^\/record\/status$/],
  ['DELETE', /^\/record\/clear$/],
  ['POST', /^\/contract\/verify$/],
  ['GET', /^\/contract\/files$/],
  ['POST', /^\/import\/(curl|insomnia|openapi)$/],
  ['POST', /^\/export\/curl$/],
  ['GET', /^\/system\/info$/],
];

const MAX_TIMEOUT_MS = 10 * 60_000;
const ACCEPT_RE = /^[\w.+-]+\/[\w.+-]+(?:\s*;\s*charset=[\w-]+)?$/i;

/** Segments must decode cleanly and never be "." / ".." / contain a separator. */
function hasSafeSegments(pathname) {
  for (const raw of pathname.split('/')) {
    if (raw === '') continue;
    let seg;
    try {
      seg = decodeURIComponent(raw);
    } catch {
      return false;
    }
    if (seg === '.' || seg === '..' || /[/\\\0]/.test(seg)) return false;
  }
  return !pathname.includes('//');
}

/**
 * Validate and normalise a renderer-supplied REST request. Only the known
 * fields survive, so extra properties never reach `Backend.request`.
 * @returns {{ok:true, request:object}|{ok:false, error:string}}
 */
export function validateApiRequest(req) {
  const fail = (error) => ({ ok: false, error });
  if (!req || typeof req !== 'object' || Array.isArray(req)) return fail('invalid api request');
  const method = String(req.method ?? 'GET').toUpperCase();
  const pathname = req.path;
  if (typeof pathname !== 'string' || !pathname.startsWith('/') || pathname.length > 4096) {
    return fail('invalid api path');
  }
  if (/[?#\s\0\\]/.test(pathname) || !hasSafeSegments(pathname)) return fail('invalid api path');
  if (!API_ROUTES.some(([m, re]) => m === method && re.test(pathname))) {
    return fail(`api request not allowed: ${method} ${pathname}`);
  }

  const out = { method, path: pathname };

  if (req.query !== undefined && req.query !== null) {
    if (typeof req.query !== 'object' || Array.isArray(req.query)) return fail('invalid api query');
    const query = {};
    for (const [k, v] of Object.entries(req.query)) {
      if (v === undefined || v === null) continue;
      if (!['string', 'number', 'boolean'].includes(typeof v)) return fail('invalid api query value');
      query[k] = v;
    }
    out.query = query;
  }
  if (req.body !== undefined && req.body !== null) {
    if (typeof req.body !== 'string' && (typeof req.body !== 'object')) return fail('invalid api body');
    out.body = req.body;
  }
  if (req.raw !== undefined) out.raw = Boolean(req.raw);
  if (req.accept !== undefined && req.accept !== null) {
    if (typeof req.accept !== 'string' || !ACCEPT_RE.test(req.accept)) return fail('invalid content type');
    out.accept = req.accept;
  }
  if (req.timeout !== undefined && req.timeout !== null) {
    if (!Number.isFinite(req.timeout) || req.timeout <= 0) return fail('invalid timeout');
    out.timeout = Math.min(req.timeout, MAX_TIMEOUT_MS);
  }
  return { ok: true, request: out };
}

/* -------------------------------------------------------------------------- */
/* settings & child process environment                                        */
/* -------------------------------------------------------------------------- */

export const LOG_LEVELS = ['debug', 'info', 'warn', 'error'];
const THEME_RE = /^[a-z0-9-]{1,32}$/;
const VIEW_RE = /^[a-z]{1,24}$/;

/**
 * Reduce a renderer-supplied settings patch to the keys and shapes the UI
 * legitimately writes. Notably absent: `backend.childEnv` (arbitrary child
 * environment, e.g. DYLD_INSERT_LIBRARIES), `backend.allowShell` (needs a
 * main-process confirmation, see ipc.js) and `lastWorkspace`.
 */
export function sanitizeSettingsPatch(patch) {
  const out = {};
  if (!patch || typeof patch !== 'object' || Array.isArray(patch)) return out;

  if (typeof patch.theme === 'string' && THEME_RE.test(patch.theme)) out.theme = patch.theme;
  if (typeof patch.view === 'string' && VIEW_RE.test(patch.view)) out.view = patch.view;

  const w = patch.window;
  if (w && typeof w === 'object') {
    const win = {};
    for (const k of ['width', 'height', 'x', 'y']) {
      if (Number.isFinite(w[k])) win[k] = w[k];
    }
    if (typeof w.maximized === 'boolean') win.maximized = w.maximized;
    out.window = win;
  }

  const b = patch.backend;
  if (b && typeof b === 'object') {
    const backend = {};
    if (typeof b.watch === 'boolean') backend.watch = b.watch;
    if (typeof b.readOnly === 'boolean') backend.readOnly = b.readOnly;
    if (LOG_LEVELS.includes(b.logLevel)) backend.logLevel = b.logLevel;
    out.backend = backend;
  }
  return out;
}

/**
 * Child-process environment keys that change how a program is loaded or
 * where it looks for code. Never forwarded from option bags.
 */
const DANGEROUS_ENV = /^(DYLD_|LD_|NODE_OPTIONS$|NODE_PATH$|ELECTRON_|HITSPEC_API_TOKEN$|PATH$|PATHEXT$|COMSPEC$|SHELL$)/i;

/** Drop environment overrides that could hijack the child (defense in depth). */
export function sanitizeChildEnv(env) {
  const out = {};
  if (!env || typeof env !== 'object') return out;
  for (const [k, v] of Object.entries(env)) {
    if (typeof v !== 'string' || !/^[A-Za-z_][A-Za-z0-9_]*$/.test(k) || DANGEROUS_ENV.test(k)) continue;
    out[k] = v;
  }
  return out;
}

/**
 * Backend start options derived from stored settings plus main-process state.
 * `childEnv` is never read from settings, and `allowShell` comes only from an
 * explicit per-workspace confirmation.
 */
export function buildStartOptions(backendSettings, { allowShell = false } = {}) {
  const s = backendSettings && typeof backendSettings === 'object' ? backendSettings : {};
  const opts = {
    watch: s.watch !== false,
    readOnly: s.readOnly === true,
    logLevel: LOG_LEVELS.includes(s.logLevel) ? s.logLevel : 'info',
    allowShell: allowShell === true,
  };
  if (typeof s.env === 'string' && /^[\w.-]{1,64}$/.test(s.env)) opts.env = s.env;
  return opts;
}

/**
 * Workspaces for which the user confirmed `--allow-shell` in this session.
 * Deliberately in-memory only: a restart of the app asks again.
 */
export class ShellTrust {
  constructor() {
    this.approved = new Set();
  }

  has(dir) {
    return Boolean(dir) && this.approved.has(dir);
  }

  grant(dir) {
    if (dir) this.approved.add(dir);
  }

  revoke(dir) {
    this.approved.delete(dir);
  }
}

/**
 * Enable/disable `--allow-shell` for a workspace. Enabling always goes through
 * `confirm` (a native dialog owned by the main process) unless this workspace
 * was already approved in this session; the renderer only ever *asks*.
 * @returns {Promise<{allowed:boolean}>}
 */
export async function requestAllowShell({ enabled, dir, trust, confirm }) {
  if (!dir) throw new Error('open a workspace before changing shell execution');
  if (!enabled) {
    trust.revoke(dir);
    return { allowed: false };
  }
  if (trust.has(dir)) return { allowed: true };
  const approved = (await confirm(dir)) === true;
  if (approved) trust.grant(dir);
  return { allowed: approved };
}

/**
 * Tracks the renderer's unsaved-edit state and asks before throwing it away.
 * `confirmDiscard` is injected (a native dialog in production).
 */
export class DirtyGuard {
  constructor({ confirmDiscard }) {
    this.confirmDiscard = confirmDiscard;
    this.dirty = new Map(); // webContents id -> boolean
  }

  set(id, value) {
    if (value) this.dirty.set(id, true);
    else this.dirty.delete(id);
  }

  clear(id) {
    this.dirty.delete(id);
  }

  isDirty(id) {
    return id === undefined ? this.dirty.size > 0 : this.dirty.has(id);
  }

  /** @returns {boolean} true when it is fine to proceed (clean, or the user agreed to discard) */
  confirm(id, context) {
    if (!this.isDirty(id)) return true;
    const ok = this.confirmDiscard(context) === true;
    if (ok) {
      if (id === undefined) this.dirty.clear();
      else this.dirty.delete(id);
    }
    return ok;
  }
}
