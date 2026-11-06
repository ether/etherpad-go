'use strict';

import {expect, Page, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad} from "../helper/padHelper";

test.beforeEach(async ({page}) => {
    await goToNewPad(page);
});

// The editor no longer lives in iframes: #outerdocbody and #innerdocbody are
// divs inside #editorcontainer, which is the element that scrolls the pad.
const editorScroll = (page: Page) => page.locator('#editorcontainer').evaluate((el) => ({
    top: el.scrollTop,
    scrollHeight: el.scrollHeight,
    clientHeight: el.clientHeight,
}));

test.describe('editor height and scrolling', function () {

    test('a short pad has no scrollbar', async function ({page}) {
        const padBody = await getPadBody(page);
        await padBody.click();
        await clearPadContent(page);

        const {scrollHeight, clientHeight} = await editorScroll(page);
        expect(scrollHeight).toBeLessThanOrEqual(clientHeight + 1);
    });

    test('a long pad can be scrolled with the mouse wheel', async function ({page}) {
        const padBody = await getPadBody(page);
        await padBody.click();
        await clearPadContent(page);
        // Type line by line like a user: insertText() with "\n" only creates
        // new lines in Chromium.
        const lines = 80;
        for (let i = 0; i < lines; i++) {
            if (i > 0) await page.keyboard.press('Enter');
            await page.keyboard.insertText(`Line ${i}`);
        }
        await expect(padBody.locator('div').nth(lines - 1)).toHaveText(`Line ${lines - 1}`);

        await page.locator('#editorcontainer').evaluate((el) => { el.scrollTop = 0; });
        const before = await editorScroll(page);
        expect(before.scrollHeight).toBeGreaterThan(before.clientHeight);

        const box = (await page.locator('#editorcontainer').boundingBox())!;
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.mouse.wheel(0, 1500);
        await expect.poll(async () => (await editorScroll(page)).top).toBeGreaterThan(0);

        // The last line must be reachable.
        await page.locator('#editorcontainer').evaluate((el) => { el.scrollTop = el.scrollHeight; });
        await expect(padBody.locator('div').nth(lines - 1)).toBeInViewport();
    });
});
