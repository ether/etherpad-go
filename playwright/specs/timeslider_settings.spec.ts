import {expect, Page, test} from "@playwright/test";
import {goToNewPad, writeToPad} from "../helper/padHelper";
import {gotoTimeslider} from "../helper/timeslider";

const timesliderSettingsButton = 'li[data-key="settings"] button.buttonicon-settings';

const ensureTimesliderSettingsVisible = async (page: Page) => {
    const settings = page.locator('#settings');
    await page.waitForSelector('#timeslider-wrapper');
    for (let i = 0; i < 3; i++) {
        const classes = await settings.getAttribute('class');
        if (classes?.includes('popup-show')) return;
        await page.locator(timesliderSettingsButton).click();
        await page.waitForTimeout(150);
    }
    await expect(settings).toHaveClass(/popup-show/);
};

const getTimesliderBodyFont = async (page: Page) => {
    return await page.locator('#innerdocbody').evaluate((el) => {
        return window.getComputedStyle(el).getPropertyValue('font-family').toLowerCase();
    });
};

test.beforeEach(async ({page}) => {
    await goToNewPad(page);
    await writeToPad(page, 'Timeslider settings test');
    await gotoTimeslider(page, 0);
});

test.describe('timeslider settings', () => {
    test('toggles settings popup from toolbar button', async ({page}) => {
        const settings = page.locator('#settings');
        await expect(settings).not.toHaveClass(/popup-show/);

        await page.locator(timesliderSettingsButton).click();
        await expect(settings).toHaveClass(/popup-show/);

        await page.locator(timesliderSettingsButton).click();
        await expect(settings).not.toHaveClass(/popup-show/);
    });

    test('applies selected font to timeslider content', async ({page}) => {
        await ensureTimesliderSettingsVisible(page);
        const fontMenu = page.locator('#viewfontmenu');

        await page.evaluate(() => {
            const menu = document.getElementById('viewfontmenu') as HTMLSelectElement | null;
            if (!menu) return;
            const value = 'RobotoMono';
            if (!Array.from(menu.options).some((opt) => opt.value === value)) {
                const option = document.createElement('option');
                option.value = value;
                option.text = value;
                menu.add(option);
            }
        });

        await fontMenu.selectOption('RobotoMono');
        await expect(fontMenu).toHaveValue('RobotoMono');
        await expect.poll(async () => await getTimesliderBodyFont(page)).toContain('robotomono');
    });

    test('resets font to normal option', async ({page}) => {
        await ensureTimesliderSettingsVisible(page);
        const fontMenu = page.locator('#viewfontmenu');

        await page.evaluate(() => {
            const innerDocBody = document.getElementById('innerdocbody') as HTMLElement | null;
            if (innerDocBody) innerDocBody.style.fontFamily = 'RobotoMono';
        });

        await expect.poll(async () => await getTimesliderBodyFont(page)).toContain('robotomono');

        await fontMenu.selectOption('');
        await expect(fontMenu).toHaveValue('');
        await expect.poll(async () => await getTimesliderBodyFont(page)).not.toContain('robotomono');
    });
});
