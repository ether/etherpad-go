import {expect, Page, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

// A read-only pad shows a hint instead of the formatting buttons, which
// would do nothing there, and says so when someone taps into the text.

const openReadOnly = async (page: Page) => {
    await goToNewPad(page);
    const readOnlyId = await page.evaluate(() => (window as any).clientVars.readOnlyId);
    await page.goto(new URL(`/p/${readOnlyId}`, page.url()).href);
    await page.waitForFunction(() => document.body.classList.contains('readonly'));
};

test.describe('read-only pad', () => {
    test('hint instead of the formatting toolbar, no login offer on a read-only link', async ({page}) => {
        await openReadOnly(page);
        const hint = page.getByTestId('readonly-hint');
        await expect(hint).toBeVisible();
        await expect(hint).toContainText(/read only|nur lesen/i);
        await expect(page.locator('#editbar ul.menu_left')).toBeHidden();
        await expect(page.getByTestId('readonly-hint-login')).toHaveCount(0);
        // The pad actions stay.
        await expect(page.locator('#editbar ul.menu_right li[data-key="showTimeSlider"]')).toBeVisible();
    });

    test('a tap into the text says the pad is read-only', async ({page}) => {
        await openReadOnly(page);
        const box = await page.locator('#editorcontainer').boundingBox();
        expect(box).not.toBeNull();
        if (!box) return;
        await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
        await expect(page.locator('ep-notification').filter({hasText: /read-only|schreibgeschützt/i})).toHaveCount(1);
    });

    test('on a phone the hint takes the top bar', async ({page}) => {
        await page.setViewportSize({width: 390, height: 844});
        await openReadOnly(page);
        const hint = await page.getByTestId('readonly-hint').boundingBox();
        expect(hint).not.toBeNull();
        if (hint) expect(hint.y).toBeLessThan(80);
        await expect(page.locator('#editbar ul.menu_right')).toBeVisible();
    });

    test('a pad that can be edited shows no hint', async ({page}) => {
        await goToNewPad(page);
        await expect(page.locator('#editbar ul.menu_left')).toBeVisible();
        await expect(page.getByTestId('readonly-hint')).toHaveCount(0);
    });
});
