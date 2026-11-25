import {expect, Page, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

// With dark mode on, the server sends a theme-color for a light and one for
// a dark OS. Once the page knows which toolbar it shows, the address bar
// must follow that toolbar, not the OS (etherpad-lite #7606, #7690).

const themeColors = (page: Page) => page.evaluate(() =>
    Array.from(document.querySelectorAll('meta[name="theme-color"]'))
        .map((m) => ({content: m.getAttribute('content'), media: m.getAttribute('media')})));

const toolbarIsDark = (page: Page) => page.evaluate(() =>
    document.documentElement.classList.contains('super-dark-toolbar'));

test.describe('dark OS', () => {
    test.use({colorScheme: 'dark'});

    test('light chosen: one light theme-color, the dark one is gone', async ({page}) => {
        await page.addInitScript(() => localStorage.setItem('ep_darkMode', 'false'));
        await goToNewPad(page);
        expect(await toolbarIsDark(page)).toBe(false);
        const metas = await themeColors(page);
        expect(metas).toHaveLength(1);
        expect(metas[0].media).toBeNull();
        expect(metas[0].content).toBe('#ffffff');
    });

    test('no choice: the dark toolbar and a dark theme-color', async ({page}) => {
        await goToNewPad(page);
        expect(await toolbarIsDark(page)).toBe(true);
        const metas = await themeColors(page);
        expect(metas).toHaveLength(1);
        expect(metas[0].media).toBeNull();
        expect(metas[0].content).not.toBe('#ffffff');

        await showSettings(page);
        await expect(page.locator('#theme-switcher [data-theme="dark"]')).toHaveAttribute('aria-pressed', 'true');
        await page.locator('#theme-switcher [data-theme="light"]').click();
        await expect.poll(() => toolbarIsDark(page)).toBe(false);
        await expect(page.locator('#theme-switcher [data-theme="light"]')).toHaveAttribute('aria-pressed', 'true');
        expect(await themeColors(page)).toEqual([{content: '#ffffff', media: null}]);
    });
});

test.describe('light OS', () => {
    test.use({colorScheme: 'light'});

    test('dark chosen in the settings: the theme-color follows', async ({page}) => {
        await goToNewPad(page);
        expect(await toolbarIsDark(page)).toBe(false);
        await showSettings(page);
        await page.locator('#theme-switcher [data-theme="dark"]').click();
        await expect.poll(() => toolbarIsDark(page)).toBe(true);
        const metas = await themeColors(page);
        expect(metas).toHaveLength(1);
        expect(metas[0].media).toBeNull();
        expect(metas[0].content).not.toBe('#ffffff');

        // The choice survives a reload (also the one after a language change).
        await page.reload();
        await page.waitForSelector('#editorcontainer.initialized');
        expect(await toolbarIsDark(page)).toBe(true);

        await showSettings(page);
        await page.locator('#theme-switcher [data-theme="light"]').click();
        await expect.poll(() => toolbarIsDark(page)).toBe(false);
        expect(await themeColors(page)).toEqual([{content: '#ffffff', media: null}]);
    });
});

// Without storage (blocked, some embedded frames) the pad still starts and
// the theme switch still works, it is only not kept.
test('no storage: the pad starts and the switch works', async ({page}) => {
    await page.addInitScript(() => {
        const fail = () => { throw new DOMException('denied', 'SecurityError'); };
        Storage.prototype.getItem = fail;
        Storage.prototype.setItem = fail;
    });
    await goToNewPad(page);
    await showSettings(page);
    await page.locator('#theme-switcher [data-theme="dark"]').click();
    await expect.poll(() => toolbarIsDark(page)).toBe(true);
});

// Safari on iOS 26 ignores theme-color and tints the area around the notch /
// Dynamic Island with the page background, so that follows the toolbar too.
// The home-screen app's status bar style follows it as well.
test.describe('page background behind the iOS status bar', () => {
    test.use({colorScheme: 'light'});

    const chrome = (page: Page) => page.evaluate(() => ({
        page: getComputedStyle(document.documentElement).backgroundColor,
        toolbar: getComputedStyle(document.getElementById('editbar')!).backgroundColor,
        statusBar: document.querySelector('meta[name="apple-mobile-web-app-status-bar-style"]')?.getAttribute('content'),
    }));

    test('matches the toolbar, light and dark', async ({page}) => {
        await goToNewPad(page);
        const light = await chrome(page);
        expect(light.page).toBe(light.toolbar);
        expect(light.statusBar).toBe('default');

        await showSettings(page);
        await page.locator('#theme-switcher [data-theme="dark"]').click();
        await expect.poll(() => toolbarIsDark(page)).toBe(true);
        const dark = await chrome(page);
        expect(dark.page).toBe(dark.toolbar);
        expect(dark.page).not.toBe(light.page);
        expect(dark.statusBar).toBe('black-translucent');
    });
});
