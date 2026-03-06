import {expect, Page, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

test.beforeEach(async ({page}) => {
    await goToNewPad(page);
});

const waitForQrImage = async (page: Page) => {
    await page.waitForFunction(() => {
        const image = document.getElementById('qrcodeimg');
        return image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0;
    });
};

test.describe('QR share popup', () => {
    test('is hidden on initial load', async ({page}) => {
        const qrPopup = page.locator('#share_qr');
        await expect(qrPopup).not.toHaveClass(/popup-show/);
        await expect(qrPopup).toHaveCSS('visibility', 'hidden');
        await expect(qrPopup).toHaveCSS('opacity', '0');
    });

    test('renders a large QR code from toolbar button', async ({page}) => {
        const qrButton = page.locator('li[data-key="share_qr"] button');
        await qrButton.click();

        const qrPopup = page.locator('#share_qr');
        const qrImage = page.locator('#qrcodeimg');
        const qrLinkInput = page.locator('#qrcodelinkinput');

        await expect(qrPopup).toHaveClass(/popup-show/);
        const expectedLink = page.url().split('?')[0];
        await expect(qrImage).toHaveAttribute('src', `${expectedLink}/qr?readonly=false`);
        await waitForQrImage(page);
        await expect(qrLinkInput).toHaveValue(expectedLink);
    });

    test('switches QR target to read-only link', async ({page}) => {
        const qrButton = page.locator('li[data-key="share_qr"] button');
        await qrButton.click();

        const readOnlyId = await page.evaluate(() => (window as any).clientVars.readOnlyId);
        const qrReadonlyToggle = page.locator('#qrreadonlyinput');
        const qrLinkInput = page.locator('#qrcodelinkinput');
        const qrImage = page.locator('#qrcodeimg');

        await qrReadonlyToggle.evaluate((element: HTMLInputElement) => {
            element.checked = true;
            element.dispatchEvent(new Event('click', {bubbles: true}));
        });
        await expect(qrLinkInput).toHaveValue(new RegExp(`/${readOnlyId}$`));
        await expect(qrImage).toHaveAttribute('src', `${page.url().split('?')[0]}/qr?readonly=true`);
        await waitForQrImage(page);
    });

    test('keeps the QR link footer inside the dialog bounds', async ({page}) => {
        const qrButton = page.locator('li[data-key="share_qr"] button');
        await qrButton.click();
        await waitForQrImage(page);

        const popupBox = await page.locator('#share_qr .popup-content').boundingBox();
        await expect(page.locator('#qrcodefooter')).toBeVisible();
        await expect(page.locator('#qrcodelinkinput')).toBeVisible();
        const hasHorizontalOverflow = await page.evaluate(() => {
            return document.documentElement.scrollWidth > window.innerWidth;
        });

        expect(popupBox).not.toBeNull();
        expect(hasHorizontalOverflow).toBe(false);
        expect(popupBox!.width).toBeGreaterThan(0);
    });

    test('closes the QR popup when clicking the backdrop', async ({page}) => {
        const qrButton = page.locator('li[data-key="share_qr"] button');
        const qrPopup = page.locator('#share_qr');
        await qrButton.click();

        await expect(qrPopup).toHaveClass(/popup-show/);
        await qrPopup.click({position: {x: 10, y: 10}});
        await expect(qrPopup).not.toHaveClass(/popup-show/);
    });
});
