import {expect, test} from "@playwright/test";
import {loginToAdmin} from "../helper/adminhelper";

// The Updates page: what runs, what the updater does, its log and where
// its mails go.
test.describe('admin updates page', () => {
    test('shows the updater status, mail notices and log', async ({page}) => {
        await loginToAdmin(page, 'admin', 'changeme1')
        await page.getByRole('link', {name: 'Updates'}).click()
        await expect(page).toHaveURL(/\/admin\/updates$/)
        await expect(page.getByRole('heading', {name: 'Updates'})).toBeVisible()
        await expect(page.getByTestId('update-status')).not.toHaveText('n/a', {timeout: 10000})
        await expect(page.getByTestId('update-admin-email')).toBeVisible()
        // No update scheduled or failed: nothing to cancel or acknowledge.
        await expect(page.getByTestId('update-cancel')).toHaveCount(0)
        await expect(page.getByTestId('update-acknowledge')).toHaveCount(0)

        // A check writes to the update log.
        await page.getByRole('button', {name: 'Check now'}).click()
        await expect(page.getByTestId('update-log')).toContainText('updater:', {timeout: 15000})
    })
})
