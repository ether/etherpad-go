'use strict';
import {expect, test} from "@playwright/test";
import {clearPadContent, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

test.describe('enter keystroke', function () {

    test('creates a new line & puts cursor onto a new line', async function ({page}) {
        // Clear pad and write test content
        await clearPadContent(page);
        await writeToPad(page, 'Test Line');

        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        // Verify we have one line with content
        await expect(body.locator('div').first()).toHaveText('Test Line');

        // Click at the end and press Enter
        await body.locator('div').first().click();
        await page.keyboard.press('End');
        await page.keyboard.press('Enter');

        // Check that we now have 2 lines
        await expect(body.locator('div')).toHaveCount(2);

        // First line should still have the text
        await expect(body.locator('div').first()).toHaveText('Test Line');

        // Second line should be empty
        await expect(body.locator('div').nth(1)).toHaveText('');
    });

    test('enter is always visible after event', async function ({page}) {
        await clearPadContent(page);

        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        // Start with 1 line
        await expect(body.locator('div')).toHaveCount(1);

        // Add lines by pressing Enter
        const numberOfLines = 15;
        for (let i = 0; i < numberOfLines; i++) {
            await body.locator('div').last().click();
            await page.keyboard.press('End');
            await page.keyboard.press('Enter');
        }

        // Check that we have the expected number of lines
        await expect(body.locator('div')).toHaveCount(numberOfLines + 1);
    });
});
