import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';
import os from 'node:os';

const isWindows = os.platform() === 'win32';
const isLinux = os.platform() === 'linux';
const isMac = os.platform() === 'darwin';
const isCI = !!process.env.CI;

process.env['NODE_ENV'] = 'production';
process.env['ETHERPAD_LOADTEST'] = 'true'

export default defineConfig({
  fullyParallel: true,
  testDir: '.',
  testMatch: 'specs/**/*.spec.ts',
  forbidOnly: isCI,
  retries: isCI ? 2 : 0,
  // Reduce workers in CI to avoid overwhelming the server
  workers: isCI ? 2 : '75%',
  reporter: isCI ? [['html', { open: 'never' }], ['github']] : 'html',
  timeout: isCI ? 120000 : 60000,
  expect: {
    timeout: isCI ? 20000 : 10000,
  },
  use: {
    baseURL: 'http://localhost:9001',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    actionTimeout: isCI ? 20000 : 10000,
    navigationTimeout: isCI ? 60000 : 30000,
  },
  projects: [
      ...(isWindows ? [] : [{
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    }]),
      ...(isLinux ? [] : [{
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
      }]),
    // Only run webkit on macOS where it's most stable
    ...(isMac ? [{
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    }] : []),
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
