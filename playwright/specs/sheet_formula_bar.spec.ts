import { test, expect, type Page } from '@playwright/test';

// 0-based cell locator. Row header is th (child 1), so data column c is
// td:nth-child(c + 2); data row r is tbody tr:nth-child(r + 1). Mirrors
// sheet_selection.spec.ts / sheet_formatting.spec.ts.
const cell = (page: Page, r: number, c: number) =>
  page.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);

async function openSheet(page: Page, padId: string): Promise<void> {
  await page.goto(`/s/${padId}`);
  await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
}

test.describe('Sheet formula bar', () => {
  test('name box shows the active ref; committing from the bar sets the cell', async ({ page }) => {
    const padId = `fx-e2e-${Date.now()}`;
    await openSheet(page, padId);

    await cell(page, 2, 1).click(); // row2, col1 -> B3
    await expect(page.locator('.sheet-namebox')).toHaveText('B3');

    const fx = page.locator('.sheet-fx-input');
    await fx.click();
    await fx.fill('=1+2');
    await fx.press('Enter');
    await expect(cell(page, 2, 1)).toHaveText('3');
  });

  test('an invalid formula renders as a styled error cell', async ({ page }) => {
    const padId = `fx-err-${Date.now()}`;
    await openSheet(page, padId);

    await cell(page, 0, 0).click();
    const fx = page.locator('.sheet-fx-input');
    await fx.click();
    await fx.fill('=1/0');
    await fx.press('Enter');

    await expect(cell(page, 0, 0)).toHaveText('#DIV/0!');
    await expect(cell(page, 0, 0)).toHaveClass(/sheet-cell-error/);
  });

  test('typing a function prefix opens autocomplete with a matching entry', async ({ page }) => {
    const padId = `fx-ac-${Date.now()}`;
    await openSheet(page, padId);

    await cell(page, 0, 0).click();
    const fx = page.locator('.sheet-fx-input');
    await fx.click();
    // pressSequentially (not fill) — the dropdown is driven by real 'input'
    // events plus caret position (see functionPrefix in autocomplete.ts),
    // which fill() does not reliably produce.
    await fx.pressSequentially('=SU');

    const ac = page.locator('.sheet-fx-ac');
    await expect(ac).toBeVisible();
    // Exact match: hasText is substring-based and would also count SUMIF,
    // SUMSQ, ... — six entries match the 'SU' prefix.
    await expect(ac.locator('div').filter({ hasText: /^SUM$/ })).toHaveCount(1);
  });
});
