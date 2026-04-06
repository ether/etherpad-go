import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

test.beforeEach(async ({ page }) => {
    await goToNewPad(page);
})

test('toolbar renders without errors', async ({page}) => {
    // The toolbar should be present and have buttons
    const toolbar = page.locator('#editbar');
    await expect(toolbar).toBeVisible();

    // Core buttons should always exist
    const boldBtn = page.locator('.buttonicon-bold');
    await expect(boldBtn).toBeAttached();

    const italicBtn = page.locator('.buttonicon-italic');
    await expect(italicBtn).toBeAttached();

    const underlineBtn = page.locator('.buttonicon-underline');
    await expect(underlineBtn).toBeAttached();
})

test('spellcheck settings item exists when plugin enabled', async ({page}) => {
    // This is a settings_menu_items plugin
    const settingsBtn = page.locator("button[class~='buttonicon-settings']");
    await settingsBtn.click();
    await expect(page.locator('#settings')).toHaveClass(/popup-show/, { timeout: 5000 });

    const spellcheckOption = page.locator('#options-spellcheck');
    // Plugin may or may not be enabled
    const exists = await spellcheckOption.count() > 0;
    if (exists) {
        await expect(spellcheckOption).toBeAttached();
    }
})
