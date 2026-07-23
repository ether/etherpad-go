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

// Real mouse drag selection (mousedown -> mouseover -> mouseup), matching
// DomSheetView's listeners. Mirrors sheet_selection.spec.ts.
async function dragSelect(page: Page, r0: number, c0: number, r1: number, c1: number): Promise<void> {
  await cell(page, r0, c0).hover();
  await page.mouse.down();
  await cell(page, r1, c1).hover();
  await page.mouse.up();
}

test.describe('Sheet Excel chrome', () => {
  test('titlebar shows the pad name', async ({ page }) => {
    const padId = `xl-title-${Date.now()}`;
    await openSheet(page, padId);

    const titlebar = page.locator('.sheet-titlebar');
    await expect(titlebar).toBeVisible();
    await expect(titlebar).toContainText(padId);
  });

  test('font family and size selects restyle the selection', async ({ page }) => {
    const padId = `xl-font-${Date.now()}`;
    await openSheet(page, padId);
    await commitCell(page, 0, 0, 'hi'); // A1

    // Home tab is the default; selects are only unique by title.
    await cell(page, 0, 0).click();
    await page.locator('select[title="Font"]').selectOption({ label: 'Arial' });
    await page.locator('select[title="Font size"]').selectOption({ label: '20' });

    await expect(cell(page, 0, 0)).toHaveCSS('font-family', /Arial/);
    // 20pt = 26.667px computed.
    await expect(cell(page, 0, 0)).toHaveCSS('font-size', /^26\.6/);
  });

  test('wrap text switches the cell to normal white-space', async ({ page }) => {
    const padId = `xl-wrap-${Date.now()}`;
    await openSheet(page, padId);
    await commitCell(page, 0, 0, 'a rather long text that overflows the default column width'); // A1

    // Default: no wrapping (overflow is clipped instead).
    const before = await cell(page, 0, 0).evaluate((el) => getComputedStyle(el).whiteSpace);
    expect(before).not.toBe('normal');

    await cell(page, 0, 0).click();
    await page.locator('.sheet-toolbar button[title="Wrap text"]').click();

    await expect(cell(page, 0, 0)).toHaveCSS('white-space', 'normal');
  });

  test('AutoSum seeds =SUM( in the formula bar and commits the sum', async ({ page }) => {
    const padId = `xl-autosum-${Date.now()}`;
    await openSheet(page, padId);
    await commitCell(page, 0, 0, '2'); // A1
    await commitCell(page, 1, 0, '3'); // A2

    await cell(page, 3, 0).click(); // A4
    await page.locator('.sheet-toolbar button[title="AutoSum"]').click();

    const fx = page.locator('.sheet-fx-input');
    await expect(fx).toBeFocused();
    await expect(fx).toHaveValue(/^=SUM\(/);

    // The caret sits inside the parens of '=SUM()' -> typing produces '=SUM(A1:A2)'.
    await page.keyboard.type('A1:A2');
    await page.keyboard.press('Enter');

    await expect(cell(page, 3, 0)).toHaveText('5');
  });

  test('formula-bar cancel reverts, commit applies', async ({ page }) => {
    const padId = `xl-fx-btns-${Date.now()}`;
    await openSheet(page, padId);
    await commitCell(page, 0, 0, 'alt'); // A1

    await cell(page, 0, 0).click();
    const fx = page.locator('.sheet-fx-input');
    await fx.click();
    await fx.fill('neu');
    await page.locator('.sheet-fx-cancel').click();
    await expect(cell(page, 0, 0)).toHaveText('alt');

    await fx.click();
    await fx.fill('neu2');
    await page.locator('.sheet-fx-commit').click();
    await expect(cell(page, 0, 0)).toHaveText('neu2');
  });

  test('statusbar shows average, count, min, max, and sum for a numeric selection', async ({ page }) => {
    const padId = `xl-stats-${Date.now()}`;
    await openSheet(page, padId);
    await commitCell(page, 0, 0, '1'); // A1
    await commitCell(page, 1, 0, '2'); // A2
    await commitCell(page, 2, 0, '3'); // A3

    await dragSelect(page, 0, 0, 2, 0); // A1:A3

    const stats = page.locator('.sheet-statusbar .sheet-stats');
    await expect(stats).toContainText('Average: 2');
    await expect(stats).toContainText('Count: 3');
    await expect(stats).toContainText('Min: 1');
    await expect(stats).toContainText('Max: 3');
    await expect(stats).toContainText('Sum: 6');
  });

  test('ribbon Copy/Paste buttons roundtrip a cell', async ({ browser, browserName }) => {
    // Firefox rejects clipboard-read/write permission grants at context
    // creation (see sheet_selection.spec.ts).
    test.skip(browserName === 'firefox', 'Firefox does not support clipboard-read/write permission grants in Playwright');

    const ctx = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
    const page = await ctx.newPage();
    const padId = `xl-clip-${Date.now()}`;
    await openSheet(page, padId);
    await commitCell(page, 0, 0, 'x'); // A1

    await cell(page, 0, 0).click();
    await page.locator('.sheet-toolbar button[title="Copy"]').click();
    await cell(page, 0, 1).click(); // B1
    await page.locator('.sheet-toolbar button[title="Paste"]').click();

    await expect(cell(page, 0, 1)).toHaveText('x', { timeout: 10000 });
    await ctx.close();
  });

  test('selected cell highlights its row and column headers', async ({ page }) => {
    const padId = `xl-headhl-${Date.now()}`;
    await openSheet(page, padId);

    await cell(page, 1, 1).click(); // B2

    // thead: corner th (0) + 'A' (1) + 'B' (2); row header is the tr's th.
    await expect(page.locator('.sheet-grid thead th').nth(2)).toHaveClass(/sheet-head-hl/);
    await expect(page.locator('.sheet-grid tbody tr:nth-child(2) th')).toHaveClass(/sheet-head-hl/);
  });
});
