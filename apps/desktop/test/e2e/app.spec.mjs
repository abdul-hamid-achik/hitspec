import { expect, test, _electron as electron } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { createWorkspace, findHitspecBinary, startFixtureApi } from '../helpers/harness.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const desktopRoot = path.resolve(here, '..', '..');
const shotDir = path.join(desktopRoot, '.playwright', 'screenshots');

/**
 * Boots the real Electron app against a throwaway workspace and a throwaway
 * HOME, so the test neither touches the developer's settings nor writes runs
 * into ~/.hitspec/history.db.
 */
let app;
let page;
let api;
let workspace;
let workspace2;
let home;
let binary;
const consoleErrors = [];
const pageErrors = [];

test.describe.serial('hitspec Studio desktop app', () => {
  test.beforeAll(async () => {
    fs.mkdirSync(shotDir, { recursive: true });
    binary = await findHitspecBinary();
    api = await startFixtureApi();
    workspace = createWorkspace({ port: api.port, prefix: 'hitspec-e2e-' });
    workspace2 = createWorkspace({ port: api.port, prefix: 'hitspec-e2e-second-' });
    home = fs.mkdtempSync(path.join(os.tmpdir(), 'hitspec-e2e-home-'));

    app = await electron.launch({
      args: [desktopRoot, workspace.dir],
      cwd: desktopRoot,
      env: {
        ...process.env,
        HOME: home,
        USERPROFILE: home,
        HITSPEC_BIN: binary.path,
        HITSPEC_DESKTOP_USER_DATA: path.join(home, 'userdata'),
        ELECTRON_ENABLE_LOGGING: '1',
      },
    });

    page = await app.firstWindow();
    page.on('console', (msg) => {
      const text = msg.text();
      if (msg.type() === 'error' && !/favicon/i.test(text)) consoleErrors.push(text);
    });
    page.on('pageerror', (err) => pageErrors.push(String(err?.stack ?? err)));

    await expect(page).toHaveTitle('hitspec Studio');
    // The backend is spawned asynchronously after the window opens.
    await expect(page.locator('.workspace-chip .name')).toHaveText(path.basename(workspace.dir), {
      timeout: 60_000,
    });
  });

  test.afterAll(async () => {
    if (app) {
      if (consoleErrors.length || pageErrors.length) {
        console.error('renderer console errors:', consoleErrors);
        console.error('renderer page errors:', pageErrors);
      }
      await app.close().catch(() => {});
    }
    await api?.close();
    if (workspace) workspace.cleanup();
    if (workspace2) workspace2.cleanup();
    if (home) fs.rmSync(home, { recursive: true, force: true });
  });

  test('boots with the workspace loaded and no renderer errors', async () => {
    // The workspace path lives in the statusbar; the chip only shows the name.
    await expect(page.locator('#statusbar .item').first()).toHaveText(workspace.dir);
    await expect(page.locator('.status-group')).toContainText('ready');

    // Both fixture directories are discovered and the file/request counts add up.
    await expect(page.locator('.tree-row.dir')).toHaveCount(2);
    await expect(page.locator('#statusbar')).toContainText('3 files');
    await expect(page.locator('#statusbar')).toContainText('8 requests');
    await expect(page.locator('#statusbar')).toContainText('env dev');
    // The realtime WebSocket bridge from the main process is up.
    await expect(page.locator('#statusbar')).toContainText('live', { timeout: 30_000 });

    expect(pageErrors, `renderer threw: ${pageErrors.join('\n')}`).toEqual([]);
    await page.screenshot({ path: path.join(shotDir, '01-boot.png') });
  });

  test('browses the file tree and opens a file', async () => {
    await page.locator('.tree-row.dir', { hasText: 'api' }).click();
    const healthFile = page.locator('.tree-row', { hasText: 'health.http' });
    await expect(healthFile).toBeVisible();
    await healthFile.click();

    await expect(page.locator('#statusbar')).toContainText('api/health.http');
    await expect(page.locator('.req-row')).toHaveCount(1);
    await expect(page.locator('.req-row .req-name')).toHaveText('health');
  });

  test('inspects the parsed request', async () => {
    await page.locator('.req-row').click();
    await expect(page.locator('.view-head h2')).toHaveText('health');
    await expect(page.locator('.view-head .method')).toHaveText('GET');
    await expect(page.locator('.view-head .sub')).toContainText('{{baseUrl}}/health');

    await expect(page.locator('.card-head h3', { hasText: 'URL' })).toBeVisible();
    await expect(page.locator('.card-head h3', { hasText: 'Headers' })).toBeVisible();
    await expect(page.locator('.card-head h3', { hasText: 'Assertions' })).toBeVisible();
    await expect(page.locator('.view-head .tag', { hasText: 'smoke' })).toBeVisible();

    // {{baseUrl}} is highlighted as an interpolation, not as plain text.
    await expect(page.locator('.tok-var').first()).toHaveText('{{baseUrl}}');
    await page.screenshot({ path: path.join(shotDir, '02-request.png') });
  });

  test('runs the request and shows the response', async () => {
    await page.locator('.view-head .btn', { hasText: 'Run' }).first().click();

    await expect(page.locator('.stat', { hasText: 'Status' }).locator('.value')).toHaveText('200', {
      timeout: 60_000,
    });
    await expect(page.locator('.stat', { hasText: 'Assertions' }).locator('.value')).toHaveText('4/4');
    await expect(page.locator('.tabs-wrap .tab.active')).toHaveText(/Response/);

    const body = page.locator('pre.code').first();
    await expect(body).toContainText('hitspec-fixture');

    await page.locator('.tab', { hasText: 'Assertions' }).click();
    await expect(page.locator('.assertion.pass')).toHaveCount(4);
    await expect(page.locator('.assertion.fail')).toHaveCount(0);

    await page.locator('.tab', { hasText: 'Headers' }).click();
    await expect(page.locator('.kv .k', { hasText: 'content-type' })).toBeVisible();

    await page.locator('.tab', { hasText: 'Body' }).click();
    await page.screenshot({ path: path.join(shotDir, '03-response.png') });
  });

  test('runs a whole file, resolving captures and dependencies', async () => {
    // The api directory is already expanded from the earlier navigation.
    await page.locator('.tree-row', { hasText: 'users.http' }).click();
    await expect(page.locator('.req-row')).toHaveCount(5);

    await page.locator('.view-head .btn', { hasText: 'Run file' }).click();

    await expect(page.locator('.pill', { hasText: '5 passed' })).toBeVisible({ timeout: 60_000 });
    await expect(page.locator('.btn.sm .method')).toHaveCount(5);
    await page.screenshot({ path: path.join(shotDir, '04-run-file.png') });
  });

  test('edits and saves the source with syntax highlighting', async () => {
    // The editor always tracks the selected file; the previous test left
    // users.http selected.
    await page.locator('.tree-row', { hasText: 'health.http' }).click();
    await page.locator('.tab', { hasText: 'Source' }).click();

    const editor = page.locator('.editor-input');
    await expect(editor).toHaveValue(/### Health check/);
    // The highlight layer mirrors the buffer.
    await expect(page.locator('.editor-hl')).toContainText('Health check');
    await expect(page.locator('.editor-gutter')).toContainText('1');

    await editor.click();
    // Put the caret at the very top so the inserted comment cannot land inside
    // a request block and change how the file parses.
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+ArrowUp' : 'Control+Home');
    await page.keyboard.type('# edited by the e2e test\n');
    await expect(page.locator('.pill', { hasText: 'unsaved changes' })).toBeVisible();

    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+s' : 'Control+s');
    await expect(page.locator('.pill', { hasText: 'unsaved changes' })).toHaveCount(0, { timeout: 30_000 });

    const onDisk = fs.readFileSync(workspace.file('api', 'health.http'), 'utf8');
    expect(onDisk).toContain('# edited by the e2e test');
    await page.screenshot({ path: path.join(shotDir, '05-source.png') });
  });

  test('shows the run in persistent history', async () => {
    await page.locator('.rail-item', { hasText: 'History' }).click();
    await expect(page.locator('.table tbody tr')).not.toHaveCount(0, { timeout: 30_000 });
    await expect(page.locator('.table')).toContainText('users.http');

    await page.locator('.table tbody tr').first().click();
    await expect(page.locator('.modal')).toBeVisible();
    await expect(page.locator('.modal')).toContainText('Run #');
    await page.screenshot({ path: path.join(shotDir, '06-history.png') });
    await page.keyboard.press('Escape');
    await expect(page.locator('.modal')).toHaveCount(0);
  });

  test('reads stress profiles from hitspec.yaml', async () => {
    await page.locator('.rail-item', { hasText: 'Stress' }).click();
    await expect(page.locator('.card-head h3', { hasText: 'Load profile' })).toBeVisible();
    await expect(page.locator('.card-head h3', { hasText: 'Saved profiles' })).toBeVisible();
    await expect(page.locator('.card', { hasText: 'Saved profiles' })).toContainText('quick');
    await page.screenshot({ path: path.join(shotDir, '07-stress.png') });
  });

  test('switches environments from the topbar', async () => {
    const select = page.locator('.topbar select');
    await expect(select).toHaveValue('dev');
    await select.selectOption('staging');
    await expect(select).toHaveValue('staging');

    await page.locator('.rail-item', { hasText: 'Requests' }).click();
    await expect(page.locator('#statusbar')).toContainText('env staging');

    await select.selectOption('dev');
    await expect(page.locator('#statusbar')).toContainText('env dev');
  });

  test('imports a curl command into a preview', async () => {
    await page.locator('.rail-item', { hasText: 'Import' }).click();
    await page.locator('.tab', { hasText: 'curl' }).click();

    await page
      .locator('textarea[aria-label="Import source"]')
      .fill(`curl -X POST '${api.url}/users' -H 'Content-Type: application/json' -d '{"name":"Imported"}'`);
    await page.locator('.view-head .btn', { hasText: 'Import' }).click();

    await expect(page.locator('.card-head h3', { hasText: 'Preview' })).toBeVisible({ timeout: 30_000 });
    await expect(page.locator('.pill', { hasText: 'request(s)' })).toBeVisible();
    await page.screenshot({ path: path.join(shotDir, '08-import.png') });
  });

  test('drives everything from the command palette', async () => {
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+k' : 'Control+k');
    const palette = page.locator('.palette');
    await expect(palette).toBeVisible();
    await expect(palette.locator('.palette-item')).not.toHaveCount(0);

    await palette.locator('input').fill('mock');
    await page.keyboard.press('Enter');
    await expect(page.locator('.view-head h2')).toHaveText('Mock server');

    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+k' : 'Control+k');
    await page.locator('.palette input').fill('switch to staging');
    await page.keyboard.press('Enter');
    await expect(page.locator('.topbar select')).toHaveValue('staging');
    await page.locator('.topbar select').selectOption('dev');
  });

  test('changes theme and persists settings', async () => {
    await page.locator('.rail-item', { hasText: 'Settings' }).click();
    await expect(page.locator('.card-head h3', { hasText: 'API server' })).toBeVisible();
    await expect(
      page.locator('.card', { has: page.locator('.card-head h3', { hasText: 'API server' }) }),
    ).toContainText(binary.path);

    await page.locator('.card', { hasText: 'Appearance' }).locator('.btn', { hasText: 'Dracula' }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dracula');
    await page.screenshot({ path: path.join(shotDir, '09-settings-dracula.png') });

    await page.locator('.card', { hasText: 'Appearance' }).locator('.btn', { hasText: 'Nord' }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'nord');

    const settingsPath = path.join(home, 'userdata', 'settings.json');
    await expect
      .poll(() => (fs.existsSync(settingsPath) ? JSON.parse(fs.readFileSync(settingsPath, 'utf8')).theme : null))
      .toBe('nord');
  });

  test('renders the remaining engine views without errors', async () => {
    const views = [
      ['Mock', 'Mock server'],
      ['Record', 'Recording proxy'],
      ['Contracts', 'Contract testing'],
    ];
    for (const [nav, heading] of views) {
      await page.locator('.rail-item', { hasText: nav }).click();
      await expect(page.locator('.view-head h2')).toHaveText(heading);
      // Each view must produce real content, not an empty pane.
      await expect(page.locator('.card').first()).toBeVisible();
    }
    await page.screenshot({ path: path.join(shotDir, '10-contracts.png') });
    await page.locator('.rail-item', { hasText: 'Requests' }).click();
  });

  test('switches and manages projects from the workspace switcher', async () => {
    // Open a second project through the same bridge the switcher uses.
    await page.evaluate((dir) => window.hitspec.openWorkspacePath(dir), workspace2.dir);
    await expect(page.locator('.workspace-chip .name')).toHaveText(path.basename(workspace2.dir), {
      timeout: 60_000,
    });

    await page.locator('.workspace-chip').click();
    const popover = page.locator('.popover');
    await expect(popover).toBeVisible();
    await expect(popover.locator('.switcher-main')).toHaveCount(2);
    await expect(popover).toContainText(path.basename(workspace.dir));
    await expect(popover.locator('.switcher-main.active .name')).toHaveText(path.basename(workspace2.dir));
    await page.screenshot({ path: path.join(shotDir, '13-switcher.png') });

    // Switch back to the first project through the UI.
    await popover.locator('.switcher-main', { hasText: path.basename(workspace.dir) }).click();
    await expect(page.locator('.workspace-chip .name')).toHaveText(path.basename(workspace.dir), {
      timeout: 60_000,
    });

    // Drop the second project from the recents list.
    await page.locator('.workspace-chip').click();
    await popover
      .locator('.switcher-row', { hasText: path.basename(workspace2.dir) })
      .locator('button.btn')
      .click();
    await expect(popover.locator('.switcher-main')).toHaveCount(1);
    await page.keyboard.press('Escape');
    await expect(popover).toHaveCount(0);
  });

  test('leaves no renderer errors behind', async () => {
    expect(pageErrors, `renderer threw:\n${pageErrors.join('\n')}`).toEqual([]);
    expect(consoleErrors, `console errors:\n${consoleErrors.join('\n')}`).toEqual([]);
  });
});
