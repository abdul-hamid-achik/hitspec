#!/usr/bin/env node
/**
 * Builds the Go CLI for the desktop app: for development, and per target for
 * electron-builder (see `extraResources` in electron-builder.yml).
 *
 *   npm run build:binary                  host target only -> build/bin/<os>-<arch>/
 *   node scripts/build-binary.mjs --dist  every arch electron-builder packages on this OS
 *   node scripts/build-binary.mjs --all   every target (cross-compiles; pure Go, no CGO)
 *   node scripts/build-binary.mjs --targets mac-arm64,win-x64
 *   HITSPEC_VERSION=2.20.0 npm run build:binary
 *
 * Each binary lands in build/bin/<os>-<arch>/ so a packaged app can never
 * carry another CPU's executable. The host build is also copied to
 * build/bin/ for tools that look there.
 */
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { hostTarget, selectTargets } from './targets.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const desktopRoot = path.resolve(here, '..');
const repoRoot = path.resolve(desktopRoot, '..', '..');
const binRoot = path.join(desktopRoot, 'build', 'bin');

function version() {
  if (process.env.HITSPEC_VERSION) return process.env.HITSPEC_VERSION;
  try {
    return execFileSync('git', ['describe', '--tags', '--match', 'v[0-9]*.[0-9]*.[0-9]*', '--always', '--dirty'], {
      cwd: repoRoot,
      encoding: 'utf8',
    }).trim();
  } catch {
    return 'dev';
  }
}

const targets = selectTargets(process.argv.slice(2));
const host = hostTarget();
const ver = version();
const buildTime = new Date().toISOString();

for (const target of targets) {
  const outDir = path.join(binRoot, target.id);
  const out = path.join(outDir, target.file);
  fs.mkdirSync(outDir, { recursive: true });

  console.log(`building ${target.file} ${ver} for ${target.id} (${target.goos}/${target.goarch}) -> ${path.relative(repoRoot, out)}`);
  execFileSync(
    'go',
    ['build', '-trimpath', '-ldflags', `-s -w -X main.version=${ver} -X main.buildTime=${buildTime}`, '-o', out, './apps/cli'],
    {
      cwd: repoRoot,
      stdio: 'inherit',
      env: { ...process.env, GOOS: target.goos, GOARCH: target.goarch, CGO_ENABLED: '0' },
    },
  );

  if (host && target.id === host.id) {
    fs.copyFileSync(out, path.join(binRoot, target.file));
  }
}
console.log('done');
