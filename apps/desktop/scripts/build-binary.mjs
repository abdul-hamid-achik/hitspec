#!/usr/bin/env node
/**
 * Builds the Go CLI next to the desktop app so it can be spawned in
 * development and bundled by electron-builder (see extraResources).
 *
 *   npm run build:binary
 *   HITSPEC_VERSION=2.19.0 npm run build:binary
 */
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const desktopRoot = path.resolve(here, '..');
const repoRoot = path.resolve(desktopRoot, '..', '..');

const name = process.platform === 'win32' ? 'hitspec.exe' : 'hitspec';
const outDir = path.join(desktopRoot, 'build', 'bin');
const out = path.join(outDir, name);

function version() {
  if (process.env.HITSPEC_VERSION) return process.env.HITSPEC_VERSION;
  try {
    return execFileSync('git', ['describe', '--tags', '--always', '--dirty'], {
      cwd: repoRoot,
      encoding: 'utf8',
    }).trim();
  } catch {
    return 'dev';
  }
}

fs.mkdirSync(outDir, { recursive: true });

console.log(`building ${name} ${version()} → ${path.relative(repoRoot, out)}`);
execFileSync(
  'go',
  ['build', '-ldflags', `-X main.version=${version()} -X main.buildTime=${new Date().toISOString()}`, '-o', out, './apps/cli'],
  { cwd: repoRoot, stdio: 'inherit' },
);
console.log('done');
