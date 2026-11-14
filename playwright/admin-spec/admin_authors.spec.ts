import {expect, test} from "@playwright/test";
import {loginToAdmin} from "../helper/adminhelper";
import {goToNewPad, setUserName, toggleUserList} from "../helper/padHelper";

// The Authors page finds an author and erases them (GDPR Art. 17) after a
// dry run. Needs gdprAuthorErasure.enabled; without it the page says so.
test.describe('admin authors', () => {
    test('find and erase an author', async ({page, browser}) => {
        const name = `Erase me ${Date.now().toString(36)}`
        const writer = await browser.newContext()
        try {
            const pad = await writer.newPage()
            await goToNewPad(pad)
            await toggleUserList(pad)
            await setUserName(pad, name)
            await pad.keyboard.press('Enter')
            await pad.waitForTimeout(500)
        } finally {
            await writer.close()
        }

        await loginToAdmin(page, 'admin', 'changeme1')
        await page.getByRole('link', {name: 'Authors'}).click()
        const disabled = page.getByTestId('authors-disabled')
        const search = page.getByLabel('Search authors')
        await expect(disabled.or(search)).toBeVisible()
        test.skip(await disabled.isVisible(), 'gdprAuthorErasure.enabled is off on this server')

        await search.fill(name)
        const row = page.getByTestId('author-row').filter({hasText: name})
        await expect(row).toHaveCount(1)
        await row.getByRole('button', {name: `Erase ${name}`}).click()
        await expect(page.getByTestId('erasure-preview')).toContainText('1 pad(s)')
        await page.getByRole('button', {name: 'Erase for good'}).click()

        // Erased authors lose their name, so the search no longer finds them.
        await expect(page.getByTestId('author-row')).toHaveCount(0)
        await expect(page.getByTestId('authors-total')).toContainText('0 authors')
    })
})
