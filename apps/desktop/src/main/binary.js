import { execFile } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Binary resolution is pure (no Electron imports) so it can be unit tested and
 * reused by the smoke script.
 */

export function binaryName(platform) {
  return platform === 'win32' ? 'hitspec.exe' : 'hitspec';
}

/**
 * Ordered candidate locations for the hitspec CLI.
 *
 * @param {object} opts
 * @param {string} opts.platform
 * @param {boolean} opts.isPackaged
 * @param {string} [opts.resourcesPath] process.resourcesPath when packaged
 * @param {string} opts.appRoot absolute path to apps/desktop
 * @param {Record<string,string|undefined>} opts.env
 * @returns {{path:string, source:string}[]}
 */
export function candidatePaths({ platform, isPackaged, resourcesPath, appRoot, env }) {
  const name = binaryName(platform);
  const repoRoot = path.resolve(appRoot, '..', '..');
  const candidates = [];

  if (env.HITSPEC_BIN) {
    candidates.push({ path: env.HITSPEC_BIN, source: 'HITSPEC_BIN' });
  }
  if (isPackaged && resourcesPath) {
    candidates.push({ path: path.join(resourcesPath, name), source: 'bundled' });
  }
  // Source checkout: `task build` / `npm run build:binary` outputs.
  candidates.push({ path: path.join(appRoot, 'build', 'bin', name), source: 'desktop-build' });
  candidates.push({ path: path.join(repoRoot, 'bin', name), source: 'repo-bin' });

  return candidates;
}

async function defaultExists(p) {
  try {
    const st = await fs.promises.stat(p);
    return st.isFile();
  } catch {
    return false;
  }
}

function defaultWhich(name) {
  const finder = process.platform === 'win32' ? 'where' : 'which';
  return new Promise((resolve) => {
    execFile(finder, [name], { windowsHide: true }, (err, stdout) => {
      if (err) return resolve(null);
      const first = String(stdout).split(/\r?\n/).map((l) => l.trim()).find(Boolean);
      resolve(first || null);
    });
  });
}

/**
 * Find the hitspec CLI to spawn.
 *
 * @param {object} opts
 * @param {string} opts.platform
 * @param {boolean} opts.isPackaged
 * @param {string} [opts.resourcesPath]
 * @param {string} opts.appRoot
 * @param {Record<string,string|undefined>} opts.env
 * @param {(p:string)=>Promise<boolean>} [opts.exists]
 * @param {(name:string)=>Promise<string|null>} [opts.which]
 * @returns {Promise<{path:string, source:string}|{error:string, tried:{path:string,source:string}[]}>}
 */
export async function resolveBinary(opts) {
  const exists = opts.exists ?? defaultExists;
  const which = opts.which ?? defaultWhich;
  const tried = candidatePaths(opts);

  for (const candidate of tried) {
    if (await exists(candidate.path)) {
      return { path: candidate.path, source: candidate.source };
    }
  }

  const name = binaryName(opts.platform);
  const onPath = await which(name);
  if (onPath) {
    return { path: onPath, source: 'PATH' };
  }

  return {
    error:
      `Could not find the hitspec CLI. Looked in:\n` +
      tried.map((c) => `  - ${c.path} (${c.source})`).join('\n') +
      `\n  - ${name} on PATH\n\n` +
      `Build it from the repository root with "task build" (or "npm run build:binary" inside apps/desktop), ` +
      `install it with "brew install hitspec", or point HITSPEC_BIN at an existing binary.`,
    tried,
  };
}
