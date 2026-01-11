import {goToNewPad, goToPad} from "../helper/padHelper";
import {expect, Page, test} from "@playwright/test";

test.describe('Messages in the COLLABROOM', function () {
    const user1Text = 'text created by user 1';
    const user2Text = 'text created by user 2';

    const replaceLineText = async (lineNumber: number, newText: string, page: Page) => {
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');

        const div = innerFrame.locator('#innerdocbody div').nth(lineNumber);
        await div.click();
        await page.keyboard.press('Home');
        await page.keyboard.down('Shift');
        await page.keyboard.press('End');
        await page.keyboard.up('Shift');
        await page.keyboard.type(newText);
    };

    test('bug #4978 regression test', async function ({browser}) {
        test.setTimeout(60000);

        // User 1 creates the pad
        const context1 = await browser.newContext();
        const page1 = await context1.newPage();
        const padId = await goToNewPad(page1);

        // Set up content with User 1
        const innerFrame1 = page1.frame('ace_inner');
        if (!innerFrame1) throw new Error('Could not find ace_inner frame');

        await innerFrame1.locator('#innerdocbody').click();
        await page1.keyboard.down('Control');
        await page1.keyboard.press('a');
        await page1.keyboard.up('Control');
        await page1.keyboard.type("Line 0\nLine 1\nLine 2\nLine 3\nLine 4");

        // User 2 joins the same pad
        const context2 = await browser.newContext();
        const page2 = await context2.newPage();
        await goToPad(page2, padId);

        const innerFrame2 = page2.frame('ace_inner');
        if (!innerFrame2) throw new Error('Could not find ace_inner frame');
        const body2 = innerFrame2.locator('#innerdocbody');

        // Wait for User 2 to see the content
        await expect(body2.locator('div').first()).toContainText('Line', { timeout: 10000 });

        // User 1 makes a change
        await replaceLineText(0, user1Text, page1);
        await expect(body2.locator('div').nth(0)).toHaveText(user1Text, { timeout: 10000 });

        // User 2 makes a change
        const body1 = innerFrame1.locator('#innerdocbody');
        await replaceLineText(1, user2Text, page2);
        await expect(body1.locator('div').nth(1)).toHaveText(user2Text, { timeout: 10000 });

        // More changes
        await replaceLineText(3, user2Text, page2);
        await expect(body1.locator('div').nth(3)).toHaveText(user2Text, { timeout: 10000 });

        await replaceLineText(2, user1Text, page1);
        await expect(body2.locator('div').nth(2)).toHaveText(user1Text, { timeout: 10000 });

        // Cleanup
        await context1.close();
        await context2.close();
    });
});
