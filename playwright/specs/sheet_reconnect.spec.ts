import { test, expect, type Page, type WebSocketRoute } from '@playwright/test';

// A spreadsheet keeps the edits of a connection that drops. The WebSocket is
// routed through the test, which drops one message at the right moment and
// cuts the connection; the page reconnects on its own.

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

// cutter routes the page's socket. Once armed, the first message matching
// the given direction and type is swallowed and the connection is closed.
async function cutter(page: Page) {
  let armed: { dir: 'up' | 'down'; type: string } | null = null;
  let cuts = 0;
  await page.routeWebSocket(/\/socket\.io\//, (ws: WebSocketRoute) => {
    const server = ws.connectToServer();
    const maybeCut = (dir: 'up' | 'down', message: string | Buffer): boolean => {
      if (!armed || armed.dir !== dir || !String(message).includes(`"${armed.type}"`)) return false;
      armed = null;
      cuts++;
      ws.close();
      server.close();
      return true;
    };
    ws.onMessage((m) => { if (!maybeCut('up', m)) server.send(m); });
    server.onMessage((m) => { if (!maybeCut('down', m)) ws.send(m); });
  });
  return {
    arm: (dir: 'up' | 'down', type: string) => { armed = { dir, type }; },
    cuts: () => cuts,
  };
}

test.describe('sheet reconnect', () => {
  test('an edit lost with the connection, and edits made offline, are saved', async ({ page, browser }) => {
    const padId = `sheet-reconnect-lost-${Date.now()}`;
    const net = await cutter(page);
    await openSheet(page, padId);
    await typeInto(page, 0, 0, 'before');

    // The next op never reaches the server: the connection drops with it.
    net.arm('up', 'SHEET_OP');
    await typeInto(page, 1, 0, 'lost op');
    await expect.poll(net.cuts).toBe(1);
    await expect(page.locator('.sheet-connection')).toContainText('Reconnecting');
    // Typed while offline.
    await typeInto(page, 2, 0, 'offline');
    await expect(page.locator('.sheet-connection')).toHaveText('Ready', { timeout: 15000 });

    const other = await browser.newContext();
    try {
      const page2 = await other.newPage();
      await openSheet(page2, padId);
      await expect(cell(page2, 0, 0)).toHaveText('before');
      await expect(cell(page2, 1, 0)).toHaveText('lost op');
      await expect(cell(page2, 2, 0)).toHaveText('offline');
    } finally {
      await other.close();
    }
  });

  test('an op whose acknowledgement was lost is not applied twice', async ({ page, browser }) => {
    const padId = `sheet-reconnect-ack-${Date.now()}`;
    const net = await cutter(page);
    await openSheet(page, padId);
    await typeInto(page, 0, 0, 'first');
    await typeInto(page, 1, 0, 'second');

    // The server stores the row insert, but its acknowledgement is lost.
    net.arm('down', 'ACCEPT_SHEET_OP');
    await cell(page, 0, 0).click();
    await page.locator('.sheet-toolbar button[title="Insert row above"]').click();
    await expect.poll(net.cuts).toBe(1);
    await expect(page.locator('.sheet-connection')).toHaveText('Ready', { timeout: 15000 });
    await typeInto(page, 3, 0, 'after');

    const expected = ['', 'first', 'second', 'after'];
    for (const [r, text] of expected.entries()) await expect(cell(page, r, 0)).toHaveText(text);
    const other = await browser.newContext();
    try {
      const page2 = await other.newPage();
      await openSheet(page2, padId);
      // One empty row on top, not two.
      for (const [r, text] of expected.entries()) await expect(cell(page2, r, 0)).toHaveText(text);
    } finally {
      await other.close();
    }
  });
});
