import { test, expect, type Page } from '@playwright/test';

// 0-based cell locator. Row header is th (child 1), so data column c is
// td:nth-child(c + 2); data row r is tbody tr:nth-child(r + 1).
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

// Drags a mouse-based selection from (r0,c0) to (r1,c1) via real mouse events
// (mousedown -> mouseover -> mouseup), matching DomSheetView's listeners in
// ui/src/js/sheet/sheetView.ts (drag state is tracked on mousedown/mouseover,
// committed on a document-level mouseup).
async function dragSelect(page: Page, r0: number, c0: number, r1: number, c1: number): Promise<void> {
  await cell(page, r0, c0).hover();
  await page.mouse.down();
  await cell(page, r1, c1).hover();
  await page.mouse.up();
}

// Drags the fill handle of the current selection's bottom-right cell to
// (r,c). The handle's mousedown handler calls stopPropagation so it doesn't
// restart a plain selection drag; mouseover on target cells updates the fill
// target, and the document-level mouseup commits it via onFill.
async function dragFillHandleTo(page: Page, r: number, c: number): Promise<void> {
  const handle = page.locator('.sheet-fill-handle');
  await handle.hover();
  await page.mouse.down();
  await cell(page, r, c).hover();
  await page.mouse.up();
}

test.describe('Sheet selection, fill, and clipboard', () => {
  test('drag selection highlights only the in-range cells', async ({ page }) => {
    const padId = `sheet-sel-${Date.now()}`;
    await openSheet(page, padId);

    // Drag a 2x2 block from A1 to B2.
    await dragSelect(page, 0, 0, 1, 1);

    await expect(cell(page, 0, 0)).toHaveClass(/sheet-sel/);
    await expect(cell(page, 0, 1)).toHaveClass(/sheet-sel/);
    await expect(cell(page, 1, 0)).toHaveClass(/sheet-sel/);
    await expect(cell(page, 1, 1)).toHaveClass(/sheet-sel/);

    // Out-of-range neighbors must not be highlighted.
    await expect(cell(page, 0, 2)).not.toHaveClass(/sheet-sel/);
    await expect(cell(page, 2, 0)).not.toHaveClass(/sheet-sel/);
    await expect(cell(page, 2, 2)).not.toHaveClass(/sheet-sel/);
  });

  test('fill down adjusts a relative formula reference', async ({ page }) => {
    const padId = `sheet-fill-${Date.now()}`;
    await openSheet(page, padId);

    await commitCell(page, 0, 0, '10');      // A1 = 10
    await commitCell(page, 0, 1, '=A1*2');   // B1 = =A1*2
    await expect(cell(page, 0, 1)).toHaveText('20');
    await commitCell(page, 1, 0, '5');       // A2 = 5

    // Select B1, then drag its fill handle down to B2.
    await cell(page, 0, 1).click();
    await dragFillHandleTo(page, 1, 1);

    // B2 should now be =A2*2 = 10 (relative ref shifted down one row).
    await expect(cell(page, 1, 1)).toHaveText('10');
  });

  test('bottom-up drag selection then fill covers the full source rect plus extension', async ({ page }) => {
    const padId = `sheet-fill-bottomup-${Date.now()}`;
    await openSheet(page, padId);

    // Seed a 2-row source column C: C1=1, C2=2.
    await commitCell(page, 0, 2, '1'); // C1
    await commitCell(page, 1, 2, '2'); // C2

    // Select bottom-up: mousedown on the LOWER cell (C2), drag to the UPPER
    // cell (C1). anchor = C2 (bottom), focus = C1 (top) — this is the path
    // that previously mis-anchored the fill target (see commit c71b66c).
    await dragSelect(page, 1, 2, 0, 2);
    await expect(cell(page, 0, 2)).toHaveClass(/sheet-sel/);
    await expect(cell(page, 1, 2)).toHaveClass(/sheet-sel/);

    // Now drag the fill handle further down to row 3 (0-based row index 2).
    // The fill handle sits at the selection's bottom-right cell regardless of
    // anchor/focus order, so this drags from the visual bottom of the 2-cell
    // selection down one more row.
    await dragFillHandleTo(page, 2, 2);

    // The filled range must cover the FULL original source rectangle
    // (rows 0-1, untouched) plus the extension (row 2). fillOps tiles the
    // source pattern modulo its height (see sheetClipboard.ts fillOps), so
    // row 2 wraps back to source row 0's value ('1'), not row 1's.
    await expect(cell(page, 0, 2)).toHaveText('1');
    await expect(cell(page, 1, 2)).toHaveText('2');
    await expect(cell(page, 2, 2)).toHaveText('1');
  });

  test.describe('clipboard', () => {
    test.use({ permissions: ['clipboard-read', 'clipboard-write'] });

    test('copy and paste a TSV range', async ({ page, browserName }) => {
      // Firefox's Playwright build does not support granting clipboard-read/
      // clipboard-write permissions, so the async Clipboard API calls in
      // sheetEditor.ts's Ctrl+C/Ctrl+V handlers silently no-op there. The fill
      // test above already proves the underlying op pipeline (setCell via
      // collab.applyLocal) without depending on the OS clipboard, so this is
      // a real-browser-support gap, not an untested code path.
      test.fixme(browserName === 'firefox', 'Firefox does not support clipboard-read/write permission grants in Playwright');

      const padId = `sheet-clip-${Date.now()}`;
      await openSheet(page, padId);

      await commitCell(page, 0, 0, 'foo'); // A1
      await commitCell(page, 0, 1, 'bar'); // B1

      // Select A1:B1 and copy.
      await dragSelect(page, 0, 0, 0, 1);
      await page.keyboard.press('Control+C');

      // Select A3 and paste.
      await cell(page, 2, 0).click();
      await page.keyboard.press('Control+V');

      await expect(cell(page, 2, 0)).toHaveText('foo', { timeout: 10000 });
      await expect(cell(page, 2, 1)).toHaveText('bar', { timeout: 10000 });
    });
  });
});
