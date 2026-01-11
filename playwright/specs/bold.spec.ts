import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, selectAllText, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

test.describe('bold button', ()=>{

    test('makes text bold on click', async ({page}) => {
        await clearPadContent(page);
        await writeToPad(page, "Hi Etherpad");
        await page.waitForTimeout(300);

        // Get the inner frame directly
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        // Triple-click to select the line
        await body.locator('div').first().click({ clickCount: 3 });
        await page.waitForTimeout(100);

        // Click the bold button
        await page.locator("button[class~='buttonicon-bold']").click();
        await page.waitForTimeout(500);

        // Check if there are bold elements
        const boldCount = await body.locator('b').count();
        expect(boldCount).toBeGreaterThanOrEqual(1);

        // Verify text is still there
        await expect(body.locator('div').first()).toContainText('Hi Etherpad');
    })

    test('makes text bold on keypress', async ({page}) => {
        await clearPadContent(page);
        await writeToPad(page, "Hi Etherpad");
        await page.waitForTimeout(300);

        // Get the inner frame directly
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        // Triple-click to select the line
        await body.locator('div').first().click({ clickCount: 3 });
        await page.waitForTimeout(100);

        // Press CTRL + B
        await page.keyboard.down('Control');
        await page.keyboard.press('b');
        await page.keyboard.up('Control');
        await page.waitForTimeout(500);

        // Check if there are bold elements
        const boldCount = await body.locator('b').count();
        expect(boldCount).toBeGreaterThanOrEqual(1);

        // Verify text is still there
        await expect(body.locator('div').first()).toContainText('Hi Etherpad');
    })

})
