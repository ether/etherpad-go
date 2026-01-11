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

        // get the first text element inside the editable space
        const firstTextElement = padBody.locator('div').first()
        const originalValue = await firstTextElement.textContent(); // get the original value (should be empty)

        await writeToPad(page, 'foo'); // send line 1 to the pad
        await page.waitForTimeout(200);

        const modifiedValue = await firstTextElement.textContent(); // get the modified value
        expect(modifiedValue).toBe('foo'); // expect the value to be 'foo'

        // get clear authorship button as a variable
        const undoButton = page.locator('.buttonicon-undo')
        await undoButton.click() // click the button
        await page.waitForTimeout(500);

        await expect(firstTextElement).toHaveText(originalValue || '');
    });

    test('undo some typing using a keypress', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)

        // get the first text element inside the editable space
        const firstTextElement = padBody.locator('div').first()
        const originalValue = await firstTextElement.textContent(); // get the original value

        await writeToPad(page, 'foo'); // send line 1 to the pad
        await page.waitForTimeout(200);

        const modifiedValue = await firstTextElement.textContent(); // get the modified value
        expect(modifiedValue).toBe('foo'); // expect the value to be 'foo'

        // undo the change
        await page.keyboard.down('Control');
        await page.keyboard.press('z');
        await page.keyboard.up('Control');
        await page.waitForTimeout(500);

        await expect(firstTextElement).toHaveText(originalValue || '');
    });
});
