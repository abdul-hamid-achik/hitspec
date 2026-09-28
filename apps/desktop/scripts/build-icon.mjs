#!/usr/bin/env node
/**
 * Rasterises build/icon.svg into build/icon.png (1024x1024, transparent
 * corners) with macOS Quick Look — no image dependencies and no Electron
 * offscreen window (which hangs on macOS). electron-builder derives the
 * .icns/.ico from this PNG at package time.
 *
 *   npm run build:icon        (macOS only; the PNG is committed for other OSes)
 */
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

if (process.platform !== 'darwin') {
  console.log('icon rendering uses macOS Quick Look; skipping on', process.platform);
  process.exit(0);
}

const here = path.dirname(fileURLToPath(import.meta.url));
const buildDir = path.join(path.resolve(here, '..'), 'build');
const svgPath = path.join(buildDir, 'icon.svg');
const pngPath = path.join(buildDir, 'icon.png');

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-icon-'));
try {
  // qlmanage renders through a per-user QuickLook agent and occasionally
  // queues behind it; retry a few times before giving up.
  const rendered = path.join(tmp, 'icon.svg.png');
  let lastError = null;
  for (let attempt = 1; attempt <= 3 && !fs.existsSync(rendered); attempt++) {
    try {
      // NB: qlmanage blocks (rather than erroring) when the input path does
      // not exist, so a wrong svgPath looks like a hang; the timeout bounds it.
      execFileSync('qlmanage', ['-t', '-s', '1024', '-o', tmp, svgPath], {
        stdio: 'ignore',
        timeout: 20_000,
      });
    } catch (err) {
      lastError = err;
    }
  }
  if (!fs.existsSync(rendered)) {
    throw lastError ?? new Error(`qlmanage produced no output in ${tmp}`);
  }

  const props = execFileSync('sips', ['-g', 'hasAlpha', '-g', 'pixelWidth', rendered], { encoding: 'utf8' });
  if (!/hasAlpha: yes/.test(props)) throw new Error('rendered icon lost its alpha channel');
  if (!/pixelWidth: 1024/.test(props)) throw new Error('rendered icon is not 1024px wide');

  fs.copyFileSync(rendered, pngPath);
  console.log(`wrote ${path.relative(process.cwd(), pngPath)} (1024x1024, transparent corners)`);
} finally {
  fs.rmSync(tmp, { recursive: true, force: true });
}
