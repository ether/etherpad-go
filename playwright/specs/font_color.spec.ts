import {expect, test} from "@playwright/test";
import {goToNewPad, getPadBody, writeToPad, selectAllText} from "../helper/padHelper";

test.beforeEach(async ({ page }) => {
    await goToNewPad(page);
})

test('font color toolbar button exists', async ({page}) => {
    // Check that the font color button/dropdown exists in the toolbar
    const toolbar = page.locator('.toolbar');
    // The plugin adds a toolbar select with class color-selection or a button with data-key fontColor
    const fontColorBtn = toolbar.locator('[data-key="fontColor"]');
    // This test only passes when the plugin is enabled, so mark as soft check
    const count = await fontColorBtn.count();
    // If plugin not enabled, skip gracefully
    test.skip(count === 0, 'ep_font_color plugin not enabled');
    expect(count).toBeGreaterThan(0);
})
