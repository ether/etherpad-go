import { test, expect, type Page } from '@playwright/test';

const cellAt = (page: Page, r: number, c: number) =>
  page.locator(`.sheet-grid tbody tr[data-row="${r}"] td:nth-child(${c + 2})`);

async function openSheet(page: Page, padId: string): Promise<void> {
  await page.goto(`/s/${padId}`);
  await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
}

async function typeInto(page: Page, r: number, c: number, text: string): Promise<void> {
  await cellAt(page, r, c).click();
  await page.keyboard.type(text);
  await page.keyboard.press('Enter');
}

test.describe('Sheet charts', () => {
  test('insert, follow the data, edit, move, share, delete and undo', async ({ page, browser }) => {
    const pad = `charts-${Date.now()}`;
    await openSheet(page, pad);
    const rows = [['Monat', 'Umsatz', 'Kosten'], ['Jan', '120', '80'], ['Feb', '150', '95'], ['Mär', '90', '70']];
    for (const [r, row] of rows.entries()) for (const [c, v] of row.entries()) await typeInto(page, r, c, v);

    await cellAt(page, 0, 0).click();
    await page.keyboard.down('Shift');
    for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowDown');
    for (let i = 0; i < 2; i++) await page.keyboard.press('ArrowRight');
    await page.keyboard.up('Shift');
    await page.locator('.sheet-ribbon-tabs button', { hasText: /^Insert$/ }).click();
    await page.locator('[title="Insert a chart of the selected cells"]').click();
    await expect(page.locator('#sheet-dlg-range')).toHaveValue('A1:C4');
    await page.locator('#sheet-dlg-title').fill('Umsatz');
    await page.locator('.sheet-dialog button.primary').click();

    const chart = page.locator('.sheet-chart');
    await expect(chart).toHaveCount(1);
    // First series: three bars plus its legend swatch.
    await expect(chart.locator('svg rect[fill="#4472c4"]')).toHaveCount(4);
    await expect(chart.locator('svg')).toHaveAttribute('aria-label', 'Umsatz');
    const bars = () => chart.locator('svg rect').count();
    const before = await bars();

    // The chart follows its data.
    await typeInto(page, 4, 0, 'Apr');
    await typeInto(page, 4, 1, '200');
    await expect.poll(bars).toBe(before); // A5 is outside A1:C4
    await chart.dblclick();
    await page.locator('#sheet-dlg-range').fill('a1:c5');
    await page.locator('#sheet-dlg-type').selectOption('line');
    await page.locator('.sheet-dialog button.primary').click();
    await expect(chart.locator('svg path')).toHaveCount(2);

    // Drag it: it snaps to the cell under its top-left corner.
    const box = (await chart.boundingBox())!;
    await page.mouse.move(box.x + 40, box.y + 40);
    await page.mouse.down();
    await page.mouse.move(box.x + 40, box.y + 40 + 6 * 27, { steps: 5 });
    await page.mouse.up();
    const moved = (await chart.boundingBox())!;
    expect(moved.y).toBeGreaterThan(box.y + 100);

    // Everyone sees it, also after a reload.
    const other = await browser.newPage();
    await openSheet(other, pad);
    await expect(other.locator('.sheet-chart svg path')).toHaveCount(2);
    await expect(other.locator('.sheet-chart svg')).toHaveAttribute('aria-label', 'Umsatz');

    // Delete with the keyboard, undo brings it back.
    await chart.click();
    await page.keyboard.press('Delete');
    await expect(chart).toHaveCount(0);
    await expect(other.locator('.sheet-chart')).toHaveCount(0);
    await cellAt(page, 8, 8).click();
    await page.keyboard.press('Control+z');
    await expect(chart).toHaveCount(1);
    await expect(other.locator('.sheet-chart')).toHaveCount(1);
    await other.close();
  });
});
