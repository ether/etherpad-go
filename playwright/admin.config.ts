import {defineConfig} from '@playwright/test';

// Admin specs need a server with SSO and the users of
// settings.template.json (admin and user, password "changeme1"):
//   npx playwright test -c admin.config.ts
// PW_CHANNEL=chrome uses an installed Chrome instead of Playwright's own.
// http_api_group_session.spec.ts needs authenticationMethod "apikey" and
// PW_APIKEY_FILE pointing at the server's APIKEY.txt.
export default defineConfig({
    testDir: './admin-spec',
    testMatch: process.env.PW_MATCH || '**/*.spec.ts',
    workers: 1,
    reporter: process.env.CI ? 'github' : 'line',
    timeout: 60000,
    use: {
        baseURL: 'http://localhost:9001',
        headless: true,
        ...(process.env.PW_CHANNEL ? {channel: process.env.PW_CHANNEL} : {}),
    },
});
