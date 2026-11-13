import {expect, test} from "@playwright/test";
import {createApiKey, loginToAdmin} from "../helper/adminhelper";

// HTTP API keys are created on the admin page, one per service; each can be
// disabled and deleted without touching the others.
test.describe('admin API keys', () => {
    test('create, use, disable and delete keys', async ({page, request}) => {
        const call = async (key: string) => {
            const res = await request.get('/api/1.3.1/listAllPads', {params: {apikey: key}})
            return res.status()
        }
        await loginToAdmin(page, 'admin', 'changeme1')
        const suffix = Date.now().toString(36)
        const moodle = await createApiKey(page, `Moodle ${suffix}`)
        expect(moodle).toMatch(/^ep_[A-Za-z0-9]{40}$/)
        await page.getByRole('button', {name: 'Close'}).click()
        await expect(page.getByTestId('apikey-value')).toHaveCount(0)
        const backup = await createApiKey(page, `Backup ${suffix}`)

        expect(await call(moodle)).toBe(200)
        expect(await call(backup)).toBe(200)

        // the list shows the start of the key, never the key
        const row = page.getByTestId('apikey-row').filter({hasText: `Moodle ${suffix}`})
        await expect(row).toContainText(moodle.slice(0, 10))
        await expect(page.getByTestId('apikey-table')).not.toContainText(moodle)
        await page.reload()
        await page.getByRole('link', {name: 'API keys'}).click()
        await expect(row).not.toContainText('never', {timeout: 5000})

        await row.getByRole('button', {name: /disable/}).click()
        await expect(row.getByRole('button', {name: /enable/})).toBeVisible()
        expect(await call(moodle)).toBe(401)
        expect(await call(backup)).toBe(200)

        await row.getByRole('button', {name: /enable/}).click()
        await expect(row.getByRole('button', {name: /disable/})).toBeVisible()
        expect(await call(moodle)).toBe(200)

        await row.getByRole('button', {name: `Delete Moodle ${suffix}`}).click()
        await row.getByRole('button', {name: 'Delete for good'}).click()
        await expect(page.getByTestId('apikey-row').filter({hasText: `Moodle ${suffix}`})).toHaveCount(0)
        expect(await call(moodle)).toBe(401)
        expect(await call(backup)).toBe(200)
    })
})
