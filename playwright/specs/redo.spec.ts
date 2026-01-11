import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})


test.describe('undo button then redo button', function () {


    test('redo some typing with button', async function ({page}) {
        const padBody = await getPadBody(page);
        const newString = 'Foo';

        await clearPadContent(page)
        await writeToPad(page, newString);
        await page.waitForTimeout(200);

        // Verify text was written
        const firstDiv = padBody.locator('div').first();
        await expect(firstDiv).toHaveText(newString);

        // Undo
        await page.locator('.buttonicon-undo').click()
        await page.waitForTimeout(300);

        // Redo
        await page.locator('.buttonicon-redo').click()
        await page.waitForTimeout(300);

        // Check that text is back
        await expect(firstDiv).toHaveText(newString);
    });

    test('redo some typing with keypress', async function ({page}) {
        const padBody = await getPadBody(page);
        const newString = 'Foo';

        await clearPadContent(page)
        await writeToPad(page, newString);
        await page.waitForTimeout(200);

        // Verify text was written
        const firstDiv = padBody.locator('div').first();
        await expect(firstDiv).toHaveText(newString);

        // Undo the change
        await page.keyboard.down('Control');
        await page.keyboard.press('z');
        await page.keyboard.up('Control');
        await page.waitForTimeout(300);

        // Redo the change
        await page.keyboard.down('Control');
        await page.keyboard.press('y');
        await page.keyboard.up('Control');
        await page.waitForTimeout(300);

        // Check that text is back
        await expect(firstDiv).toHaveText(newString);
    });
});
