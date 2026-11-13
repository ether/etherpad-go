import {expect, test} from "@playwright/test";
import {randomUUID} from "node:crypto";
import {loginToAdmin} from "../helper/adminhelper";
import {getPadBody, goToPad} from "../helper/padHelper";

// The admin pad list filter runs on the server before paging (etherpad-lite
// admin/padLoadFilter), so a filter finds matches beyond the first page.
test.describe('admin pad filter', () => {
    test('"Empty" lists only untouched pads', async ({page, browser}) => {
        const tag = `pf${randomUUID().slice(0, 8)}`
        const untouched = [`${tag}-a`, `${tag}-b`]
        const edited = `${tag}-edited`

        const padPage = await browser.newPage()
        for (const id of untouched) await goToPad(padPage, id)
        await goToPad(padPage, edited)
        const body = await getPadBody(padPage)
        await body.click()
        await padPage.keyboard.type('changed')
        await expect(body).toContainText('changed')
        await padPage.waitForTimeout(1000)
        await padPage.close()

        await loginToAdmin(page, 'admin', 'changeme1')
        await page.getByRole('link', {name: 'Pads'}).click()
        const search = page.getByPlaceholder('Search pads...')
        await search.fill(tag)
        await search.press('Enter')
        const rows = page.locator('tbody tr')
        await expect(rows).toHaveCount(3)

        await page.getByTestId('pad-filter').selectOption('empty')
        await expect(rows).toHaveCount(2)
        for (const id of untouched) await expect(page.getByText(id, {exact: true})).toBeVisible()
        await expect(page.getByText(edited, {exact: true})).toHaveCount(0)

        await page.getByTestId('pad-filter').selectOption('all')
        await expect(rows).toHaveCount(3)
    })
})
