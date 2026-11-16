import {expect, test} from '@playwright/test';

// etherpad-lite #8486: with enableDarkMode (the default) the home page
// follows the browser's colour scheme, unless light was chosen in a pad.
const light = 'rgb(247, 249, 251)';
const dark = 'rgb(22, 27, 34)';

test.describe('home page dark mode', () => {
  test('follows changes to the colour scheme without reloading', async ({page}) => {
    await page.emulateMedia({colorScheme: 'light'});
    await page.goto('/');
    const body = page.locator('body');
    await expect(body).toHaveCSS('background-color', light);
    await page.emulateMedia({colorScheme: 'dark'});
    await expect(body).toHaveCSS('background-color', dark);
    await page.emulateMedia({colorScheme: 'light'});
    await expect(body).toHaveCSS('background-color', light);
  });

  test('honours the light mode chosen in a pad', async ({page}) => {
    await page.emulateMedia({colorScheme: 'dark'});
    await page.addInitScript(() => localStorage.setItem('ep_darkMode', 'false'));
    await page.goto('/');
    await expect(page.locator('body')).toHaveCSS('background-color', light);
  });
});
