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
    await page.locator('.sheet-toolbar button', { hasText: '❄R' }).click();
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
    await page.locator('.sheet-toolbar button', { hasText: 'A→Z' }).click();
    await page.waitForTimeout(600);
    await expect(cell(page, 0, 0)).toHaveText('1');
    await expect(cell(page, 1, 0)).toHaveText('2');
    await expect(cell(page, 2, 0)).toHaveText('3');
  });

  test('filter hides non-matching rows client-side', async ({ page }) => {
    await openSheet(page, `m4-filter-${Date.now()}`);
    await typeInto(page, 0, 0, 'x');
    await typeInto(page, 1, 0, 'y');
    await typeInto(page, 2, 0, 'x');
    await cell(page, 0, 0).click();
    // The dropdown fills its options lazily on open; selectOption() bypasses
    // the native open, so trigger the repopulation explicitly first.
    await page.locator('.sheet-toolbar select').last().dispatchEvent('mousedown');
    await page.locator('.sheet-toolbar select').last().selectOption('x');
    await expect(page.locator('.sheet-grid tbody tr').nth(1)).toBeHidden();
    await expect(page.locator('.sheet-grid tbody tr').nth(0)).toBeVisible();
    await expect(page.locator('.sheet-grid tbody tr').nth(2)).toBeVisible();
    // clear
    await page.locator('.sheet-toolbar select').last().selectOption('');
    await expect(page.locator('.sheet-grid tbody tr').nth(1)).toBeVisible();
  });
});
