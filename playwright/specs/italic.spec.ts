import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

test.describe('italic some text', function () {

    test('makes text italic using button', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)

        // Write some text
        await writeToPad(page, 'Foo')
        await page.waitForTimeout(200);

        // Select all text
        await page.keyboard.down('Control');
        await page.keyboard.press('a');
        await page.keyboard.up('Control');
        await page.waitForTimeout(100);

        // Click the italic button
        const $italicButton = page.locator('.buttonicon-italic');
        await $italicButton.click();
        await page.waitForTimeout(300);

        // Check for italic element
        const $firstTextElement = padBody.locator('div').first();
        await expect($firstTextElement.locator('i')).toHaveCount(1, { timeout: 10000 });

        // Verify text is still there
        await expect($firstTextElement).toHaveText('Foo');
    });

    test('makes text italic using keypress', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)

        // Write some text
        await writeToPad(page, 'Foo')
        await page.waitForTimeout(200);

        // Select all text
        await page.keyboard.down('Control');
        await page.keyboard.press('a');
        await page.keyboard.up('Control');
        await page.waitForTimeout(100);

        // Press Ctrl+I
        await page.keyboard.down('Control');
        await page.keyboard.press('i');
        await page.keyboard.up('Control');
        await page.waitForTimeout(300);

        // Check for italic element
        const $firstTextElement = padBody.locator('div').first();
        await expect($firstTextElement.locator('i')).toHaveCount(1, { timeout: 10000 });

        // Verify text is still there
        await expect($firstTextElement).toHaveText('Foo');
    });
});
