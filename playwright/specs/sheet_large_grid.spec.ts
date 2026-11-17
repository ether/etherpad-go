import { test, expect, type Page } from '@playwright/test';

// Rows are rendered only near the viewport, so address them by data-row.
const cellAt = (page: Page, r: number, c: number) =>
  page.locator(`.sheet-grid tbody tr[data-row="${r}"] td:nth-child(${c + 2})`);

async function openSheet(page: Page, padId: string): Promise<void> {
  await page.goto(`/s/${padId}`);
  await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
}

test.describe('Sheet large grid', () => {
  test('10000 rows and 104 columns, only the visible rows in the DOM', async ({ page }) => {
    await openSheet(page, `big-${Date.now()}`);
    await expect(page.locator('.sheet-grid thead th').last()).toHaveText('CZ');
    const rendered = await page.locator('.sheet-grid tbody tr[data-row]').count();
    expect(rendered).toBeLessThan(200);

    await page.locator('.sheet-grid-host').evaluate((el) => { el.scrollTop = 4999 * 27; });
    await expect(cellAt(page, 4999, 1)).toBeVisible();
    await cellAt(page, 4999, 1).click();
    await page.keyboard.type('weit unten');
    await page.keyboard.press('Enter');
    await expect(cellAt(page, 4999, 1)).toHaveText('weit unten');
    await expect(page.locator('.sheet-grid tbody tr[data-row="0"]')).toHaveCount(0);

    // A formula far up sees it.
    await page.locator('.sheet-grid-host').evaluate((el) => { el.scrollTop = 0; });
    await cellAt(page, 0, 0).click();
    await page.keyboard.type('=B5000');
    await page.keyboard.press('Enter');
    await expect(cellAt(page, 0, 0)).toHaveText('weit unten');
  });

  test('arrow keys move the typing cell and scroll along', async ({ page }) => {
    await openSheet(page, `big-keys-${Date.now()}`);
    await cellAt(page, 0, 0).click();
    for (let i = 0; i < 60; i++) await page.keyboard.press('ArrowDown');
    await page.keyboard.type('x');
    await page.keyboard.press('Enter');
    await expect(cellAt(page, 60, 0)).toHaveText('x');
    await expect(cellAt(page, 60, 0)).toBeInViewport();
  });

  test('freeze panes at the selected cell keeps rows and columns in place', async ({ page }) => {
    await openSheet(page, `big-freeze-${Date.now()}`);
    await cellAt(page, 2, 1).click(); // B3: freeze rows 1-2 and column A
    await page.locator('.sheet-ribbon-tabs button', { hasText: /^View$/ }).click();
    await page.locator('.sheet-toolbar button', { hasText: '❄ Panes' }).click();
    await expect(cellAt(page, 1, 0)).toHaveCSS('position', 'sticky');
    await expect(cellAt(page, 5, 0)).toHaveCSS('position', 'sticky');
    await expect(cellAt(page, 5, 1)).not.toHaveCSS('position', 'sticky');
    await page.locator('.sheet-grid-host').evaluate((el) => { el.scrollTop = 3000; });
    await expect(cellAt(page, 1, 0)).toBeInViewport();

    // A reload keeps it (the freeze is stored with the sheet).
    await page.reload();
    await page.locator('.sheet-grid').waitFor();
    await expect(cellAt(page, 1, 1)).toHaveCSS('position', 'sticky');
  });
});
