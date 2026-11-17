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

test.describe('Sheet formula references', () => {
  test('inserting and deleting rows keeps formulas on their cells; undo restores', async ({ page }) => {
    await openSheet(page, `refs-rows-${Date.now()}`);
    await typeInto(page, 0, 0, '5');
    await typeInto(page, 1, 0, '=A1*2');
    await typeInto(page, 2, 0, '=SUM(A1:A2)');
    await typeInto(page, 0, 1, '=A2+1');
    await expect(cell(page, 0, 1)).toHaveText('11');

    await cell(page, 0, 0).click();
    await page.locator('[title="Insert row above"]').click();
    await expect(cell(page, 2, 0)).toHaveText('10');
    await expect(cell(page, 3, 0)).toHaveText('15');
    await expect(cell(page, 1, 1)).toHaveText('11');

    // Deleting the row a formula points at gives #REF!, like Excel.
    await cell(page, 1, 0).click();
    await page.locator('[title="Delete selected rows"]').click();
    await expect(cell(page, 1, 0)).toHaveText('#REF!');
    await page.keyboard.press('Control+z');
    await expect(cell(page, 2, 0)).toHaveText('10');
    await expect(cell(page, 1, 1)).toHaveText('11');
  });

  test('formulas reference other sheets and follow a rename', async ({ page }) => {
    const pad = `refs-sheets-${Date.now()}`;
    await openSheet(page, pad);
    await typeInto(page, 0, 0, '21');
    await page.locator('.sheet-tab-add').click();
    await typeInto(page, 0, 0, '=Sheet1!A1*2');
    await expect(cell(page, 0, 0)).toHaveText('42');

    page.once('dialog', (d) => d.accept('Daten 2024'));
    await page.locator('.sheet-tabs button', { hasText: 'Sheet1' }).dblclick();
    await expect(page.locator('.sheet-tabs button', { hasText: 'Daten 2024' })).toHaveCount(1);
    await page.locator('.sheet-tabs button', { hasText: 'Sheet2' }).click();
    await expect(cell(page, 0, 0)).toHaveText('42');
    await cell(page, 0, 0).click();
    await expect(page.locator('.sheet-fx-input').first()).toHaveValue("='Daten 2024'!A1*2");
    await page.keyboard.press('Escape');

    // After a reload the server's copy says the same.
    await openSheet(page, pad);
    await page.locator('.sheet-tabs button', { hasText: 'Sheet2' }).click();
    await expect(cell(page, 0, 0)).toHaveText('42');
  });

  test('numbers are right-aligned unless the cell says otherwise', async ({ page }) => {
    await openSheet(page, `refs-align-${Date.now()}`);
    await typeInto(page, 0, 0, '12');
    await typeInto(page, 1, 0, 'Text');
    await expect(cell(page, 0, 0)).toHaveCSS('text-align', 'right');
    await expect(cell(page, 1, 0)).not.toHaveCSS('text-align', 'right');
  });
});
