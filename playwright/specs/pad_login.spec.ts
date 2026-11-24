import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

// The optional login on pads: "Log in" in the toolbar, the provider's
// login page, back to the same pad with the profile menu, "Log out".
test.describe('pad login', () => {
    test('without a login client the toolbar has no account entry', async ({page, request}) => {
        const me = await (await request.get('/auth/me')).json()
        test.skip(me.loginAvailable, 'this server has a login client')
        expect(me.loggedIn).toBe(false)
        await goToNewPad(page)
        await expect(page.locator('#editbar li[data-key="showusers"]')).toBeVisible()
        await expect(page.locator('#editbar li[data-key="account"]')).toHaveCount(0)
    })

    test('log in from a pad and out again', async ({page, request}) => {
        const available = (await (await request.get('/auth/me')).json()).loginAvailable
        test.skip(!available, 'no SSO client of type "user" on this server')
        const padId = await goToNewPad(page)
        const account = page.locator('#editbar li[data-key="account"]')
        const login = account.getByTestId('account-login')
        await expect(login).toBeVisible()
        await expect(login).toHaveText(/log in|anmelden/i)

        await login.click()
        await page.waitForSelector('input[name="username"]')
        await page.fill('input[name="username"]', 'admin')
        await page.fill('input[name="password"]', 'changeme1')
        await page.click('input[type="submit"]')

        // back on the same pad, logged in
        await page.waitForURL((u) => u.pathname === `/p/${padId}`)
        await page.waitForSelector('#editorcontainer.initialized')
        const button = account.getByTestId('account-button')
        await expect(button).toHaveText('A')
        await expect(button).toHaveAttribute('aria-expanded', 'false')
        await button.click()
        const menu = account.getByTestId('account-menu')
        await expect(menu).toBeVisible()
        await expect(menu.getByTestId('account-name')).toHaveText('admin')
        await expect(menu.locator('a.account-admin')).toBeVisible()
        // Escape and a click elsewhere close it
        await button.press('Escape')
        await expect(menu).toBeHidden()

        // the pad's author is the account's, also in another browser tab
        await expect(page.locator('#userlist input')).toHaveValue('admin')

        // the same pad open in a second tab of this browser
        const other = await page.context().newPage()
        await other.goto(page.url())
        await other.waitForSelector('#editorcontainer.initialized')
        await expect(other.locator('#editbar').getByTestId('account-button')).toBeVisible()

        await button.click()
        await menu.getByTestId('account-logout').click()
        // the logout ends that tab's connection too: it does not keep
        // writing with the account's rights
        await expect(other.locator('#connectivity .kicked')).toBeVisible()
        await page.waitForURL((u) => u.pathname === `/p/${padId}`)
        await page.waitForSelector('#editorcontainer.initialized')
        await expect(account.getByTestId('account-login')).toBeVisible()
        const me = await page.request.get('/auth/me')
        expect((await me.json()).loggedIn).toBe(false)
    })
})
