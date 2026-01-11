import {expect, test} from "@playwright/test";
import {getPadBody, goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({ page })=>{
    // create a new pad before each test run
    await goToNewPad(page);
})


test.describe('font select', function () {

    test('makes text RobotoMono', async function ({page}) {
        // click on the settings button to make settings visible
        await showSettings(page);
        await page.waitForTimeout(300);

        // Find and click the font dropdown
        const dropdown = page.locator('.dropdowns-container .dropdown-line .current').nth(0);
        await dropdown.waitFor({ state: 'visible', timeout: 10000 });
        await dropdown.click();
        await page.waitForTimeout(200);

        // Select RobotoMono
        const robotoOption = page.locator('li:text("RobotoMono")');
        await robotoOption.waitFor({ state: 'visible', timeout: 5000 });
        await robotoOption.click();
        await page.waitForTimeout(500);

        // Check if font changed to RobotoMono
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        const fontFamily = await body.evaluate((e) => {
            return window.getComputedStyle(e).getPropertyValue("font-family").toLowerCase();
        });

        expect(fontFamily).toContain('robotomono');
    });
});
