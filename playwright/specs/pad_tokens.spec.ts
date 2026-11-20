import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

// "My tokens" in the pad's profile menu: issue a token, use its key once,
// give it a new key and revoke both.
test.describe('my tokens', () => {
    test('issue, rotate and revoke own tokens', async ({page, request}) => {
        const available = (await (await request.get('/auth/me')).json()).loginAvailable
        test.skip(!available, 'no SSO client of type "user" on this server')
        const padId = await goToNewPad(page)
        const account = page.locator('#editbar li[data-key="account"]')
        await account.getByTestId('account-login').click()
        await page.waitForSelector('input[name="username"]')
        await page.fill('input[name="username"]', 'admin')
        await page.fill('input[name="password"]', 'changeme1')
        await page.click('input[type="submit"]')
        await page.waitForURL((u) => u.pathname === `/p/${padId}`)
        await page.waitForSelector('#editorcontainer.initialized')

        await account.getByTestId('account-button').click()
        await account.getByTestId('account-tokens-open').click()
        const dialog = page.getByTestId('account-tokens')
        await expect(dialog).toBeVisible()
        await expect(account.getByTestId('account-menu')).toBeHidden()

        const name = `pw-${Date.now()}-${Math.floor(Math.random() * 1e6)}`
        await dialog.getByTestId('tokens-name').fill(name)
        await expect(dialog.getByTestId('tokens-scope-pad:read')).toBeChecked()
        await dialog.getByTestId('tokens-pads').fill(`${padId}`)
        await dialog.getByTestId('tokens-issue').click()

        const key = dialog.getByTestId('tokens-key-value')
        await expect(key).toHaveValue(/^ep_/)
        const value = await key.inputValue()
        const mine = dialog.getByTestId('token').filter({hasText: name})
        await expect(mine).toHaveCount(1)

        // The key reads this pad and nothing else.
        const read = await request.get(`/api/1/getText?padID=${encodeURIComponent(padId)}`,
            {headers: {Authorization: `Bearer ${value}`}})
        expect((await read.json()).code).toBe(0)
        const other = await request.get(`/api/1/getText?padID=${encodeURIComponent(padId + 'x')}`,
            {headers: {Authorization: `Bearer ${value}`}})
        expect((await other.json()).code).not.toBe(0)

        // A new key; the old one keeps working for a day.
        await mine.getByTestId('token-grace-choice').selectOption('1')
        await mine.getByTestId('token-rotate').click()
        await expect(key).not.toHaveValue(value)
        await expect(dialog.getByTestId('token').filter({hasText: name})).toHaveCount(2)
        await expect(dialog.getByTestId('token-grace')).toHaveCount(1)

        // Revoke asks once more, then both go.
        for (let i = 0; i < 2; i++) {
            const tk = dialog.getByTestId('token').filter({hasText: name}).first()
            await tk.getByTestId('token-revoke').click()
            await expect(tk.getByTestId('token-revoke')).toHaveText(/really|wirklich/i)
            await tk.getByTestId('token-revoke').click()
            await expect(dialog.getByTestId('token').filter({hasText: name})).toHaveCount(1 - i)
        }
        const gone = await request.get(`/api/1/getText?padID=${encodeURIComponent(padId)}`,
            {headers: {Authorization: `Bearer ${value}`}})
        expect((await gone.json()).code).not.toBe(0)

        await dialog.getByTestId('tokens-close').click()
        await expect(dialog).toHaveCount(0)
    })
})
