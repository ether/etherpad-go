import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({ page, browser })=>{
    const context = await browser.newContext()
    await context.clearCookies()
    await goToNewPad(page);
})

// Skip on WebKit - the nice-select dropdown doesn't work reliably on Safari
test.describe('Language select and change', function () {
    test.skip(({ browserName }) => browserName === 'webkit', 'Skipping on WebKit due to dropdown issues');

    test('makes text german', async function ({page}) {
        await showSettings(page);
        const languageDropDown = page.locator('.nice-select').nth(1);
        await languageDropDown.click();
        await page.locator('.nice-select.open [data-value=de]').click();
        await expect(languageDropDown.locator('.current')).toHaveText('Deutsch');
    });

    test('makes text English', async function ({page}) {
        await showSettings(page);
        const languageDropDown = page.locator('.nice-select').nth(1);

        // Select German first
        await languageDropDown.click();
        await page.locator('.nice-select.open [data-value=de]').click();
        await expect(languageDropDown.locator('.current')).toHaveText('Deutsch');

        // Now change to English
        await showSettings(page);
        await languageDropDown.click();
        await page.locator('.nice-select.open [data-value=en]').click();
        await expect(languageDropDown.locator('.current')).toHaveText('English');
    });

    test('changes direction when picking an rtl lang', async function ({page}) {
        await showSettings(page);
        const languageDropDown = page.locator('.nice-select').nth(1);
        await languageDropDown.click();
        await page.locator('.nice-select.open [data-value=ar]').click();
        await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
    });

    test('changes direction when picking an ltr lang', async function ({page}) {
        await showSettings(page);
        const languageDropDown = page.locator('.nice-select').nth(1);
        await languageDropDown.click();
        await page.locator('.nice-select.open [data-value=en]').click();
        await expect(languageDropDown.locator('.current')).toHaveText('English');
        await expect(page.locator('html')).toHaveAttribute('dir', 'ltr');
    });
});
