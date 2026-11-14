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
        await page.locator('#theme-switcher').click();
        await expect.poll(() => toolbarIsDark(page)).toBe(false);
        expect(await themeColors(page)).toEqual([{content: '#ffffff', media: null}]);
    });
});

test.describe('light OS', () => {
    test.use({colorScheme: 'light'});

    test('dark chosen in the settings: the theme-color follows', async ({page}) => {
        await goToNewPad(page);
        expect(await toolbarIsDark(page)).toBe(false);
        await showSettings(page);
        await page.locator('#theme-switcher').click();
        await expect.poll(() => toolbarIsDark(page)).toBe(true);
        const metas = await themeColors(page);
        expect(metas).toHaveLength(1);
        expect(metas[0].media).toBeNull();
        expect(metas[0].content).not.toBe('#ffffff');

        await showSettings(page);
        await page.locator('#theme-switcher').click();
        await expect.poll(() => toolbarIsDark(page)).toBe(false);
        expect(await themeColors(page)).toEqual([{content: '#ffffff', media: null}]);
    });
});
