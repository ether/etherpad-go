import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({page}) => {
    await page.setViewportSize({width: 390, height: 844});
    await goToNewPad(page);
});

test.describe('mobile layout', () => {
    test('uses mobile toolbar layout', async ({page}) => {
        await expect(page.locator('body')).toHaveClass(/mobile-layout/);
        await expect(page.locator('.toolbar .menu_right')).toHaveCSS('position', 'fixed');
    });

    test('shows settings popup above bottom toolbar', async ({page}) => {
        await showSettings(page);
        const popup = page.locator('#settings .popup-content');
        const toolbar = page.locator('.toolbar .menu_right');

        await expect(popup).toBeVisible();

        const popupBox = await popup.boundingBox();
        const toolbarBox = await toolbar.boundingBox();
        expect(popupBox).not.toBeNull();
        expect(toolbarBox).not.toBeNull();
        if (popupBox && toolbarBox) {
            expect(popupBox.y + popupBox.height).toBeLessThanOrEqual(toolbarBox.y);
        }
    });

    test('does not overflow horizontally on tiny mobile viewport', async ({page}) => {
        await page.setViewportSize({width: 320, height: 568});
        await goToNewPad(page);
        await showSettings(page);

        const hasOverflow = await page.evaluate(() => {
            return document.documentElement.scrollWidth > window.innerWidth;
        });
        expect(hasOverflow).toBe(false);

        const popup = page.locator('#settings .popup-content');
        const popupBox = await popup.boundingBox();
        expect(popupBox).not.toBeNull();
        if (popupBox) {
            expect(popupBox.width).toBeLessThanOrEqual(320);
        }
    });

    test('keeps popup and bottom toolbar separated in landscape', async ({page}) => {
        await page.setViewportSize({width: 844, height: 390});
        await goToNewPad(page);
        await showSettings(page);

        const popup = page.locator('#settings .popup-content');
        const toolbar = page.locator('.toolbar .menu_right');
        await expect(popup).toBeVisible();

        const popupBox = await popup.boundingBox();
        const toolbarBox = await toolbar.boundingBox();
        expect(popupBox).not.toBeNull();
        expect(toolbarBox).not.toBeNull();
        if (popupBox && toolbarBox) {
            expect(popupBox.y + popupBox.height).toBeLessThanOrEqual(toolbarBox.y + 1);
        }
    });
});
