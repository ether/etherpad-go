import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';

// The spreadsheet on a small Android phone (360×640).
test.describe('sheet on a phone', () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 640 });
    await page.goto(`/s/mobile_${randomUUID()}`);
    await page.locator('.sheet-grid').waitFor({ state: 'visible', timeout: 20000 });
  });

  test('the page does not scroll sideways and all ribbon tabs are on screen', async ({ page }) => {
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360);
    const tabs = await page.evaluate(() =>
      [...document.querySelectorAll('.sheet-ribbon-tabs button')]
        .filter((b) => (b as HTMLElement).offsetParent !== null)
        .map((b) => ({ text: b.textContent?.trim() ?? '', right: b.getBoundingClientRect().right })));
    expect(tabs.length).toBeGreaterThanOrEqual(5);
    for (const t of tabs) expect(t.right, t.text).toBeLessThanOrEqual(360);
  });

  test('the ribbon scrolls sideways inside itself', async ({ page }) => {
    const body = page.locator('.sheet-ribbon-body');
    const { scroll, client } = await body.evaluate((el) => ({ scroll: el.scrollWidth, client: el.clientWidth }));
    expect(scroll).toBeGreaterThan(client);
    await body.evaluate((el) => { el.scrollLeft = 200; });
    expect(await body.evaluate((el) => el.scrollLeft)).toBeGreaterThan(0);
  });

  test('the row headers stay narrow and the formula input does not trigger zoom', async ({ page }) => {
    const header = await page.locator('.sheet-grid tbody th').first().boundingBox();
    expect(header).not.toBeNull();
    if (header) expect(header.width).toBeLessThanOrEqual(50);
    expect(await page.locator('.sheet-fx-input').evaluate((el) => getComputedStyle(el).fontSize)).toBe('16px');
  });
});
