#!/usr/bin/env node
/**
 * Captures a screenshot of every view into .playwright/screenshots/ so the UI
 * can be reviewed visually without clicking through the app by hand.
 *
 *   npm run screenshots
 */
import { _electron as electron } from 'playwright';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { createWorkspace, findHitspecBinary, startFixtureApi } from '../test/helpers/harness.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const desktopRoot = path.resolve(here, '..');
const shotDir = path.join(desktopRoot, '.playwright', 'screenshots');
fs.mkdirSync(shotDir, { recursive: true });

const binary = await findHitspecBinary();
const api = await startFixtureApi();
const workspace = createWorkspace({ port: api.port, prefix: 'hitspec-shots-' });

const launch = (args, userData) =>
  electron.launch({
    args,
    cwd: desktopRoot,
    env: {
      ...process.env,
      HOME: userData,
      USERPROFILE: userData,
      HITSPEC_BIN: binary.path,
      HITSPEC_DESKTOP_USER_DATA: path.join(userData, 'userdata'),
    },
  });

const shot = async (page, name) => {
  const file = path.join(shotDir, `${name}.png`);
  await page.screenshot({ path: file });
  console.log(`  ${path.relative(desktopRoot, file)}`);
};

let app;
let home;
try {
  /* ---- welcome screen (no workspace) ---- */
  home = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-shots-home-'));
  app = await launch([desktopRoot], home);
  let page = await app.firstWindow();
  await page.waitForSelector('.welcome-card', { timeout: 60_000 });
  await shot(page, 'view-00-welcome');
  await app.close();

  /* ---- every view with a live workspace ---- */
  app = await launch([desktopRoot, workspace.dir], home);
  page = await app.firstWindow();
  await page.waitForSelector('.workspace-chip .name', { timeout: 60_000 });
  await page.locator('.tree-row.dir', { hasText: 'api' }).click();
  await page.locator('.tree-row', { hasText: 'health.http' }).click();
  await page.locator('.req-row').click();
  await page.waitForTimeout(250);
  await shot(page, 'view-01-request');

  await page.locator('.view-head .btn', { hasText: 'Run' }).first().click();
  await page
    .locator('.stat', { hasText: 'Status' })
    .locator('.value')
    .waitFor({ timeout: 60_000 });
  await page.waitForTimeout(250);
  await shot(page, 'view-02-response');

  await page.locator('.tab', { hasText: 'Source' }).click();
  await page.waitForTimeout(250);
  await shot(page, 'view-03-source');

  await page.locator('.tree-row', { hasText: 'users.http' }).click();
  await page.locator('.view-head .btn', { hasText: 'Run file' }).click();
  await page.locator('.pill', { hasText: '5 passed' }).waitFor({ timeout: 60_000 });
  await shot(page, 'view-04-run-file');

  const views = [
    ['History', 'view-05-history'],
    ['Stress', 'view-06-stress'],
    ['Mock', 'view-07-mock'],
    ['Record', 'view-08-record'],
    ['Import', 'view-09-import'],
    ['Contracts', 'view-10-contracts'],
    ['Settings', 'view-11-settings'],
  ];
  for (const [label, name] of views) {
    await page.locator('.rail-item', { hasText: label }).click();
    await page.waitForTimeout(350);
    await shot(page, name);
  }

  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+k' : 'Control+k');
  await page.waitForSelector('.palette');
  await page.locator('.palette input').fill('run');
  await page.waitForTimeout(200);
  await shot(page, 'view-12-palette');
} finally {
  await app?.close().catch(() => {});
  await api.close();
  workspace.cleanup();
  if (home) fs.rmSync(home, { recursive: true, force: true });
}
console.log('done');
