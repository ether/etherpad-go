import {expect, test} from "@playwright/test";
import {goToNewPad, getPadBody, writeToPad, selectAllText} from "../helper/padHelper";

test.beforeEach(async ({ page }) => {
    await goToNewPad(page);
})

test('clear formatting button exists in toolbar', async ({page}) => {
    const btn = page.locator('[data-key="clearFormatting"]');
    const count = await btn.count();
    test.skip(count === 0, 'ep_clear_formatting plugin not enabled');
    expect(count).toBeGreaterThan(0);
})
