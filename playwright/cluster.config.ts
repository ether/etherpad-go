import {defineConfig, devices} from '@playwright/test';

// Two etherpad-go instances on one PostgreSQL database, started by
// cluster-spec/start.sh (CI) or by hand: EP_NODE_A / EP_NODE_B are their URLs.
const isCI = !!process.env.CI;

export default defineConfig({
  testDir: './cluster-spec',
  testMatch: '**/*.spec.ts',
  forbidOnly: isCI,
  retries: isCI ? 2 : 0,
  workers: isCI ? 2 : 1,
  reporter: isCI ? [['html', {open: 'never', outputFolder: 'playwright-report-cluster'}], ['github']] : 'line',
  timeout: 60000,
  expect: {timeout: 10000},
  use: {
    baseURL: process.env.EP_NODE_A ?? 'http://localhost:9001',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  // PW_CHANNEL=chrome uses an installed Chrome instead of Playwright's build.
  projects: [{name: 'chromium', use: {...devices['Desktop Chrome'], ...(process.env.PW_CHANNEL ? {channel: process.env.PW_CHANNEL} : {})}}],
});
