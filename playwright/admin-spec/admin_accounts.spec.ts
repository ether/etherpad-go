import {expect, Page, test} from "@playwright/test";
import {loginToAdmin} from "../helper/adminhelper";

const accountRow = (page: Page, username: string) =>
    page.locator(`[data-testid="account-row"][data-username="${username}"]`)

// Accounts, roles and visitors' rights are managed on the admin pages; every
// change lands in the audit log.
test.describe('admin accounts', () => {
    test('create a person, give roles, reset, disable and delete', async ({page, browser}) => {
        await loginToAdmin(page, 'admin', 'changeme1')
        await page.getByRole('link', {name: 'Accounts'}).click()
        const anna = `pw-anna-${Date.now().toString(36)}`

        await page.getByLabel('New username').fill(anna)
        await page.getByLabel('New display name').fill('Anna Test')
        await page.getByLabel('New role').selectOption('editor')
        await page.getByLabel('New pads').fill('team/*')
        await page.getByRole('button', {name: 'Create', exact: true}).click()

        const password = page.getByTestId('account-password-value')
        await expect(password).toBeVisible()
        const oneTime = (await password.textContent())!.trim()
        expect(oneTime.length).toBeGreaterThanOrEqual(12)
        const row = accountRow(page, anna)
        await expect(row).toContainText('Anna Test')
        await expect(row.getByTestId('role-chip')).toHaveText(/editor.*team\/\*/)
        await expect(row).toContainText('must change password')

        // the one-time password must be changed at the first login
        const other = await browser.newContext()
        const login = await other.newPage()
        await loginToAdmin(login, anna, oneTime)
        await expect(login.locator('input[name="new_password"]')).toBeVisible()
        await other.close()

        // a second role for all pads, then the first one goes
        await row.getByRole('button', {name: `Add a role to ${anna}`}).click()
        await row.getByLabel('Role').selectOption('ai-user')
        await row.getByRole('button', {name: 'Add', exact: true}).click()
        await expect(row.getByTestId('role-chip')).toHaveCount(2)
        await row.getByRole('button', {name: 'Remove editor for team/*'}).click()
        await expect(row.getByTestId('role-chip')).toHaveCount(1)
        await expect(row.getByTestId('role-chip')).toHaveText(/ai-user.*all pads/)

        await row.getByRole('button', {name: `Edit ${anna}`}).click()
        await row.getByLabel('E-mail').fill('anna@example.org')
        await row.getByRole('button', {name: 'Save'}).click()
        await expect(row).toContainText('anna@example.org')

        await page.getByRole('button', {name: 'Close'}).click()
        await row.getByRole('button', {name: 'Reset password'}).click()
        await expect(password).toBeVisible()
        expect((await password.textContent())!.trim()).not.toBe(oneTime)

        await row.getByRole('button', {name: /Active – disable/}).click()
        await expect(row.getByRole('button', {name: /Disabled – enable/})).toBeVisible()

        // the last admin stays an admin
        const admin = accountRow(page, 'admin')
        await admin.getByRole('button', {name: /Active – disable/}).click()
        await expect(page.getByTestId('accounts-error')).toContainText('last active admin')

        await row.getByRole('button', {name: `Delete ${anna}`}).click()
        await row.getByRole('button', {name: 'Delete for good'}).click()
        await expect(accountRow(page, anna)).toHaveCount(0)

        await page.getByRole('link', {name: 'Audit log'}).click()
        await page.getByLabel('Filter audit log').fill(anna)
        await expect(page.getByTestId('audit-row').filter({hasText: 'account.create'})).toHaveCount(1)
        await expect(page.getByTestId('audit-row').filter({hasText: 'assignment.remove'})).toHaveCount(1)
    })

    test('roles: create, edit, delete; built-in roles reset', async ({page}) => {
        await loginToAdmin(page, 'admin', 'changeme1')
        await page.getByRole('link', {name: 'Roles'}).click()
        const name = `pw-role-${Date.now().toString(36)}`

        await page.getByRole('button', {name: 'New role'}).click()
        const editor = page.getByTestId('role-editor')
        await editor.getByLabel('Role name').fill(name)
        await editor.getByLabel('Role description').fill('AI under claude/')
        await editor.getByRole('checkbox', {name: /^ai:use/}).check()
        await editor.getByRole('button', {name: 'Add limit'}).click()
        await editor.getByLabel('Limit name').fill('ai.requestsPerDay')
        await editor.getByLabel('Limit value').fill('50')
        await editor.getByRole('button', {name: 'Save role'}).click()
        await expect(editor).toHaveCount(0)

        const card = page.getByTestId('role-card').filter({hasText: name})
        await expect(card).toContainText('ai:use')
        await expect(card).toContainText('ai.requestsPerDay: 50')

        // nobody gives rights they do not have, and the admin role is fixed
        const adminCard = page.locator('[data-testid="role-card"][data-role="admin"]')
        await expect(adminCard.getByRole('button', {name: 'Edit'})).toHaveCount(0)

        // a built-in role changed and reset
        const viewer = page.locator('[data-testid="role-card"][data-role="viewer"]')
        await viewer.getByRole('button', {name: 'Edit'}).click()
        await editor.getByRole('checkbox', {name: /^chat:write/}).check()
        await editor.getByRole('button', {name: 'Save role'}).click()
        await expect(viewer).toContainText('changed')
        await expect(viewer).toContainText('chat:write')
        await viewer.getByRole('button', {name: 'Reset'}).click()
        await expect(viewer).not.toContainText('changed')
        await expect(viewer).not.toContainText('chat:write')

        await card.getByRole('button', {name: `Delete ${name}`}).click()
        await card.getByRole('button', {name: 'Delete for good'}).click()
        await expect(page.getByTestId('role-card').filter({hasText: name})).toHaveCount(0)
    })

    test('visitors: roles of everyone logged in', async ({page}) => {
        await loginToAdmin(page, 'admin', 'changeme1')
        await page.getByRole('link', {name: 'Visitors'}).click()
        await expect(page.getByTestId('template-open')).toBeVisible()
        const everyone = page.getByTestId('pseudo-authenticated')
        const before = await everyone.getByTestId('role-chip').count()

        await everyone.getByRole('button', {name: /Add a role to/}).click()
        // admin roles are not offered for everyone
        await expect(everyone.getByLabel('Role').locator('option[value="admin"]')).toHaveCount(0)
        await everyone.getByLabel('Role').selectOption('ai-user')
        await everyone.getByRole('button', {name: 'Add', exact: true}).click()
        await expect(everyone.getByTestId('role-chip')).toHaveCount(before + 1)
        await everyone.getByRole('button', {name: 'Remove ai-user for all pads'}).click()
        await expect(everyone.getByTestId('role-chip')).toHaveCount(before)
    })
})
