import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';
import os from 'node:os';

const isWindows = os.platform() === 'win32';


process.env['NODE_ENV'] = 'production';
process.env['ETHERPAD_LOADTEST'] = 'true'

export default defineConfig({
  fullyParallel: true,
  testDir: '.',
  testMatch: 'specs/**/*.spec.ts',
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 5,
  reporter: process.env.CI ? [['html', { open: 'never' }], ['github']] : 'html',
  use: {
    baseURL: 'http://localhost:9001',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
  ],
  webServer: {
    command: isWindows ? 'cmd /c "go build -o etherpad-go.exe . && etherpad-go.exe"' : 'go build -o etherpad-go . && ./etherpad-go',
    cwd: path.resolve(__dirname, '..'),
    url: 'http://localhost:9001',
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
  },
});
