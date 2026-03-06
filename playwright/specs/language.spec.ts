import {expect, Page, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await page.context().clearCookies();
    await goToNewPad(page);
})

const selectLanguage = async (page: Page, language: string) => {
    const languageMenu = page.locator('#languagemenu');
    await page.waitForSelector('iframe[name="ace_outer"]');
    for (let i = 0; i < 3; i++) {
        if (await languageMenu.isVisible()) break;
        await page.locator("button[class~='buttonicon-settings']").click();
        await page.waitForTimeout(150);
    }
    await expect(languageMenu).toBeVisible();
    await Promise.all([
        page.waitForLoadState('load'),
        languageMenu.selectOption(language),
    ]);
};

// Skip on WebKit - the dropdown doesn't work reliably on Safari
test.describe('Language select and change', function () {
    test.skip(({ browserName }) => browserName === 'webkit', 'Skipping on WebKit due to dropdown issues');

    test('makes text german', async function ({page}) {
        await selectLanguage(page, 'de');
        await expect(page.locator('#languagemenu')).toHaveValue('de');
        await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    });

    test('makes text English', async function ({page}) {
        await selectLanguage(page, 'de');
        await selectLanguage(page, 'en');
        await expect(page.locator('#languagemenu')).toHaveValue('en');
        await expect(page.locator('html')).toHaveAttribute('lang', 'en');
    });

    test('changes direction when picking an rtl lang', async function ({page}) {
        await selectLanguage(page, 'ar');
        await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
    });

    test('changes direction when picking an ltr lang', async function ({page}) {
        await selectLanguage(page, 'ar');
        await selectLanguage(page, 'en');
        await expect(page.locator('#languagemenu')).toHaveValue('en');
        await expect(page.locator('html')).toHaveAttribute('dir', 'ltr');
    });

    test('keeps selected language after reload', async function ({page}) {
        await selectLanguage(page, 'de');
        await goToNewPad(page);
        await expect(page.locator('#languagemenu')).toHaveValue('de');
        await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    });

    test('stores language in cookie', async function ({page}) {
        await selectLanguage(page, 'de');
        const cookies = await page.context().cookies();
        const languageCookie = cookies.find((cookie) => cookie.name === 'language');
        expect(languageCookie?.value).toBe('de');
    });
});
