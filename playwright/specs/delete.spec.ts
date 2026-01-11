import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    // create a new pad before each test run
    await goToNewPad(page);
})


test('delete keystroke', async ({page}) => {
    const padText = "Hello World this is a test"
    const body = await getPadBody(page)
    await clearPadContent(page)
    await writeToPad(page, padText)
    // Navigate to the end of the text
    await page.keyboard.press('End');
    // Delete the last character
    await page.keyboard.press('Backspace');
    // Wait for change to be applied
    await page.waitForTimeout(200);
    const text = await body.locator('div').first().innerText();
    expect(text).toBe(padText.slice(0, -1));
})
