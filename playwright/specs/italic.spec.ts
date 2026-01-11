import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

test.describe('italic some text', function () {

    test('makes text italic using button', async function ({page}) {
        await clearPadContent(page)

        // Write some text
        await writeToPad(page, 'Foo')
        await page.waitForTimeout(300);

        // Get the inner frame directly
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        // Triple-click to select the line
        await body.locator('div').first().click({ clickCount: 3 });
        await page.waitForTimeout(100);

        // Click the italic button
        const $italicButton = page.locator('.buttonicon-italic');
        await $italicButton.click();
        await page.waitForTimeout(500);

        // Check for italic element - may be one or more <i> tags
        const italicCount = await body.locator('i').count();
        expect(italicCount).toBeGreaterThanOrEqual(1);

        // Verify text is still there
        await expect(body.locator('div').first()).toContainText('Foo');
    });

    test('makes text italic using keypress', async function ({page}) {
        await clearPadContent(page)

        // Write some text
        await writeToPad(page, 'Foo')
        await page.waitForTimeout(300);

        // Get the inner frame directly
        const innerFrame = page.frame('ace_inner');
        if (!innerFrame) throw new Error('Could not find ace_inner frame');
        const body = innerFrame.locator('#innerdocbody');

        // Triple-click to select the line
        await body.locator('div').first().click({ clickCount: 3 });
        await page.waitForTimeout(100);

        // Press Ctrl+I
        await page.keyboard.down('Control');
        await page.keyboard.press('i');
        await page.keyboard.up('Control');
        await page.waitForTimeout(500);

        // Check for italic element - may be one or more <i> tags
        const italicCount = await body.locator('i').count();
        expect(italicCount).toBeGreaterThanOrEqual(1);

        // Verify text is still there
        await expect(body.locator('div').first()).toContainText('Foo');
    });
});
