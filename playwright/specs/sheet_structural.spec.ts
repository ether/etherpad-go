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

// Ribbon groups of inactive tabs are display:none — activate the tab first.
async function ribbonTab(page: Page, name: 'Home' | 'Data' | 'View'): Promise<void> {
  await page.locator('.sheet-ribbon-tabs button', { hasText: new RegExp(`^${name}$`) }).click();
}

test.describe('Sheet M4 structural', () => {
  test('tabs: add, switch, rename; data stays per sheet', async ({ page }) => {
    await openSheet(page, `m4-tabs-${Date.now()}`);
    await typeInto(page, 0, 0, 'erstes');

    await page.locator('.sheet-tab-add').click();
    await expect(page.locator('.sheet-tabs button.sheet-tab-active')).toHaveText('Sheet2');
    // The new sheet is empty; the first sheet's value must not bleed through.
    await expect(cell(page, 0, 0)).toHaveText('');

    await typeInto(page, 0, 0, 'zweites');
    await page.locator('.sheet-tabs button', { hasText: 'Sheet1' }).click();
    await expect(cell(page, 0, 0)).toHaveText('erstes');

    // rename via dblclick prompt
    page.once('dialog', (d) => d.accept('Daten'));
    await page.locator('.sheet-tabs button', { hasText: 'Sheet2' }).dblclick();
    await expect(page.locator('.sheet-tabs button', { hasText: 'Daten' })).toHaveCount(1);
  });

  test('column resize emits setDimension and persists across reload', async ({ page }) => {
    const pad = `m4-resize-${Date.now()}`;
    await openSheet(page, pad);
    const colB = page.locator('.sheet-grid thead th').nth(2); // corner + A + B
    const before = (await colB.boundingBox())!.width;
    const grip = colB.locator('.sheet-resizer-col');
    const gb = (await grip.boundingBox())!;
    await page.mouse.move(gb.x + gb.width / 2, gb.y + gb.height / 2);
    await page.mouse.down();
    await page.mouse.move(gb.x + gb.width / 2 + 60, gb.y + gb.height / 2);
    await page.mouse.up();
    await page.waitForTimeout(500);
    const after = (await colB.boundingBox())!.width;
    expect(after).toBeGreaterThan(before + 40);

    await page.reload();
    await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
    const persisted = (await page.locator('.sheet-grid thead th').nth(2).boundingBox())!.width;
    expect(persisted).toBeGreaterThan(before + 40);
  });

  test('freeze first row makes it sticky', async ({ page }) => {
    await openSheet(page, `m4-freeze-${Date.now()}`);
    await typeInto(page, 0, 0, 'kopf');
    await ribbonTab(page, 'View');
    await page.locator('.sheet-toolbar button', { hasText: '❄ Row' }).click();
    await page.waitForTimeout(400);
    await expect(page.locator('.sheet-grid')).toHaveClass(/sheet-frozen-r/);
    const sticky = await cell(page, 0, 0).evaluate((el) => getComputedStyle(el).position);
    expect(sticky).toBe('sticky');
  });

  test('sort A→Z reorders the selected range by the focused column', async ({ page }) => {
    await openSheet(page, `m4-sort-${Date.now()}`);
    await typeInto(page, 0, 0, '3');
    await typeInto(page, 1, 0, '1');
    await typeInto(page, 2, 0, '2');
    // select A1:A3 by dragging
    await cell(page, 0, 0).hover();
    await page.mouse.down();
    await cell(page, 2, 0).hover();
    await page.mouse.up();
    await ribbonTab(page, 'Data');
    await page.locator('.sheet-toolbar button', { hasText: 'A→Z' }).click();
    await page.waitForTimeout(600);
    await expect(cell(page, 0, 0)).toHaveText('1');
    await expect(cell(page, 1, 0)).toHaveText('2');
    await expect(cell(page, 2, 0)).toHaveText('3');
  });

  test('filter by a checklist of values, for everyone', async ({ page, browser }) => {
    const pad = `m4-filter-${Date.now()}`;
    await openSheet(page, pad);
    for (const [r, v] of ['Name', 'x', 'y', 'z', 'x'].entries()) await typeInto(page, r, 0, v);
    const row = (p: typeof page, r: number) => p.locator(`.sheet-grid tbody tr[data-row="${r}"]`);
    const menu = page.locator('.sheet-filter-menu');
    await cell(page, 1, 0).click();
    await ribbonTab(page, 'Data');
    const filterBtn = page.locator('.sheet-filter-btn');

    // Uncheck y: its row goes, the header (first filled row) stays.
    await filterBtn.click();
    await expect(menu.locator('label')).toHaveText(['(Select all)', 'x', 'y', 'z']);
    await menu.locator('label', { hasText: /^y$/ }).locator('input').uncheck();
    await menu.locator('button.primary').click();
    await expect(row(page, 2)).toBeHidden();
    await expect(row(page, 0)).toBeVisible();
    // The keyboard is back in the grid.
    expect(await page.evaluate(() => document.activeElement?.tagName)).toBe('TD');
    await expect(row(page, 3)).toBeVisible();
    await expect(page.locator('.sheet-grid thead th.sheet-head-filter')).toHaveText('A');

    // The filter is part of the sheet: a second user sees it too.
    const other = await browser.newPage();
    await openSheet(other, pad);
    await expect(row(other, 2)).toBeHidden();
    await expect(row(other, 4)).toBeVisible();

    // Search narrows the list; "(Select all)" then acts on what is shown.
    await filterBtn.click();
    await menu.locator('input[type=search]').fill('z');
    await menu.locator('label.sheet-filter-all input').uncheck();
    await menu.locator('button.primary').click();
    await expect(row(page, 3)).toBeHidden();
    await expect(row(other, 3)).toBeHidden();
    await expect(row(page, 1)).toBeVisible();

    // Enter on Cancel cancels.
    await filterBtn.click();
    await menu.locator('label.sheet-filter-all input').uncheck();
    await menu.locator('label', { hasText: /^x$/ }).locator('input').check();
    await menu.locator('button', { hasText: 'Cancel' }).focus();
    await page.keyboard.press('Enter');
    await expect(menu).toHaveCount(0);
    await expect(row(page, 4)).toBeVisible();
    await expect(row(page, 2)).toBeHidden();

    await filterBtn.click();
    await menu.locator('button', { hasText: 'Clear filter from this column' }).click();
    await expect(row(page, 2)).toBeVisible();
    await expect(row(other, 3)).toBeVisible();
    await other.close();
  });
});
