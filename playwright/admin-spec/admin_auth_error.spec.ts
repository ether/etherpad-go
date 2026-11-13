import {expect, test} from "@playwright/test";
import {loginToAdmin} from "../helper/adminhelper";

// Port of etherpad-lite's adminSettingsAuthError (#7819): a sign-in that is
// not an admin one used to leave the admin page retrying its connection
// forever, showing only "Disconnected". It must say what is wrong.
test.describe('admin auth error', () => {
    test('a non-admin sign-in is told so instead of retrying silently', async ({page}) => {
        const sockets: string[] = []
        page.on('websocket', ws => sockets.push(ws.url()))
        await loginToAdmin(page, 'user', 'changeme1');

        const alert = page.getByTestId('admin-auth-error')
        await expect(alert).toBeVisible({timeout: 15000})
        await expect(alert).toContainText('not signed in as an administrator')

        // It stops trying: no new connection attempts after the message.
        const attempts = sockets.length
        await page.waitForTimeout(5000)
        expect(sockets.length).toBe(attempts)
    })

    test('an admin sign-in connects', async ({page}) => {
        await loginToAdmin(page, 'admin', 'changeme1');
        await expect(page.getByText('Connected', {exact: true})).toBeVisible({timeout: 15000})
        await expect(page.getByTestId('admin-auth-error')).toHaveCount(0)
    })
})
