import {clearPadContent, getPadBody, goToNewPad, goToPad, writeToPad} from "../helper/padHelper";
import {expect, Page, test} from "@playwright/test";

test.describe('Messages in the COLLABROOM', function () {
    const user1Text = 'text created by user 1';
    const user2Text = 'text created by user 2';

    const replaceLineText = async (lineNumber: number, newText: string, page: Page) => {
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) {
            throw new Error('Could not find ace_inner frame');
        }
        const body = innerFrame.locator('#innerdocbody');
        const div = body.locator('div').nth(lineNumber);

        // Click at the beginning of the line
        await div.click();
        await page.waitForTimeout(50);

        // Go to the beginning of the line
        await page.keyboard.press('Home');
        await page.waitForTimeout(50);

        // Select to the end of the line
        await page.keyboard.down('Shift');
        await page.keyboard.press('End');
        await page.keyboard.up('Shift');
        await page.waitForTimeout(50);

        // Delete the selected text and type the new text
        await page.keyboard.type(newText, { delay: 10 });

        // Wait for changes to propagate
        await page.waitForTimeout(300);
    };

    test('bug #4978 regression test', async function ({browser}) {
        // Increase timeout for this complex multi-user test
        test.setTimeout(120000);

        // User 1 creates the pad
        const context1 = await browser.newContext();
        const page1 = await context1.newPage();
        const padId = await goToNewPad(page1);

        // Set up content with User 1 - use direct typing for reliability
        const innerFrame1 = page1.frame('ace_inner');
        if (!innerFrame1) throw new Error('Could not find ace_inner frame');
        const body1Setup = innerFrame1.locator('#innerdocbody');

        // Clear and set up lines
        await body1Setup.click();
        await page1.keyboard.down('Control');
        await page1.keyboard.press('a');
        await page1.keyboard.up('Control');
        await page1.keyboard.press('Backspace');
        await page1.waitForTimeout(200);

        // Create 5 lines
        await page1.keyboard.type("Line 0", { delay: 10 });
        await page1.keyboard.press('Enter');
        await page1.keyboard.type("Line 1", { delay: 10 });
        await page1.keyboard.press('Enter');
        await page1.keyboard.type("Line 2", { delay: 10 });
        await page1.keyboard.press('Enter');
        await page1.keyboard.type("Line 3", { delay: 10 });
        await page1.keyboard.press('Enter');
        await page1.keyboard.type("Line 4", { delay: 10 });

        await page1.waitForTimeout(1000);

        // Verify we have at least 5 divs (may have more due to empty lines)
        const body1 = innerFrame1.locator('#innerdocbody');
        const divCount1 = await body1.locator('div').count();
        expect(divCount1).toBeGreaterThanOrEqual(5);

        // User 2 joins the same pad
        const context2 = await browser.newContext();
        const page2 = await context2.newPage();
        await goToPad(page2, padId);
        await page2.waitForTimeout(1000);

        const innerFrame2 = page2.frame('ace_inner');
        if (!innerFrame2) throw new Error('Could not find ace_inner frame');
        const body2 = innerFrame2.locator('#innerdocbody');

        // Verify User 2 sees the same content
        const divCount2 = await body2.locator('div').count();
        expect(divCount2).toBeGreaterThanOrEqual(5);

        // User 1 makes a change
        await replaceLineText(0, user1Text, page1);

        // Wait for sync
        await expect(body2.locator('div').nth(0)).toHaveText(user1Text, { timeout: 15000 });

        // User 2 makes a change
        await replaceLineText(1, user2Text, page2);
        await expect(body1.locator('div').nth(1)).toHaveText(user2Text, { timeout: 15000 });

        // More changes
        await replaceLineText(3, user2Text, page2);
        await expect(body1.locator('div').nth(3)).toHaveText(user2Text, { timeout: 15000 });

        await replaceLineText(2, user1Text, page1);
        await expect(body2.locator('div').nth(2)).toHaveText(user1Text, { timeout: 15000 });

        // All changes should appear in both views.
        const expectedLines = [
            user1Text,
            user2Text,
            user1Text,
            user2Text,
        ];

        for (let i = 0; i < expectedLines.length; i++) {
            await expect(body1.locator('div').nth(i)).toHaveText(expectedLines[i]);
        }

        for (let i = 0; i < expectedLines.length; i++) {
            await expect(body2.locator('div').nth(i)).toHaveText(expectedLines[i]);
        }

        // Cleanup
        await context1.close();
        await context2.close();
    });
});
