import {clearPadContent, goToNewPad, goToPad, writeToPad} from "../helper/padHelper";
import {expect, Page, test} from "@playwright/test";

test.describe('Messages in the COLLABROOM', function () {

    test('bug #4978 regression test - changes sync between users', async function ({browser}) {
        test.setTimeout(90000);

        // User 1 creates the pad
        const context1 = await browser.newContext();
        const page1 = await context1.newPage();
        const padId = await goToNewPad(page1);

        // User 1 writes some text
        await clearPadContent(page1);
        await writeToPad(page1, 'Hello from User 1');

        const innerFrame1 = page1.frame('ace_inner');
        if (!innerFrame1) throw new Error('Could not find ace_inner frame');
        const body1 = innerFrame1.locator('#innerdocbody');

        // Verify User 1's content
        await expect(body1.locator('div').first()).toContainText('Hello from User 1');

        // User 2 joins the same pad
        const context2 = await browser.newContext();
        const page2 = await context2.newPage();
        await goToPad(page2, padId);

        const innerFrame2 = page2.frame('ace_inner');
        if (!innerFrame2) throw new Error('Could not find ace_inner frame');
        const body2 = innerFrame2.locator('#innerdocbody');

        // User 2 should see User 1's text
        await expect(body2.locator('div').first()).toContainText('Hello from User 1', { timeout: 15000 });

        // User 2 adds more text
        await body2.locator('div').first().click();
        await page2.keyboard.press('End');
        await page2.keyboard.type(' and User 2');

        // User 1 should see User 2's addition
        await expect(body1.locator('div').first()).toContainText('and User 2', { timeout: 15000 });

        // User 1 adds a new line
        await body1.locator('div').first().click();
        await page1.keyboard.press('End');
        await page1.keyboard.press('Enter');
        await page1.keyboard.type('New line from User 1');

        // User 2 should see the new line
        await expect(body2).toContainText('New line from User 1', { timeout: 15000 });

        // Cleanup
        await context1.close();
        await context2.close();
    });
});
