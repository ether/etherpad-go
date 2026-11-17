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

async function ribbonTab(page: Page, name: string): Promise<void> {
  await page.locator('.sheet-ribbon-tabs button', { hasText: new RegExp(`^${name}$`) }).click();
}

const dialog = (page: Page) => page.locator('.sheet-dialog');

test.describe('Sheet links, notes, validation and conditional formatting', () => {
  test('insert a link; it survives a reload', async ({ page }) => {
    const pad = `meta-link-${Date.now()}`;
    await openSheet(page, pad);
    await cell(page, 0, 0).click();
    await ribbonTab(page, 'Insert');
    await page.locator('[title="Insert link (Ctrl+K)"]').click();
    await dialog(page).locator('#sheet-dlg-text').fill('Etherpad');
    await dialog(page).locator('#sheet-dlg-url').fill('javascript:alert(1)');
    await dialog(page).locator('button.primary').click();
    await expect(dialog(page).locator('.sheet-dialog-error')).toContainText('web address');
    await dialog(page).locator('#sheet-dlg-url').fill('etherpad.org');
    await dialog(page).locator('button.primary').click();
    await expect(dialog(page)).toHaveCount(0);
    await expect(cell(page, 0, 0)).toHaveText('Etherpad');
    await expect(cell(page, 0, 0)).toHaveClass(/sheet-link/);
    await expect(cell(page, 0, 0)).toHaveAttribute('title', /https:\/\/etherpad\.org/);

    await openSheet(page, pad);
    await expect(cell(page, 0, 0)).toHaveClass(/sheet-link/);
  });

  test('a note shows a marker and its text; Delete keeps it', async ({ page }) => {
    await openSheet(page, `meta-note-${Date.now()}`);
    await typeInto(page, 1, 1, '42');
    await cell(page, 1, 1).click();
    await page.keyboard.press('Shift+F2');
    await dialog(page).locator('textarea').fill('Geprüft von Sam');
    await dialog(page).locator('button.primary').click();
    await expect(cell(page, 1, 1)).toHaveClass(/sheet-has-note/);
    await expect(cell(page, 1, 1)).toHaveAttribute('title', /Geprüft von Sam/);

    // Delete clears the value, not the note (Excel).
    await cell(page, 1, 1).click();
    await page.keyboard.press('Delete');
    await expect(cell(page, 1, 1)).toHaveText('');
    await expect(cell(page, 1, 1)).toHaveClass(/sheet-has-note/);
  });

  test('a list validation offers a drop-down and rejects other values', async ({ page }) => {
    await openSheet(page, `meta-list-${Date.now()}`);
    await cell(page, 0, 0).click();
    await page.keyboard.press('Shift+ArrowDown');
    await ribbonTab(page, 'Data');
    await page.locator('[title^="Data validation"]').click();
    await dialog(page).locator('#sheet-dlg-source').fill('rot, grün, blau');
    await dialog(page).locator('button.primary').click();

    await typeInto(page, 0, 0, 'gelb');
    await expect(cell(page, 0, 0)).toHaveText('');
    await expect(page.locator('.sheet-flash')).toContainText('not in the list');
    await typeInto(page, 0, 0, 'GRÜN');
    await expect(cell(page, 0, 0)).toHaveText('grün');

    await cell(page, 1, 0).click();
    await page.locator('.sheet-list-btn').click();
    await page.locator('.sheet-list-menu button', { hasText: 'blau' }).click();
    await expect(cell(page, 1, 0)).toHaveText('blau');
  });

  test('conditional formatting colours matching cells and follows edits', async ({ page }) => {
    await openSheet(page, `meta-cf-${Date.now()}`);
    await typeInto(page, 0, 0, '5');
    await typeInto(page, 1, 0, '15');
    await typeInto(page, 2, 0, '=A1*4');
    await cell(page, 0, 0).click();
    await page.keyboard.press('Shift+ArrowDown');
    await page.keyboard.press('Shift+ArrowDown');
    await page.locator('[title="Conditional Formatting"]').click();
    await page.locator('.sheet-file-menu button', { hasText: 'Greater Than…' }).click();
    await dialog(page).locator('#sheet-dlg-v1').fill('10');
    await dialog(page).locator('button.primary').click();

    const red = 'rgb(255, 199, 206)';
    await expect(cell(page, 0, 0)).not.toHaveCSS('background-color', red);
    await expect(cell(page, 1, 0)).toHaveCSS('background-color', red);
    await expect(cell(page, 2, 0)).toHaveCSS('background-color', red); // =A1*4 = 20
    await cell(page, 0, 0).click();
    await page.keyboard.press('Delete');
    await typeInto(page, 0, 0, '1'); // A3 becomes 4
    await expect(cell(page, 2, 0)).not.toHaveCSS('background-color', red);
  });
});
