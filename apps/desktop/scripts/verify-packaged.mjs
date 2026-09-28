#!/usr/bin/env node
/**
 * Boots the *packaged* app (output of `npm run dist:dir`) with Playwright and
 * asserts it resolves the bundled Go binary from process.resourcesPath.
 *
 *   npm run dist:dir && npm run test:packaged
 *
 * Skips cleanly when no unpacked build exists for this platform.
 */
import { _electron as electron } from 'playwright';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { createWorkspace, startFixtureApi } from '../test/helpers/harness.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const desktopRoot = path.resolve(here, '..');

const candidates = {
  darwin: [
    'release/mac-arm64/hitspec Studio.app/Contents/MacOS/hitspec Studio',
    'release/mac/hitspec Studio.app/Contents/MacOS/hitspec Studio',
  ],
  linux: ['release/linux-unpacked/hitspec-studio'],
  win32: ['release/win-unpacked/hitspec Studio.exe'],
};

const exe = (candidates[process.platform] ?? [])
  .map((rel) => path.join(desktopRoot, rel))
  .find((abs) => fs.existsSync(abs));

if (!exe) {
  console.log('no unpacked build found for this platform — run `npm run dist:dir` first. skipping.');
  process.exit(0);
}

const api = await startFixtureApi();
const workspace = createWorkspace({ port: api.port, prefix: 'hitspec-packaged-' });
const home = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-packaged-home-'));

let exitCode = 0;
let app;
try {
  console.log(`launching packaged app: ${exe}`);
  app = await electron.launch({
    executablePath: exe,
    args: [workspace.dir],
    env: {
      ...process.env,
      HOME: home,
      USERPROFILE: home,
      HITSPEC_DESKTOP_USER_DATA: path.join(home, 'userdata'),
    },
  });

  const page = await app.firstWindow();
  const pageErrors = [];
  page.on('pageerror', (err) => pageErrors.push(String(err?.stack ?? err)));

  await page.waitForSelector('.workspace-chip .name', { timeout: 60_000 });
  console.log('workspace loaded:', await page.locator('.workspace-chip .name').textContent());

  // Settings shows where the CLI was resolved from; packaged builds must use
  // the bundled binary, not whatever happens to be on PATH.
  await page.locator('.rail-item', { hasText: 'Settings' }).click();
  await page.waitForSelector('.card-head h3:has-text("API server")', { timeout: 30_000 });
  const serverCard = page.locator('.card', { has: page.locator('.card-head h3', { hasText: 'API server' }) });
  await serverCard.locator('.kv .v', { hasText: 'bundled' }).waitFor({ timeout: 30_000 });
  console.log('binary resolved from: bundled');

  await page.locator('.rail-item', { hasText: 'Requests' }).click();
  await page.locator('.tree-row.dir', { hasText: 'api' }).click();
  await page.locator('.tree-row', { hasText: 'health.http' }).click();
  await page.locator('.view-head .btn', { hasText: 'Run' }).first().click();
  await page
    .locator('.stat', { hasText: 'Status' })
    .locator('.value')
    .waitFor({ timeout: 60_000 });
  const status = await page.locator('.stat', { hasText: 'Status' }).locator('.value').textContent();
  assert.equal(status.trim(), '200', 'expected the packaged app to execute a request');
  console.log('packaged app executed a request: HTTP', status.trim());

  assert.deepEqual(pageErrors, [], `renderer errors: ${pageErrors.join('\n')}`);
  console.log('✓ packaged app verified');
} catch (err) {
  console.error('✗ packaged verification failed:', err?.message ?? err);
  exitCode = 1;
} finally {
  await app?.close().catch(() => {});
  await api.close();
  workspace.cleanup();
  fs.rmSync(home, { recursive: true, force: true });
}
process.exit(exitCode);
