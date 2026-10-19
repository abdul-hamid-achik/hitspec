#!/usr/bin/env node
/**
 * Asserts each unpacked app carries a hitspec CLI built for that app's own OS
 * and CPU (the x64 dmg must not ship the arm64 binary, Windows needs .exe).
 *
 *   npm run dist:dir && npm run test:bundled
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { detectBinary } from './inspect.mjs';
import { packagedApps } from './packaged-layout.mjs';

const desktopRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const releaseDir = path.resolve(desktopRoot, process.argv[2] ?? 'release');
const apps = packagedApps(releaseDir);

if (!apps.length) {
  console.error(`no unpacked apps under ${releaseDir}; run "npm run dist:dir" first`);
  process.exit(1);
}

const FORMAT = { darwin: 'macho', win32: 'pe', linux: 'elf' };
let failed = false;
for (const app of apps) {
  if (!fs.existsSync(app.cli)) {
    failed = true;
    console.error(`x ${app.dir}: missing bundled CLI ${app.cli}`);
    continue;
  }
  const info = detectBinary(fs.readFileSync(app.cli));
  if (!info || info.format !== FORMAT[app.platform] || info.arch !== app.arch) {
    failed = true;
    console.error(`x ${app.dir}: bundled CLI is ${info ? `${info.format}/${info.arch}` : 'not a recognised executable'}, want ${FORMAT[app.platform]}/${app.arch}`);
  } else {
    console.log(`ok ${app.dir}: bundled CLI is ${info.format}/${info.arch}`);
  }
}
process.exit(failed ? 1 : 0);
