#!/usr/bin/env node
/**
 * Parse every JavaScript module in the app without evaluating it.
 *
 * There is no bundler in this project, so a typo would otherwise only surface
 * when Electron loads the file. `vm.SourceTextModule` performs the same ESM
 * parse the renderer/main process will do, and reports syntax errors up front.
 */
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const appRoot = path.resolve(here, '..');

const SKIP_DIRS = new Set(['node_modules', 'release', 'build', '.git', 'dist']);

function* walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!SKIP_DIRS.has(entry.name)) yield* walk(path.join(dir, entry.name));
      continue;
    }
    if (/\.(js|mjs|cjs)$/.test(entry.name)) yield path.join(dir, entry.name);
  }
}

let failures = 0;
let checked = 0;

for (const file of walk(appRoot)) {
  const source = fs.readFileSync(file, 'utf8');
  checked++;
  try {
    if (file.endsWith('.cjs')) {
      new vm.Script(source, { filename: file });
    } else {
      // SourceTextModule parses (and links) without running module top-level code.
      new vm.SourceTextModule(source, { identifier: file });
    }
  } catch (err) {
    failures++;
    console.error(`✗ ${path.relative(appRoot, file)}: ${err.message}`);
  }
}

if (failures) {
  console.error(`\n${failures} of ${checked} file(s) failed to parse`);
  process.exit(1);
}
console.log(`✓ ${checked} file(s) parsed cleanly`);
