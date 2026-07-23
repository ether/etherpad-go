import { test, expect, type Page } from '@playwright/test';

const cell = (page: Page, r: number, c: number) =>
  page.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);

async function openSheet(page: Page, padId: string): Promise<void> {
  await page.goto(`/s/${padId}`);
  await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
}

async function typeInto(page: Page, r: number, c: number, text: string): Promise<void> {
  await cell(page, r, c).click();
  await page.keyboard.type(text);
  await page.keyboard.press('Enter');
}

async function selectRange(page: Page, r0: number, c0: number, r1: number, c1: number): Promise<void> {
  await cell(page, r0, c0).hover();
  await page.mouse.down();
  await cell(page, r1, c1).hover();
  await page.mouse.up();
}

const mergeBtn = (page: Page) => page.locator('.sheet-toolbar button[title="Merge / unmerge cells"]');

test.describe('Sheet merged cells', () => {
  test('merge spans the anchor, hides covered cells, unmerge restores', async ({ page }) => {
    await openSheet(page, `merge-basic-${Date.now()}`);
    await typeInto(page, 0, 0, 'kopf');
    await typeInto(page, 1, 1, 'versteckt');

    await selectRange(page, 0, 0, 1, 1);
    await mergeBtn(page).click();
    await page.waitForTimeout(400);

    await expect(cell(page, 0, 0)).toHaveAttribute('colspan', '2');
    await expect(cell(page, 0, 0)).toHaveAttribute('rowspan', '2');
    await expect(cell(page, 1, 1)).toBeHidden();
    await expect(cell(page, 0, 0)).toHaveText('kopf');

    // unmerge: content of covered cells was kept, not deleted
    await cell(page, 0, 0).click();
    await mergeBtn(page).click();
    await page.waitForTimeout(400);
    await expect(cell(page, 1, 1)).toBeVisible();
    await expect(cell(page, 1, 1)).toHaveText('versteckt');
  });

  test('merge persists across reload and reaches a second client', async ({ page, browser }) => {
    const pad = `merge-collab-${Date.now()}`;
    await openSheet(page, pad);
    await selectRange(page, 2, 0, 2, 2);
    await mergeBtn(page).click();
    await page.waitForTimeout(400);

    // second client sees the merge live
    const ctx2 = await browser.newContext();
    const page2 = await ctx2.newPage();
    await openSheet(page2, pad);
    await expect(cell(page2, 2, 0)).toHaveAttribute('colspan', '3');
    await ctx2.close();

    // and it survives a reload (snapshot round-trip)
    await page.reload();
    await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
    await expect(cell(page, 2, 0)).toHaveAttribute('colspan', '3');
  });

  test('inserting a row above shifts the merge down', async ({ page }) => {
    await openSheet(page, `merge-shift-${Date.now()}`);
    await selectRange(page, 3, 0, 4, 1);
    await mergeBtn(page).click();
    await page.waitForTimeout(400);
    await expect(cell(page, 3, 0)).toHaveAttribute('rowspan', '2');

    await cell(page, 0, 0).click();
    await page.locator('.sheet-toolbar button[title="Insert row above"]').click();
    await page.waitForTimeout(400);
    await expect(cell(page, 4, 0)).toHaveAttribute('rowspan', '2');
    await expect(cell(page, 3, 0)).not.toHaveAttribute('rowspan', '2');
  });

  // Model semantics (absorb-on-overlap, insert-at-trailing-edge, delete-shrink)
  // are covered by unit tests in workbookState.test.ts / merge_test.go and are
  // not reachable through the toolbar toggle, so they stay out of the e2e layer.

  test('arrow keys snap to the anchor and step past covered cells', async ({ page }) => {
    await openSheet(page, `merge-nav-${Date.now()}`);
    await selectRange(page, 1, 0, 2, 0); // merge A2:A3 (rows 1..2, col 0)
    await mergeBtn(page).click();
    await page.waitForTimeout(400);
    await expect(cell(page, 1, 0)).toHaveAttribute('rowspan', '2');
    await expect(cell(page, 2, 0)).toBeHidden();

    await cell(page, 0, 0).click();
    await page.keyboard.press('ArrowDown'); // into the merge → snaps to anchor
    await expect(cell(page, 1, 0)).toHaveClass(/sheet-sel-focus/);
    await page.keyboard.press('ArrowDown'); // leaving the merge → skips covered row
    await expect(cell(page, 3, 0)).toHaveClass(/sheet-sel-focus/);
  });
});
