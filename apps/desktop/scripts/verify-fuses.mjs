#!/usr/bin/env node
/**
 * Asserts the unpacked app(s) from `npm run dist:dir` have the hardened
 * Electron fuses (see `electronFuses` in electron-builder.yml).
 *
 *   npm run dist:dir && npm run test:fuses
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { checkFuses, readFuseWire } from './inspect.mjs';
import { packagedApps } from './packaged-layout.mjs';

const desktopRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const releaseDir = path.resolve(desktopRoot, process.argv[2] ?? 'release');
const apps = packagedApps(releaseDir);

if (!apps.length) {
  console.error(`no unpacked apps under ${releaseDir}; run "npm run dist:dir" first`);
  process.exit(1);
}

let failed = false;
for (const app of apps) {
  const problems = checkFuses(readFuseWire(fs.readFileSync(app.fuseHost)));
  if (problems.length) {
    failed = true;
    console.error(`x ${app.dir}: ${app.fuseHost}\n    ${problems.join('\n    ')}`);
  } else {
    console.log(`ok ${app.dir}: fuses hardened`);
  }
}
process.exit(failed ? 1 : 0);
