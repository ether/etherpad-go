import {expect, Page, test} from "@playwright/test";
import {issueToken, loginToAdmin} from "../helper/adminhelper";

const tokenRow = (page: Page, name: string) => page.locator(`[data-testid="token-row"][data-token="${name}"]`)

// Tokens are issued for an account, may at most what it may, and can be
// rotated with a grace period and revoked; the API takes them.
test.describe('admin tokens', () => {
    test('issue a scoped token for a service account, use, rotate and revoke it', async ({page, request}) => {
        await loginToAdmin(page, 'admin', 'changeme1')
        const suffix = Date.now().toString(36)
        const service = `pw-claude-${suffix}`
        const pad = `pw-claude-${suffix}-notes`
        const name = `Claude Desktop ${suffix}`

        // a service account that edits its own pads only
        await page.getByRole('link', {name: 'Accounts'}).click()
        await page.getByLabel('Kind').selectOption('service')
        await page.getByLabel('New username').fill(service)
        await page.getByLabel('New role').selectOption('editor')
        await page.getByLabel('New pads').fill(`pw-claude-${suffix}-*`)
        await page.getByRole('button', {name: 'Create', exact: true}).click()
        await expect(page.locator(`[data-testid="account-row"][data-username="${service}"]`)).toBeVisible()

        // what the account lacks cannot be picked
        await page.getByRole('link', {name: 'Tokens'}).click()
        const form = page.getByTestId('token-issue')
        await form.getByLabel('Account').selectOption({label: `${service} (service)`})
        await expect(form.getByRole('checkbox', {name: 'pad:write'})).toBeChecked()
        await expect(form.getByRole('checkbox', {name: 'pad:delete'})).toBeDisabled()
        await expect(form.getByRole('checkbox', {name: 'admin:users'})).toBeDisabled()
        await form.getByRole('checkbox', {name: 'chat:write'}).uncheck()

        await form.getByLabel('Token name').fill(name)
        await form.getByLabel('Token expiry').selectOption('30')
        await form.getByRole('button', {name: 'Issue'}).click()
        const value = page.getByTestId('token-key-value')
        await expect(value).toBeVisible()
        const key = (await value.textContent())!.trim()
        expect(key).toMatch(/^ep_/)
        await expect(form.getByLabel('Token name')).toHaveValue('', {timeout: 5000})

        const row = tokenRow(page, name)
        await expect(row).toContainText(service)
        await expect(row).toContainText('pad:write')
        await expect(row).not.toContainText('chat:write')
        await expect(row).toContainText(key.slice(0, 10))
        await expect(row).toHaveAttribute('data-status', 'active')

        // the API takes it, for what it may: no deleting (the account
        // lacks it), no admin functions. (Visitors' rights count too: on an
        // open instance it may create pads anywhere, as visitors may.)
        const api = (fn: string, k: string, params: Record<string, string>) =>
            request.get(`/api/1.3.1/${fn}`, {params: {apikey: k, ...params}})
        expect((await (await api('createPad', key, {padID: pad, text: 'hello'})).json()).code).toBe(0)
        expect((await api('deletePad', key, {padID: pad})).status()).toBe(403)
        expect((await api('listAllPads', key, {})).status()).toBe(403)

        // rotate: the old key works 10 more days, the new one at once
        await row.getByRole('button', {name: 'Rotate'}).click()
        await row.getByTestId('token-rotate').getByLabel('Old key works').selectOption('10')
        await row.getByTestId('token-rotate').getByRole('button', {name: 'Rotate'}).click()
        await expect(value).not.toHaveText(key)
        const newKey = (await value.textContent())!.trim()
        await expect(page.locator(`[data-testid="token-row"][data-token="${name}"][data-status="grace"]`)).toBeVisible()
        await expect(page.locator(`[data-testid="token-row"][data-token="${name}"][data-status="active"]`)).toBeVisible()
        expect((await (await api('getText', key, {padID: pad})).json()).code).toBe(0)
        expect((await (await api('getText', newKey, {padID: pad})).json()).code).toBe(0)
        await expect(page.locator(`[data-testid="token-row"][data-token="${name}"][data-status="grace"]`).getByTestId('token-soon')).toBeVisible()

        // revoke the old one now
        const old = page.locator(`[data-testid="token-row"][data-token="${name}"][data-status="grace"]`)
        await old.getByRole('button', {name: 'Revoke'}).click()
        await old.getByRole('button', {name: 'Revoke now'}).click()
        await expect(page.locator(`[data-testid="token-row"][data-token="${name}"]`)).toHaveCount(1)
        expect((await api('getText', key, {padID: pad})).status()).toBe(401)
        expect((await (await api('getText', newKey, {padID: pad})).json()).code).toBe(0)
        await page.getByLabel('Show revoked and expired').check()
        await expect(page.locator(`[data-testid="token-row"][data-token="${name}"][data-status="revoked"]`)).toBeVisible()

        // every step is in the audit log
        await page.getByRole('link', {name: 'Audit log'}).click()
        await page.getByLabel('Filter audit log').fill('token.')
        for (const action of ['token.issue', 'token.rotate', 'token.revoke']) {
            await expect(page.getByTestId('audit-row').filter({hasText: action}).first()).toBeVisible()
        }
    })

    test('a never-expiring token is marked; the former API keys page leads here', async ({page}) => {
        await loginToAdmin(page, 'admin', 'changeme1')
        const name = `pw-never-${Date.now().toString(36)}`
        await issueToken(page, name)
        await expect(tokenRow(page, name).getByTestId('token-never')).toBeVisible()
        await page.goto('/admin/keys')
        await expect(page).toHaveURL(/\/admin\/tokens$/)
        await tokenRow(page, name).getByRole('button', {name: 'Revoke'}).click()
        await tokenRow(page, name).getByRole('button', {name: 'Revoke now'}).click()
        await expect(tokenRow(page, name)).toHaveCount(0)
    })
})
