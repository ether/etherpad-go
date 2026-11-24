import {expect, Locator, test} from "@playwright/test";
import {goToNewPad, sendChatMessage, setEpCheckbox, showChat} from "../helper/padHelper";

test.beforeEach(async ({page}) => {
    await goToNewPad(page);
})

// Computed background colour, independent of how the style is written.
const bg = (l: Locator) => l.evaluate((el) => getComputedStyle(el).backgroundColor);

test.describe('change user color', function () {

    test('Color picker matches original color and remembers the user color after a refresh',
        async function ({page}) {

            // click on the settings button to make settings visible
            let $userButton = page.locator('.buttonicon-showusers');
            await $userButton.click()

            let $userSwatch = page.locator('#userlist .color-btn');
            await $userSwatch.click()
            // Change the color value of the Farbtastic color picker

            const $colorPickerSave = page.locator('#mycolorpickersave');
            let $colorPickerPreview = page.locator('#mycolorpickerpreview');

            // Same color represented in two different ways
            const testColorHash = '#abcdef';
            const testColorRGB = 'rgb(171, 205, 239)';

            // Check that the color picker matches the automatically assigned random color on the swatch.
            // NOTE: This has a tiny chance of creating a false positive for passing in the
            // off-chance the randomly assigned color is the same as the test color.
            expect(await bg($colorPickerPreview)).toBe(await bg($userSwatch));

            // The swatch updates as the test color is picked.
            await page.evaluate((testRGBColor) => {
                document.getElementById('mycolorpickerpreview')!.style.backgroundColor = testRGBColor;
            }, testColorRGB
            )

            await $colorPickerSave.click();

            // give it a second to save the color on the server side
            await page.waitForTimeout(1000)


            // get a new pad, but don't clear the cookies
            await goToNewPad(page)


            // click on the settings button to make settings visible
            await $userButton.click()

            await $userSwatch.click()



            expect(await bg($colorPickerPreview)).toBe(await bg($userSwatch));
        });

    test('Own user color is shown when you enter a chat', async function ({page}) {

        const colorOption = page.locator('#options-colorscheck');
        await setEpCheckbox(colorOption, true);

        // click on the settings button to make settings visible
        const $userButton = page.locator('.buttonicon-showusers');
        await $userButton.click()

        const $userSwatch = page.locator('#userlist .color-btn');
        await $userSwatch.click()

        const $colorPickerSave = page.locator('#mycolorpickersave');

        // Same color represented in two different ways
        const testColorHash = '#abcdef';
        const testColorRGB = 'rgb(171, 205, 239)';

        // The swatch updates as the test color is picked.
        await page.evaluate((testRGBColor) => {
                document.getElementById('mycolorpickerpreview')!.style.backgroundColor = testRGBColor;
            }, testColorRGB
        )


        await $colorPickerSave.click();
        // click on the chat button to make chat visible
        await showChat(page)
        await sendChatMessage(page, 'O hi');

        // wait until the chat message shows up — chat now renders as
        // <ep-chat-message> webcomponents rather than <p> elements.
        const chatMsg = page.locator('#chattext').locator('ep-chat-message').first()
        await expect(chatMsg).toBeVisible({timeout: 10000});
        const chatText = (await chatMsg.textContent()) ?? '';

        expect(chatText).toContain('O hi');

        // The author color is rendered inside the shadow DOM on the
        // `.author` span via inline `color: ${authorColor}`. Read it from
        // the shadow root.
        const authorColor = await chatMsg.evaluate((el) => {
            const span = el.shadowRoot?.querySelector('.author') as HTMLElement | null;
            if (!span) return '';
            return window.getComputedStyle(span).getPropertyValue('color');
        });

        expect(authorColor).toBe(testColorRGB);
    });
});
