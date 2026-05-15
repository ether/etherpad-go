import {expect, Page, test} from "@playwright/test";
import {goToNewPad, selectEpDropdownItem} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

// Skip on WebKit - the dropdown doesn't work reliably on Safari
test.describe('font select', function () {
    test.skip(({ browserName }) => browserName === 'webkit', 'Skipping on WebKit due to dropdown issues');

    const getBodyFontFamily = async (page: Page) => {
        const body = page.locator('#innerdocbody');
        return await body.evaluate((e) => {
            return window.getComputedStyle(e).getPropertyValue("font-family").toLowerCase();
        });
    };

    test('makes text RobotoMono', async function ({page}) {
        await showSettings(page);
        await selectEpDropdownItem(page, '#viewfontmenu', 'RobotoMono');

        // Check if font changed to RobotoMono
        await expect.poll(async () => {
            return await getBodyFontFamily(page);
        }).toContain('robotomono');
    });

    test('resets to normal font type', async function ({page}) {
        await showSettings(page);
        await selectEpDropdownItem(page, '#viewfontmenu', 'RobotoMono');
        await expect.poll(async () => {
            return await getBodyFontFamily(page);
        }).toContain('robotomono');

        await selectEpDropdownItem(page, '#viewfontmenu', '');

        await expect.poll(async () => {
            return await getBodyFontFamily(page);
        }).not.toContain('robotomono');
    });
});
