import {expect, Page, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

// Skip on WebKit - the dropdown doesn't work reliably on Safari
test.describe('font select', function () {
    test.skip(({ browserName }) => browserName === 'webkit', 'Skipping on WebKit due to dropdown issues');

    const getBodyFontFamily = async (page: Page) => {
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');
        return await body.evaluate((e) => {
            return window.getComputedStyle(e).getPropertyValue("font-family").toLowerCase();
        });
    };

    test('makes text RobotoMono', async function ({page}) {
        await showSettings(page);
        const fontMenu = page.locator('#viewfontmenu');
        await fontMenu.selectOption('RobotoMono');
        await expect(fontMenu).toHaveValue('RobotoMono');

        // Check if font changed to RobotoMono
        await expect.poll(async () => {
            return await getBodyFontFamily(page);
        }).toContain('robotomono');
    });

    test('resets to normal font type', async function ({page}) {
        await showSettings(page);
        const fontMenu = page.locator('#viewfontmenu');
        await fontMenu.selectOption('RobotoMono');
        await expect(fontMenu).toHaveValue('RobotoMono');

        await fontMenu.selectOption('');
        await expect(fontMenu).toHaveValue('');

        await expect.poll(async () => {
            return await getBodyFontFamily(page);
        }).not.toContain('robotomono');
    });
});
