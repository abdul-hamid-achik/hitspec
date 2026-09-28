import fs from 'node:fs';
import path from 'node:path';

const MAX_RECENTS = 12;

export const DEFAULTS = Object.freeze({
  theme: 'nord',
  lastWorkspace: null,
  recents: [],
  view: 'workspace',
  window: { width: 1360, height: 860, x: null, y: null, maximized: false },
  backend: { watch: true, readOnly: false, allowShell: false, allowDB: false, logLevel: 'info' },
});

function merge(base, incoming) {
  if (!incoming || typeof incoming !== 'object') return { ...base };
  const out = { ...base };
  for (const [key, value] of Object.entries(incoming)) {
    if (value === undefined) continue;
    if (value && typeof value === 'object' && !Array.isArray(value) && base[key] && typeof base[key] === 'object') {
      out[key] = { ...base[key], ...value };
    } else {
      out[key] = value;
    }
  }
  return out;
}

/**
 * Small JSON settings store in Electron's userData directory. Writes are atomic
 * (tmp + rename) so a crash mid-write cannot corrupt the file.
 */
export class Settings {
  constructor(filePath) {
    this.filePath = filePath;
    this.data = { ...DEFAULTS };
  }

  load() {
    try {
      const text = fs.readFileSync(this.filePath, 'utf8');
      this.data = merge(DEFAULTS, JSON.parse(text));
    } catch {
      this.data = merge(DEFAULTS, {});
    }
    return this.data;
  }

  save() {
    try {
      fs.mkdirSync(path.dirname(this.filePath), { recursive: true });
      const tmp = `${this.filePath}.${process.pid}.tmp`;
      fs.writeFileSync(tmp, `${JSON.stringify(this.data, null, 2)}\n`, 'utf8');
      fs.renameSync(tmp, this.filePath);
      return true;
    } catch (err) {
      console.error('[hitspec] failed to save settings:', err?.message ?? err);
      return false;
    }
  }

  all() {
    return this.data;
  }

  get(key) {
    return this.data[key];
  }

  set(key, value) {
    this.data = key && typeof key === 'object' ? merge(this.data, key) : merge(this.data, { [key]: value });
    this.save();
    return this.data;
  }

  /** Record a workspace in the recents list (most recent first, deduped). */
  addRecent(dirPath) {
    if (!dirPath) return this.data.recents;
    const normalized = path.resolve(dirPath);
    const recents = (this.data.recents ?? []).filter((r) => r?.path !== normalized);
    recents.unshift({ path: normalized, name: path.basename(normalized), openedAt: new Date().toISOString() });
    this.data.recents = recents.slice(0, MAX_RECENTS);
    this.data.lastWorkspace = normalized;
    this.save();
    return this.data.recents;
  }

  removeRecent(dirPath) {
    const normalized = path.resolve(dirPath);
    this.data.recents = (this.data.recents ?? []).filter((r) => r?.path !== normalized);
    if (this.data.lastWorkspace === normalized) {
      this.data.lastWorkspace = this.data.recents[0]?.path ?? null;
    }
    this.save();
    return this.data.recents;
  }
}
