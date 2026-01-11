'use strict';

import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})


test.describe('undo button', function () {

    test('undo some typing by clicking undo button', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)

        const firstTextElement = padBody.locator('div').first()

        await writeToPad(page, 'foo');
        await expect(firstTextElement).toHaveText('foo');

        const undoButton = page.locator('.buttonicon-undo')
        await undoButton.click()

        await expect(firstTextElement).toHaveText('');
    });

    test('undo some typing using a keypress', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)

        const firstTextElement = padBody.locator('div').first()

        await writeToPad(page, 'foo');
        await expect(firstTextElement).toHaveText('foo');

        await page.keyboard.down('Control');
        await page.keyboard.press('z');
        await page.keyboard.up('Control');

        await expect(firstTextElement).toHaveText('');
    });
});
