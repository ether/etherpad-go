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

// The head revision once the server has stored n changes.
async function waitForHead(page: Page, pad: string, n: number): Promise<void> {
  await expect.poll(async () => (await (await page.request.get(`/s/${pad}/history`)).json()).head).toBe(n);
}

test.describe('Sheet version history', () => {
  test('lists versions, previews and restores them for everyone', async ({ page, browser }) => {
    const pad = `history-${Date.now()}`;
    await openSheet(page, pad);
    await typeInto(page, 0, 0, 'Monat');
    await typeInto(page, 1, 0, 'Jan');
    await typeInto(page, 0, 1, '=LEN(A1)');
    await waitForHead(page, pad, 3);

    const hist = await (await page.request.get(`/s/${pad}/history`)).json();
    expect(hist.available).toBe(true);
    expect(hist.revisions.map((r: { rev: number }) => r.rev)).toEqual([1, 2, 3]);
    const at1 = await (await page.request.get(`/s/${pad}/history/1`)).json();
    expect(at1.snapshot.sheets[0].cells.map((c: { raw: string }) => c.raw)).toEqual(['Monat']);
    expect((await page.request.get(`/s/${pad}/history/99`)).status()).toBe(404);

    // The panel: the current version first, the original last.
    await page.locator('.sheet-ribbon-tabs button', { hasText: /^View$/ }).click();
    await page.locator('[title^="Version history"]').click();
    const panel = page.locator('.sheet-history');
    const items = panel.locator('.sheet-history-list button');
    await expect(items).toHaveCount(2);
    await expect(items.first()).toContainText('Current version');
    await expect(items.first().locator('.sheet-history-who span')).toHaveCount(1);
    await expect(items.last()).toContainText('Original version');
    // The current version is shown, computed, and cannot be "restored".
    const preview = panel.locator('.sheet-history-sheet');
    await expect(preview.locator('td').first()).toHaveText('Monat');
    await expect(preview.locator('td').nth(1)).toHaveText('5');
    await expect(panel.locator('.sheet-history-restore')).toBeHidden();

    await items.last().click();
    await expect(preview).toContainText('This sheet is empty.');
    await expect(panel.locator('.sheet-history-restore')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(panel).toHaveCount(0);

    // Restore an older version: it arrives like anyone's edits.
    const other = await browser.newPage();
    await openSheet(other, pad);
    await expect(cellAt(other, 1, 0)).toHaveText('Jan');
    const res = await page.request.post(`/s/${pad}/history/1/restore`);
    expect(res.ok()).toBe(true);
    for (const p of [page, other]) {
      await expect(cellAt(p, 1, 0)).toHaveText('');
      await expect(cellAt(p, 0, 1)).toHaveText('');
      await expect(cellAt(p, 0, 0)).toHaveText('Monat');
    }
    // The versions in between are kept.
    const after = await (await page.request.get(`/s/${pad}/history`)).json();
    expect(after.head).toBeGreaterThan(3);
    const at3 = await (await page.request.get(`/s/${pad}/history/3`)).json();
    expect(at3.snapshot.sheets[0].cells).toHaveLength(3);

    // Restore through the panel: back to the original, empty sheet.
    await page.locator('[title^="Version history"]').click();
    await items.last().click();
    await panel.locator('.sheet-history-restore').click();
    await page.locator('.sheet-dialog button.primary').click();
    await expect(panel).toHaveCount(0);
    for (const p of [page, other]) await expect(cellAt(p, 0, 0)).toHaveText('');
    await expect(page.locator('.sheet-flash')).toContainText('Restored the version of Original version');

    // Undo-able by restoring again.
    await page.request.post(`/s/${pad}/history/3/restore`);
    await expect(cellAt(other, 0, 1)).toHaveText('5');
    await other.close();
  });

  test('the open panel follows new changes; other sites cannot restore', async ({ page, browser }) => {
    const pad = `history-live-${Date.now()}`;
    await openSheet(page, pad);
    await typeInto(page, 0, 0, 'first');
    await waitForHead(page, pad, 1);
    await page.locator('.sheet-ribbon-tabs button', { hasText: /^View$/ }).click();
    await page.locator('[title^="Version history"]').click();
    const panel = page.locator('.sheet-history');
    const preview = panel.locator('.sheet-history-sheet');
    await expect(preview.locator('td').first()).toHaveText('first');

    // Someone else edits: the current version shown moves on.
    const other = await browser.newPage();
    await openSheet(other, pad);
    await typeInto(other, 0, 1, 'second');
    await waitForHead(page, pad, 2);
    // Back in front: background tabs get their timers throttled.
    await page.bringToFront();
    // The panel reads the versions again a moment after the changes stop.
    await expect(preview.locator('td').nth(1)).toHaveText('second', { timeout: 15000 });
    await expect(panel.locator('.sheet-history-list button').first()).toContainText('Current version');
    await expect(panel.locator('.sheet-history-restore')).toBeHidden();

    // An older version stays chosen while the sheet changes, and can be
    // restored.
    await panel.locator('.sheet-history-list button').last().click();
    await expect(preview).toContainText('This sheet is empty.');
    await typeInto(other, 1, 0, 'third');
    await expect(panel.locator('.sheet-history-list button').first().locator('.sheet-history-who span')).toHaveCount(2, { timeout: 15000 });
    await expect(panel.locator('.sheet-history-list button.on')).toContainText('Original version');
    await expect(panel.locator('.sheet-history-restore')).toBeVisible();
    await other.close();

    // A form or fetch on another site carries the cookies, but is refused.
    const foreign: Array<Record<string, string>> = [{ 'Sec-Fetch-Site': 'cross-site' }, { Origin: 'https://evil.example' }];
    for (const headers of foreign) {
      expect((await page.request.post(`/s/${pad}/history/0/restore`, { headers })).status()).toBe(403);
      expect((await page.request.post(`/s/${pad}/import`, { headers })).status()).toBe(403);
    }
    await expect(cellAt(page, 1, 0)).toHaveText('third');
  });

  test('read-only viewers see the history but cannot restore', async ({ page, browser }) => {
    const pad = `history-ro-${Date.now()}`;
    await openSheet(page, pad);
    await typeInto(page, 0, 0, 'x');
    await waitForHead(page, pad, 1);
    // The read-only id, from the pad page's client vars.
    const probe = await browser.newPage();
    await probe.goto(`/p/${pad}`);
    const roId = await probe.waitForFunction(() => (window as unknown as { clientVars?: { readOnlyId?: string } }).clientVars?.readOnlyId)
      .then((h) => h.jsonValue() as Promise<string>);
    await probe.close();
    const viewer = await browser.newPage();
    await openSheet(viewer, roId);
    await viewer.locator('.sheet-ribbon-tabs button', { hasText: /^View$/ }).click();
    await viewer.locator('[title^="Version history"]').click();
    const items = viewer.locator('.sheet-history-list button');
    await expect(items).toHaveCount(2);
    await items.last().click();
    await expect(viewer.locator('.sheet-history-sheet')).toContainText('This sheet is empty.');
    await expect(viewer.locator('.sheet-history-restore')).toBeHidden();
    expect((await viewer.request.post(`/s/${roId}/history/0/restore`)).status()).toBe(403);
    // The read-only id reads the sheet's history, not an empty one.
    expect((await (await viewer.request.get(`/s/${roId}/history`)).json()).head).toBe(1);
    await viewer.close();
  });
});

test.describe('Sheet printing', () => {
  test('prints the sheet, the selection or the workbook', async ({ page }) => {
    // The real print dialog is modal (Firefox blocks the page under it):
    // count the calls instead. Init scripts also run in the print frame.
    await page.addInitScript(() => {
      window.print = () => {
        const top = window.top as unknown as { printed?: number };
        top.printed = (top.printed ?? 0) + 1;
      };
    });
    const printed = () => page.evaluate(() => (window as unknown as { printed?: number }).printed ?? 0);
    const pad = `print-${Date.now()}`;
    await openSheet(page, pad);
    await typeInto(page, 0, 0, 'Item');
    await typeInto(page, 0, 1, 'Price');
    await typeInto(page, 1, 0, '<b>Tea</b>');
    await typeInto(page, 1, 1, '=2*3');
    // Bold header.
    await cellAt(page, 0, 0).click();
    await page.keyboard.press('Control+b');

    const frameDoc = async (): Promise<string> =>
      (await page.locator('iframe.sheet-print-frame').getAttribute('srcdoc')) ?? '';

    await cellAt(page, 0, 0).click();
    await page.keyboard.press('Control+p');
    await expect(page.locator('.sheet-dialog')).toHaveAttribute('aria-label', 'Print');
    await expect(page.locator('#sheet-dlg-scope')).toHaveValue('sheet');
    await page.locator('.sheet-dialog button.primary').click();
    await expect.poll(frameDoc).toContain('Price');
    await expect.poll(printed).toBe(1);
    let doc = await frameDoc();
    expect(doc).toContain('&lt;b&gt;Tea&lt;/b&gt;');
    expect(doc).toMatch(/<td style="text-align:right">6<\/td>/);
    expect(doc).toMatch(/<td style="font-weight:bold[^"]*">Item<\/td>/);
    expect(doc).toContain('<div class="sheet-render" data-sheet="s1">');
    const frame = page.frameLocator('iframe.sheet-print-frame');
    await expect(frame.locator('td')).toHaveCount(4);

    // A selection, without gridlines.
    await cellAt(page, 1, 0).click();
    await page.keyboard.press('Shift+ArrowRight');
    await page.locator('.sheet-file-tab').click();
    await page.locator('.sheet-file-menu button', { hasText: 'Print' }).click();
    await expect(page.locator('#sheet-dlg-scope')).toHaveValue('selection');
    await page.locator('#sheet-dlg-grid').selectOption('no');
    await page.locator('.sheet-dialog button.primary').click();
    await expect.poll(frameDoc).not.toContain('Price');
    await expect.poll(printed).toBe(2);
    doc = await frameDoc();
    expect(doc).toContain('<div class="sheet-render sheet-render-nogrid"');
    await expect(page.locator('iframe.sheet-print-frame')).toHaveCount(1);

    // The whole workbook: every sheet with its name.
    await page.locator('.sheet-tab-add, [title="New sheet"]').first().click();
    await typeInto(page, 0, 0, 'second');
    await page.keyboard.press('Control+p');
    await page.locator('#sheet-dlg-scope').selectOption('workbook');
    await page.locator('.sheet-dialog button.primary').click();
    await expect.poll(frameDoc).toContain('second');
    await expect.poll(printed).toBe(3);
    doc = await frameDoc();
    expect(doc.match(/class="sheet-render-caption"/g)).toHaveLength(2);
    expect(doc).toContain('Price');
  });
});
