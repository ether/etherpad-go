import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';
import os from 'node:os';

const isWindows = os.platform() === 'win32';
const isLinux = os.platform() === 'linux';
const isMac = os.platform() === 'darwin';
const isCI = !!process.env.CI;
const isARM = os.arch() === 'arm64';

process.env['NODE_ENV'] = 'production';
process.env['ETHERPAD_LOADTEST'] = 'true'
process.env['ETHERPAD_DEVMODE'] = 'false'

// ARM CI runners are slower — use fewer workers and longer timeouts
const ciWorkers = isARM ? 2 : 4;
const ciTimeout = isARM ? 90000 : 60000;
const ciNavTimeout = isARM ? 45000 : 30000;

export default defineConfig({
  fullyParallel: true,
  testDir: '.',
  testMatch: 'specs/**/*.spec.ts',
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  workers: isCI ? ciWorkers : '75%',
  reporter: isCI ? [['html', { open: 'never' }], ['github']] : 'html',
  timeout: isCI ? ciTimeout : 30000,
  expect: {
    timeout: isCI ? 10000 : 5000,
  },
  use: {
    baseURL: 'http://localhost:9001',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    actionTimeout: isCI ? 10000 : 5000,
    navigationTimeout: isCI ? ciNavTimeout : 15000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    // Firefox only on CI Linux (most stable there), skip on Windows
    ...(isWindows ? [] : isCI && !isLinux ? [] : [{
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    }]),
  ],
  webServer: {
    command: isWindows ? 'cmd /c "go build -o etherpad-go.exe . && etherpad-go.exe"' : 'go build -o etherpad-go . && ./etherpad-go',
    cwd: path.resolve(__dirname, '..'),
    url: 'http://localhost:9001',
    reuseExistingServer: !isCI,
    timeout: 180000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
