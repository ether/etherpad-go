import {expect, Page, test} from "@playwright/test";
import {goToNewPad, setEpCheckbox} from "../helper/padHelper";

const settingsButton = "button[class~='buttonicon-settings']";

const ensureSettingsVisible = async (page: Page) => {
    const settings = page.locator('#settings');
    await page.locator('#innerdocbody').waitFor({ state: 'visible' });
    for (let i = 0; i < 3; i++) {
        const classes = await settings.getAttribute('class');
        if (classes?.includes('popup-show')) return;
        await page.locator(settingsButton).click();
        await page.waitForTimeout(150);
    }
    await expect(settings).toHaveClass(/popup-show/);
};

test.beforeEach(async ({page}) => {
    await goToNewPad(page);
});

test.describe('settings popup and options', () => {
    test('toggles settings popup from toolbar button', async ({page}) => {
        const settings = page.locator('#settings');
        await expect(settings).not.toHaveClass(/popup-show/);

        await page.locator(settingsButton).click();
        await expect(settings).toHaveClass(/popup-show/);

        await page.locator(settingsButton).click();
        await expect(settings).not.toHaveClass(/popup-show/);
    });

    test('toggles line numbers visibility in editor gutter', async ({page}) => {
        await ensureSettingsVisible(page);
        const lineNumbersCheckbox = page.locator('#options-linenoscheck');

        await setEpCheckbox(lineNumbersCheckbox, false);
        await expect.poll(async () => {
            return await page.locator('#sidediv').evaluate((node) =>
                node.parentElement?.classList.contains('line-numbers-hidden') ?? false);
        }).toBe(true);

        await setEpCheckbox(lineNumbersCheckbox, true);
        await expect.poll(async () => {
            return await page.locator('#sidediv').evaluate((node) =>
                node.parentElement?.classList.contains('line-numbers-hidden') ?? false);
        }).toBe(false);
    });

    test('toggles authorship color class', async ({page}) => {
        await ensureSettingsVisible(page);
        const colorsCheckbox = page.locator('#options-colorscheck');
        const chatText = page.locator('#chattext');

        await setEpCheckbox(colorsCheckbox, false);
        await expect(chatText).not.toHaveClass(/authorColors/);

        await setEpCheckbox(colorsCheckbox, true);
        await expect(chatText).toHaveClass(/authorColors/);
    });

    test('rtl checkbox updates document direction', async ({page}) => {
        await ensureSettingsVisible(page);
        const rtlCheckbox = page.locator('#options-rtlcheck');

        await setEpCheckbox(rtlCheckbox, true);
        await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');

        await setEpCheckbox(rtlCheckbox, false);
        await expect(page.locator('html')).toHaveAttribute('dir', 'ltr');
    });
});
