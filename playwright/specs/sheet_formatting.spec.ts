import { test, expect, type Page } from '@playwright/test';

// 0-based cell locator. Row header is th (child 1), so data column c is
// td:nth-child(c + 2); data row r is tbody tr:nth-child(r + 1). Mirrors
// sheet_selection.spec.ts.
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

test.describe('Sheet formatting', () => {
  test('bold applies to the selection and renders bold', async ({ page }) => {
    const padId = `sheet-fmt-bold-${Date.now()}`;
    await openSheet(page, padId);

    await commitCell(page, 0, 0, 'hi'); // A1

    // Re-select A1, then toggle bold via the ribbon (Home tab is the default).
    await cell(page, 0, 0).click();
    await page.locator('.sheet-toolbar button').filter({ hasText: /^B$/ }).click();

    await expect(cell(page, 0, 0)).toHaveCSS('font-weight', /700|bold/);
  });

  test('number format renders a grouped number', async ({ page }) => {
    const padId = `sheet-fmt-num-${Date.now()}`;
    await openSheet(page, padId);

    await commitCell(page, 0, 0, '1234.5'); // A1

    await cell(page, 0, 0).click();
    // Number-format select is the FIRST select in the toolbar (Home tab, default).
    await page.locator('.sheet-toolbar select').first().selectOption('number:2');

    await expect(cell(page, 0, 0)).toHaveText('1,234.50');
  });
});
