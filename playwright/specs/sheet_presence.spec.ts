import { test, expect, type Page } from '@playwright/test';

// 0-based cell locator. Row header is th (child 1), so data column c is
// td:nth-child(c + 2); data row r is tbody tr:nth-child(r + 1).
const cell = (page: Page, r: number, c: number) =>
  page.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);

async function openSheet(page: Page, padId: string): Promise<void> {
  await page.goto(`/s/${padId}`);
  await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
}

async function commitCell(page: Page, r: number, c: number, text: string): Promise<void> {
  await cell(page, r, c).click();
  await page.keyboard.type(text, { delay: 30 });
  await page.keyboard.press('Enter');
}

test.describe('Sheet live presence & live calculation', () => {
  test('cursor presence and live calculation across two sessions', async ({ browser }) => {
    test.setTimeout(120000);
    const padId = `sheet-presence-${Date.now()}`;

    // Session A sets up A1=10 and C2==B2+1.
    const ctxA = await browser.newContext();
    const pageA = await ctxA.newPage();
    await openSheet(pageA, padId);
    await commitCell(pageA, 0, 0, '10');     // A1
    await commitCell(pageA, 1, 2, '=B2+1');  // C2

    // Session B joins and sees the committed value.
    const ctxB = await browser.newContext();
    const pageB = await ctxB.newPage();
    await openSheet(pageB, padId);
    await expect(cell(pageB, 0, 0)).toHaveText('10', { timeout: 20000 });

    // A focuses B2 -> B sees a remote cursor tag on B2.
    await cell(pageA, 1, 1).click();
    await expect(cell(pageB, 1, 1).locator('.sheet-remote-tag')).toBeVisible({ timeout: 20000 });

    // A types a formula in B2 WITHOUT committing -> B sees the live formula text
    // in B2 and C2 recomputed to 31, before Enter.
    await pageA.keyboard.type('=A1*3', { delay: 50 });
    await expect(cell(pageB, 1, 1)).toHaveText('=A1*3', { timeout: 20000 });
    await expect(cell(pageB, 1, 2)).toHaveText('31', { timeout: 20000 });

    // A commits -> B2 shows the computed result 30 (overlay replaced).
    await pageA.keyboard.press('Enter');
    await expect(cell(pageB, 1, 1)).toHaveText('30', { timeout: 20000 });

    // A disconnects -> A's cursor tag disappears on B (reused USER_LEAVE).
    await ctxA.close();
    await expect(cell(pageB, 1, 1).locator('.sheet-remote-tag')).toHaveCount(0, { timeout: 20000 });

    await ctxB.close();
  });
});
