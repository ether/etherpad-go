import {expect, test} from "@playwright/test";
import {getPadBody, goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({ page, browser })=>{
    const context = await browser.newContext()
    await context.clearCookies()
    await goToNewPad(page);
})

test.describe('Language select and change', function () {

    // Destroy language cookies
    test('makes text german', async function ({page}) {
        // click on the settings button to make settings visible
        await showSettings(page)
        await page.waitForTimeout(300);

        // Wait for language dropdown to be visible
        const languageDropDown = page.locator('.nice-select').nth(1);
        await languageDropDown.waitFor({ state: 'visible', timeout: 10000 });

        await languageDropDown.click();
        await page.waitForTimeout(200);

        await page.locator('.nice-select.open [data-value=de]').click();
        await page.waitForTimeout(500);

        await expect(languageDropDown.locator('.current')).toHaveText('Deutsch', { timeout: 10000 });
    });

    test('makes text English', async function ({page}) {
        await showSettings(page);
        await page.waitForTimeout(300);

        // Wait for language dropdown to be visible
        const languageDropDown = page.locator('.nice-select').nth(1);
        await languageDropDown.waitFor({ state: 'visible', timeout: 10000 });

        // Select German first
        await languageDropDown.click();
        await page.waitForTimeout(200);
        await page.locator('.nice-select.open [data-value=de]').click();
        await page.waitForTimeout(500);

        // Now change to English
        await showSettings(page);
        await page.waitForTimeout(300);

        await languageDropDown.click();
        await page.waitForTimeout(200);
        await page.locator('.nice-select.open [data-value=en]').click();
        await page.waitForTimeout(500);

        await expect(languageDropDown.locator('.current')).toHaveText('English', { timeout: 10000 });
    });

    test('changes direction when picking an rtl lang', async function ({page}) {
        await showSettings(page);
        await page.waitForTimeout(300);

        // Wait for language dropdown to be visible
        const languageDropDown = page.locator('.nice-select').nth(1);
        await languageDropDown.waitFor({ state: 'visible', timeout: 10000 });

        // Select Arabic (RTL)
        await languageDropDown.click();
        await page.waitForTimeout(200);
        await page.locator('.nice-select.open [data-value=ar]').click();
        await page.waitForTimeout(500);

        // Check for RTL direction
        await page.waitForSelector('html[dir="rtl"]', { timeout: 10000 });
    });

    test('changes direction when picking an ltr lang', async function ({page}) {
        await showSettings(page);
        await page.waitForTimeout(300);

        // Wait for language dropdown to be visible
        const languageDropDown = page.locator('.nice-select').nth(1);
        await languageDropDown.waitFor({ state: 'visible', timeout: 10000 });

        // First set to English (LTR)
        await languageDropDown.click();
        await page.waitForTimeout(200);
        await page.locator('.nice-select.open [data-value=en]').click();
        await page.waitForTimeout(500);

        await expect(languageDropDown.locator('.current')).toHaveText('English', { timeout: 10000 });

        // Check for LTR direction
        await page.waitForSelector('html[dir="ltr"]', { timeout: 10000 });
    });
});
