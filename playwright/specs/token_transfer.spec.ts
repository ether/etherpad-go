import {expect, test} from "@playwright/test";
import {goToNewPad, goToPad, setUserName, toggleUserList} from "../helper/padHelper";

// Session transfer on the welcome page (etherpad-lite's tokenTransfer): a
// second browser that redeems the code becomes the same author, so it sees
// the name the first browser chose.
test('transfers the author session to another browser', async ({page, browser}) => {
    const padId = await goToNewPad(page);
    const name = `Transferred ${Date.now()}`;
    await toggleUserList(page);
    await setUserName(page, name);
    await page.keyboard.press('Enter');
    await toggleUserList(page);
    // Let the name reach the server before leaving the pad.
    await page.waitForTimeout(500);

    await page.goto('/');
    await page.locator('#transferSection summary').click();
    await page.locator('#transferNow').click();
    const codeField = page.locator('#transferCode');
    await expect(codeField).toBeVisible();
    const code = await codeField.inputValue();
    expect(code).toMatch(/^[0-9a-f]{32}$/);

    const other = await browser.newContext();
    try {
        const page2 = await other.newPage();
        await page2.goto(`/?transfer=${code}`);
        await expect(page2.locator('#transferSection')).toHaveAttribute('open', '');
        await expect(page2.locator('#codeInput')).toHaveValue(code);
        await Promise.all([
            page2.waitForURL((u) => !u.searchParams.has('transfer')),
            page2.locator('#receiveButton').click(),
        ]);

        await goToPad(page2, padId);
        await toggleUserList(page2);
        await expect(page2.locator('#myusernameedit')).toHaveValue(name);

        // The code is single use.
        await page2.goto(`/?transfer=${code}`);
        await page2.locator('#receiveButton').click();
        await expect(page2.locator('#receiveError')).toBeVisible();
    } finally {
        await other.close();
    }
});

test('a malformed code cannot be submitted', async ({page}) => {
    await page.goto('/');
    await page.locator('#transferSection summary').click();
    await page.locator('#codeInput').fill('../admin');
    await expect(page.locator('#receiveButton')).toBeDisabled();
});
