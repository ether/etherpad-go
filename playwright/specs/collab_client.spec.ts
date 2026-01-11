import {clearPadContent, getPadBody, goToNewPad, goToPad, writeToPad} from "../helper/padHelper";
import {expect, Page, test} from "@playwright/test";

let padId = "";

test.beforeEach(async ({ page })=>{
    // create a new pad before each test run
    padId = await goToNewPad(page);
    const body = await getPadBody(page);
    await body.click();
    await clearPadContent(page);
    await writeToPad(page, "Hello World");
    await page.keyboard.press('Enter');
    await writeToPad(page, "Hello World");
    await page.keyboard.press('Enter');
    await writeToPad(page, "Hello World");
    await page.keyboard.press('Enter');
    await writeToPad(page, "Hello World");
    await page.keyboard.press('Enter');
    await writeToPad(page, "Hello World");
    await page.keyboard.press('Enter');
    // Wait for changes to sync
    await page.waitForTimeout(500);
})

test.describe('Messages in the COLLABROOM', function () {
    const user1Text = 'text created by user 1';
    const user2Text = 'text created by user 2';

    const replaceLineText = async (lineNumber: number, newText: string, page: Page) => {
        const body = await getPadBody(page)

        const div = body.locator('div').nth(lineNumber)

        // Wait for the span to be available
        const span = div.locator('span');
        await span.waitFor({ state: 'visible', timeout: 10000 });

        // simulate key presses to delete content
        await span.selectText() // select all
        await page.keyboard.press('Backspace') // clear the first line
        await page.keyboard.type(newText, { delay: 10 }) // insert the string
        // Wait for changes to propagate
        await page.waitForTimeout(200);
    };

    test('bug #4978 regression test', async function ({browser}) {
        // Increase timeout for this complex multi-user test
        test.setTimeout(60000);

        // The bug was triggered by receiving a change from another user while simultaneously composing
        // a character and waiting for an acknowledgement of a previously sent change.

        // User 1
        const context1 = await browser.newContext();
        const page1 = await context1.newPage();
        await goToPad(page1, padId)
        const body1 = await getPadBody(page1)
        // Perform actions as User 1...

        // User 2 - Fix: use page2 instead of page1
        const context2 = await browser.newContext();
        const page2 = await context2.newPage();
        await goToPad(page2, padId)
        const body2 = await getPadBody(page2)

        await replaceLineText(0, user1Text,page1);

        // Wait for sync
        await expect(body2.locator('div').nth(0)).toHaveText(user1Text, { timeout: 10000 });

        // User 1 starts a character composition.
        await replaceLineText(1, user2Text, page2)

        await expect(body1.locator('div').nth(1)).toHaveText(user2Text, { timeout: 10000 })

        // Users 1 and 2 make some more changes.
        await replaceLineText(3, user2Text, page2);

        await expect(body1.locator('div').nth(3)).toHaveText(user2Text, { timeout: 10000 })

        await replaceLineText(2, user1Text, page1);
        await expect(body2.locator('div').nth(2)).toHaveText(user1Text, { timeout: 10000 })

        // All changes should appear in both views.
        const expectedLines = [
            user1Text,
            user2Text,
            user1Text,
            user2Text,
        ];

        for (let i=0;i<expectedLines.length;i++){
            await expect(body1.locator('div').nth(i)).toHaveText(expectedLines[i]);
        }

        for (let i=0;i<expectedLines.length;i++){
            await expect(body2.locator('div').nth(i)).toHaveText(expectedLines[i]);
        }

        // Cleanup
        await context1.close();
        await context2.close();
    });
});
