import {expect, Page, test} from "@playwright/test";
import {goToNewPad, goToPad, isEpCheckboxChecked, setEpCheckbox} from "../helper/padHelper";

// Pad-wide settings (etherpad-lite #7586, #7698): the pad's creator sets
// defaults for everyone, live. Others may override them with their own view
// unless the creator enforces them.

const settingsButton = "button[class~='buttonicon-settings']";

const openSettings = async (page: Page) => {
    const settings = page.locator('#settings');
    for (let i = 0; i < 3; i++) {
        if ((await settings.getAttribute('class'))?.includes('popup-show')) return;
        await page.locator(settingsButton).click();
        await page.waitForTimeout(150);
    }
    await expect(settings).toHaveClass(/popup-show/);
};

const lineNumbersShown = (page: Page) =>
    page.locator('#sidediv').evaluate((node) => !node.parentElement?.classList.contains('line-numbers-hidden'));

test('the creator sets, and enforces, settings for everyone', async ({page, browser}) => {
    const padId = await goToNewPad(page);
    await openSettings(page);
    await expect(page.locator('#pad-settings-section')).toBeVisible();

    const other = await browser.newContext();
    try {
        const guest = await other.newPage();
        await goToPad(guest, padId);
        await openSettings(guest);
        // Only the creator gets the pad-wide controls.
        await expect(guest.locator('#pad-settings-section')).toBeHidden();
        await expect.poll(() => lineNumbersShown(guest)).toBe(true);

        // A pad-wide change reaches the guest live.
        await setEpCheckbox(page.locator('#padsettings-options-linenoscheck'), false);
        await expect.poll(() => lineNumbersShown(guest)).toBe(false);
        expect(await isEpCheckboxChecked(guest.locator('#options-linenoscheck'))).toBe(false);

        // Not enforced: the guest's own view wins.
        await setEpCheckbox(guest.locator('#options-linenoscheck'), true);
        await expect.poll(() => lineNumbersShown(guest)).toBe(true);

        // Enforced: the pad-wide value applies and the guest's controls lock.
        await setEpCheckbox(page.locator('#padsettings-enforcecheck'), true);
        await expect(guest.locator('#enforce-settings-notice')).toBeVisible();
        await expect.poll(() => lineNumbersShown(guest)).toBe(false);
        await expect(guest.locator('#options-linenoscheck')).toHaveAttribute('disabled', '');

        // Chat switched off for everyone.
        await setEpCheckbox(page.locator('#padsettings-options-disablechat'), true);
        await expect(guest.locator('#chaticon')).toBeHidden();

        // The settings are stored with the pad: a reload keeps them.
        await guest.reload();
        await guest.locator('#editorcontainer.initialized').waitFor();
        await openSettings(guest);
        await expect(guest.locator('#enforce-settings-notice')).toBeVisible();
        await expect.poll(() => lineNumbersShown(guest)).toBe(false);
        await expect(guest.locator('#chaticon')).toBeHidden();

        // Enforcement off again: the guest's own choice is back.
        await setEpCheckbox(page.locator('#padsettings-enforcecheck'), false);
        await expect(guest.locator('#enforce-settings-notice')).toBeHidden();
        await expect.poll(() => lineNumbersShown(guest)).toBe(true);
        await expect(guest.locator('#options-linenoscheck')).not.toHaveAttribute('disabled', '');

        // The creator is never enforced upon.
        await expect(page.locator('#enforce-settings-notice')).toBeHidden();
    } finally {
        await other.close();
    }
});
