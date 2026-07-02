import fs from 'node:fs';
import { test, expect, type Page } from '@playwright/test';

// 0-based cell locator. Row header is th (child 1), so data column c is
// td:nth-child(c + 2); data row r is tbody tr:nth-child(r + 1). Mirrors
// sheet_structural.spec.ts.
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
  await page.locator('.sheet-ribbon-tabs button', { hasText: name }).click();
}

const toolbarButton = (page: Page, text: string | RegExp) =>
  page.locator('.sheet-toolbar button').filter({ hasText: text });

// Import/Export buttons are only unique by title in the Excel ribbon (the
// File menu carries similar labels).
const exportBtn = (page: Page) => page.locator('.sheet-toolbar button[title="Export as .xlsx"]');
const importBtn = (page: Page) => page.locator('.sheet-toolbar button[title="Import .xlsx (replaces this sheet)"]');

test.describe('Sheet ribbon', () => {
  test('tab switch shows the active group and hides the rest', async ({ page }) => {
    await openSheet(page, `ribbon-tabs-${Date.now()}`);

    // Home is the default tab: bold visible, sort hidden.
    await expect(toolbarButton(page, /^B$/)).toBeVisible();

    await ribbonTab(page, 'Data');
    await expect(toolbarButton(page, 'A→Z')).toBeVisible();
    await expect(toolbarButton(page, /^B$/)).toBeHidden();

    await ribbonTab(page, 'Home');
    await expect(toolbarButton(page, /^B$/)).toBeVisible();
    await expect(toolbarButton(page, 'A→Z')).toBeHidden();
  });

  test('insert row above shifts rows down; delete selected rows restores', async ({ page }) => {
    await openSheet(page, `ribbon-rows-${Date.now()}`);
    await typeInto(page, 0, 0, 'x'); // A1
    await typeInto(page, 1, 0, 'y'); // A2

    await cell(page, 1, 0).click(); // focus A2
    await ribbonTab(page, 'Home');
    await page.locator('.sheet-toolbar button[title="Insert row above"]').click();
    await expect(cell(page, 0, 0)).toHaveText('x');
    await expect(cell(page, 1, 0)).toHaveText('');
    await expect(cell(page, 2, 0)).toHaveText('y');

    await cell(page, 1, 0).click(); // select the inserted row 2
    await page.locator('.sheet-toolbar button[title="Delete selected rows"]').click();
    await expect(cell(page, 1, 0)).toHaveText('y');
  });

  test('insert column left shifts columns right', async ({ page }) => {
    await openSheet(page, `ribbon-cols-${Date.now()}`);
    await typeInto(page, 0, 1, 'b'); // B1

    await cell(page, 0, 1).click(); // focus B1
    await ribbonTab(page, 'Home');
    await page.locator('.sheet-toolbar button[title="Insert column left"]').click();
    await expect(cell(page, 0, 1)).toHaveText('');
    await expect(cell(page, 0, 2)).toHaveText('b');
  });

  test('export downloads an .xlsx file', async ({ page }) => {
    await openSheet(page, `ribbon-export-${Date.now()}`);
    await typeInto(page, 0, 0, 'wert');

    await ribbonTab(page, 'Data');
    const downloadPromise = page.waitForEvent('download');
    await exportBtn(page).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/\.xlsx$/);
  });

  test('import roundtrips an exported workbook', async ({ page }) => {
    await openSheet(page, `ribbon-import-${Date.now()}`);
    await typeInto(page, 0, 0, 'alpha'); // A1
    await typeInto(page, 1, 1, 'beta'); // B2

    await ribbonTab(page, 'Data');
    const downloadPromise = page.waitForEvent('download');
    await exportBtn(page).click();
    const download = await downloadPromise;
    const buffer = fs.readFileSync(await download.path());

    const chooserPromise = page.waitForEvent('filechooser');
    await importBtn(page).click();
    const chooser = await chooserPromise;
    // Import triggers SHEET_RELOAD — arm the load waiter before setFiles.
    const reloaded = page.waitForEvent('load', { timeout: 20000 });
    await chooser.setFiles({
      name: 'wb.xlsx',
      mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      buffer,
    });
    await reloaded;
    await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });

    await expect(cell(page, 0, 0)).toHaveText('alpha');
    await expect(cell(page, 1, 1)).toHaveText('beta');
  });
});
