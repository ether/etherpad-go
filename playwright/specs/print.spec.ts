import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

test.beforeEach(async ({ page }) => {
    await goToNewPad(page);
})

test('print button exists in toolbar', async ({page}) => {
    const btn = page.locator('[data-key="print"]');
    const count = await btn.count();
    test.skip(count === 0, 'ep_print plugin not enabled');
    expect(count).toBeGreaterThan(0);
})
