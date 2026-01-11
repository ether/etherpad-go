import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})


test.describe('undo button then redo button', function () {

    test('redo some typing with button', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)
        await writeToPad(page, 'Foo');

        const firstDiv = padBody.locator('div').first();
        await expect(firstDiv).toHaveText('Foo');

        await page.locator('.buttonicon-undo').click()
        await expect(firstDiv).toHaveText('');

        await page.locator('.buttonicon-redo').click()
        await expect(firstDiv).toHaveText('Foo');
    });

    test('redo some typing with keypress', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)
        await writeToPad(page, 'Foo');

        const firstDiv = padBody.locator('div').first();
        await expect(firstDiv).toHaveText('Foo');

        await page.keyboard.down('Control');
        await page.keyboard.press('z');
        await page.keyboard.up('Control');
        await expect(firstDiv).toHaveText('');

        await page.keyboard.down('Control');
        await page.keyboard.press('y');
        await page.keyboard.up('Control');
        await expect(firstDiv).toHaveText('Foo');
    });
});
