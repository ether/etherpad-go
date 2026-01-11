import {expect, test} from "@playwright/test";
import {clearPadContent, getPadBody, goToNewPad, writeToPad} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    await goToNewPad(page);
})

test.describe('entering a URL makes a link', function () {
    const urls = ['https://etherpad.org', 'www.etherpad.org', 'https://www.etherpad.org'];
    for (let i = 0; i < urls.length; i++) {
        const testUrl = urls[i];
        test(`url format ${i}`, async function ({page}) {
            const padBody = await getPadBody(page);
            await clearPadContent(page)
            await writeToPad(page, testUrl);
            // Wait for link to be created by the auto-linker
            await expect(padBody.locator('a')).toBeVisible();
            await expect(padBody.locator('a')).toHaveText(testUrl);
            const expectedHref = testUrl.startsWith('http') ? testUrl : `http://${testUrl}`;
            await expect(padBody.locator('a')).toHaveAttribute('href', expectedHref);
        });
    }
});


test.describe('special characters inside URL', async function () {
    const chars = '-:@_.,~%+/?=&#!;()[]$\'*';
    for (let i = 0; i < chars.length; i++) {
        const char = chars[i];
        const url = `https://etherpad.org/${char}foo`;
        test(`special char ${i}`, async function ({page}) {
            const padBody = await getPadBody(page);
            await clearPadContent(page)
            await padBody.click()
            await clearPadContent(page)
            await writeToPad(page, url);
            await expect(padBody.locator('div').first()).toHaveText(url);
            await expect(padBody.locator('a')).toHaveText(url);
            await expect(padBody.locator('a')).toHaveAttribute('href', url);
        });
    }
});

test.describe('punctuation after URL is ignored', ()=> {
    const chars = ':.,;?!)]\'*';
    for (let i = 0; i < chars.length; i++) {
        const char = chars[i];
        const want = 'https://etherpad.org';
        const input = want + char;
        test(`punctuation char ${i}`, async function ({page}) {
            const padBody = await getPadBody(page);
            await clearPadContent(page)
            await writeToPad(page, input);
            await expect(padBody.locator('a')).toHaveCount(1);
            await expect(padBody.locator('a')).toHaveAttribute('href', want);
        });
    }
});
