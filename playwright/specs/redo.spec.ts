import {expect, test} from "@playwright/test";
import os from "node:os";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

const undoShortcut = os.platform() === 'darwin' ? 'Meta+z' : 'Control+z';
const redoShortcut = os.platform() === 'darwin' ? 'Meta+Shift+z' : 'Control+y';

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})


test.describe('undo button then redo button', function () {

    test('redo some typing with button', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)
        await writeToPad(page, 'Foo');

        const firstDiv = padBody.locator('div').first();
        await expect(firstDiv).toHaveText('Foo');

        await page.locator('.buttonicon-undo').click()
        await expect(firstDiv).toHaveText('');

        await page.locator('.buttonicon-redo').click()
        await expect(firstDiv).toHaveText('Foo');
    });

    test('redo some typing with keypress', async function ({page}) {
        const padBody = await getPadBody(page);
        await clearPadContent(page)
        await writeToPad(page, 'Foo');

        const firstDiv = padBody.locator('div').first();
        await expect(firstDiv).toHaveText('Foo');

        await firstDiv.click();
        await page.keyboard.press(undoShortcut);
        await expect.poll(async () => (await firstDiv.textContent()) ?? '').not.toBe('Foo');

        await page.keyboard.press(redoShortcut);
        await expect(firstDiv).toHaveText('Foo');
    });
});
