import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

// Skip on WebKit - the dropdown doesn't work reliably on Safari
test.describe('font select', function () {
    test.skip(({ browserName }) => browserName === 'webkit', 'Skipping on WebKit due to dropdown issues');

    test('makes text RobotoMono', async function ({page}) {
        await showSettings(page);

        // Find and click the font dropdown
        const dropdown = page.locator('.dropdowns-container .dropdown-line .current').first();
        await dropdown.click();

        // Select RobotoMono
        await page.locator('li:text("RobotoMono")').click();

        // Check if font changed to RobotoMono
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        await expect.poll(async () => {
            const fontFamily = await body.evaluate((e) => {
                return window.getComputedStyle(e).getPropertyValue("font-family").toLowerCase();
            });
            return fontFamily;
        }).toContain('robotomono');
    });
});
