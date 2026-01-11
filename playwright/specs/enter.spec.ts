'use strict';
import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

test.describe('enter keystroke', function () {

    test('creates a new line & puts cursor onto a new line', async function ({page}) {
        // Get the inner frame directly
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        // Get the first text element
        const firstTextElement = body.locator('div').first();

        // Get the original string value
        const originalTextValue = await firstTextElement.textContent();

        // Click at the beginning and press Enter
        await firstTextElement.click();
        await page.waitForTimeout(100);
        await page.keyboard.press('Home');
        await page.waitForTimeout(50);
        await page.keyboard.press('Enter');
        await page.waitForTimeout(300);

        // Check that first line is now empty
        const updatedFirstElement = body.locator('div').first();
        expect(await updatedFirstElement.textContent()).toBe('');

        // Check that second line has the original content
        const newSecondLine = body.locator('div').nth(1);
        expect(await newSecondLine.textContent()).toBe(originalTextValue);
    });

    test('enter is always visible after event', async function ({page}) {
        // Get the inner frame directly
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        const originalLength = await body.locator('div').count();

        // Simulate key presses to enter content
        const numberOfLines = 15;
        for (let i = 0; i < numberOfLines; i++) {
            const lastLine = body.locator('div').last();
            await lastLine.click();
            await page.keyboard.press('End');
            await page.keyboard.press('Enter');
            await page.waitForTimeout(100);
        }

        // Check that we have the expected number of lines
        const newCount = await body.locator('div').count();
        expect(newCount).toBe(numberOfLines + originalLength);
    });
});
