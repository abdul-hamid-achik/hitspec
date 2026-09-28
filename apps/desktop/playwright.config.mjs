import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './test/e2e',
  outputDir: './.playwright/artifacts',
  timeout: 180_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list']],
  use: {
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
});
