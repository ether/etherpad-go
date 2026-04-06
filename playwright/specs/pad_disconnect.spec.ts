import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

test.beforeEach(async ({ page }) => {
    await goToNewPad(page);
})

test('kicked disconnect modal exists in DOM', async ({page}) => {
    const kickedModal = page.locator('#connectivity .kicked');
    await expect(kickedModal).toBeAttached();
})

test('deleted disconnect modal exists in DOM', async ({page}) => {
    const deletedModal = page.locator('#connectivity .deleted');
    await expect(deletedModal).toBeAttached();
})

test('connectivity popup is hidden by default', async ({page}) => {
    const connectivity = page.locator('#connectivity');
    await expect(connectivity).not.toHaveClass(/popup-show/);
})
