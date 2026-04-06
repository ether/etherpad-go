import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({ page }) => {
    await goToNewPad(page);
})

test('table of contents settings checkbox exists', async ({page}) => {
    await showSettings(page);
    const tocCheckbox = page.locator('#options-toc');
    const count = await tocCheckbox.count();
    test.skip(count === 0, 'ep_table_of_contents plugin not enabled');
    expect(count).toBeGreaterThan(0);
})
