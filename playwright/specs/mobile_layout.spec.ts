import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";
import {showSettings} from "../helper/settingsHelper";

test.beforeEach(async ({page}) => {
    await page.setViewportSize({width: 390, height: 844});
    await goToNewPad(page);
});

test.describe('mobile layout', () => {
    test('uses mobile toolbar layout', async ({page}) => {
        await expect(page.locator('.toolbar .menu_right')).toHaveCSS('position', 'fixed');
    });

    test('shows settings popup above bottom toolbar', async ({page}) => {
        await showSettings(page);
        const popup = page.locator('#settings .popup-content');
        const toolbar = page.locator('.toolbar .menu_right');

        await expect(popup).toBeVisible();

        const popupBox = await popup.boundingBox();
        const toolbarBox = await toolbar.boundingBox();
        expect(popupBox).not.toBeNull();
        expect(toolbarBox).not.toBeNull();
        if (popupBox && toolbarBox) {
            expect(popupBox.y + popupBox.height).toBeLessThanOrEqual(toolbarBox.y);
        }
    });

    test('does not overflow horizontally on tiny mobile viewport', async ({page}) => {
        await page.setViewportSize({width: 320, height: 568});
        await goToNewPad(page);
        await showSettings(page);

        const hasOverflow = await page.evaluate(() => {
            return document.documentElement.scrollWidth > window.innerWidth;
        });
        expect(hasOverflow).toBe(false);

        const popup = page.locator('#settings .popup-content');
        const popupBox = await popup.boundingBox();
        expect(popupBox).not.toBeNull();
        if (popupBox) {
            expect(popupBox.width).toBeLessThanOrEqual(320);
        }
    });

    test('keeps popup and bottom toolbar separated in landscape', async ({page}) => {
        await page.setViewportSize({width: 844, height: 390});
        await goToNewPad(page);
        await showSettings(page);

        const popup = page.locator('#settings .popup-content');
        const toolbar = page.locator('.toolbar .menu_right');
        await expect(popup).toBeVisible();

        const popupBox = await popup.boundingBox();
        const toolbarBox = await toolbar.boundingBox();
        expect(popupBox).not.toBeNull();
        expect(toolbarBox).not.toBeNull();
        if (popupBox && toolbarBox) {
            expect(popupBox.y + popupBox.height).toBeLessThanOrEqual(toolbarBox.y + 1);
        }
    });

    test('allows horizontal scrolling in the top toolbar on narrow layouts', async ({page}) => {
        await page.evaluate(() => {
            const menuLeft = document.querySelector('.toolbar .menu_left');
            if (!(menuLeft instanceof HTMLUListElement)) return;

            for (let i = 0; i < 30; i++) {
                const item = document.createElement('li');
                item.dataset.type = 'button';
                item.dataset.key = `mobile-overflow-${i}`;
                item.innerHTML = `<a title="Overflow ${i}" aria-label="Overflow ${i}"><button class="buttonicon" aria-label="Overflow ${i}">${i}</button></a>`;
                menuLeft.appendChild(item);
            }

            (window as any).padeditbar.checkAllIconsAreDisplayedInToolbar();
        });

        const toolbar = page.locator('.toolbar');
        const menuLeft = page.locator('.toolbar .menu_left');

        await expect(page.locator('.toolbar .menu_right')).toHaveCSS('position', 'fixed');
        await expect(toolbar).toHaveClass(/toolbar-scrollable/);

        await menuLeft.hover();
        await page.mouse.wheel(0, 800);

        await expect.poll(async () => {
            return await menuLeft.evaluate((node) => node.scrollLeft);
        }).toBeGreaterThan(0);
    });

    test('fits every bottom bar button on a small Android phone', async ({page}) => {
        await page.setViewportSize({width: 360, height: 640});
        await goToNewPad(page);

        const boxes = await page.evaluate(() =>
            [...document.querySelectorAll('.toolbar ul.menu_right > li')]
                .filter((li) => getComputedStyle(li).display !== 'none')
                .map((li) => {
                    const r = li.getBoundingClientRect();
                    return {key: (li as HTMLElement).dataset.key ?? '', left: r.left, right: r.right};
                }));
        expect(boxes.length).toBeGreaterThan(3);
        for (const b of boxes) {
            expect(b.left, b.key).toBeGreaterThanOrEqual(0);
            expect(b.right, b.key).toBeLessThanOrEqual(360);
        }
        // No button sits on top of another (the users button used to cover the last one).
        const sorted = [...boxes].sort((a, b) => a.left - b.left);
        for (let i = 1; i < sorted.length; i++) {
            expect(sorted[i].left, sorted[i].key).toBeGreaterThanOrEqual(sorted[i - 1].right - 0.5);
        }
        expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360);
    });

    test('text uses the screen width and the chat is a round button above the bar', async ({page}) => {
        await page.setViewportSize({width: 360, height: 640});
        await goToNewPad(page);

        const text = await page.locator('#innerdocbody').boundingBox();
        expect(text).not.toBeNull();
        if (text) {
            expect(text.x).toBeLessThan(10);
            expect(text.width).toBeGreaterThan(340);
        }

        const icon = page.locator('#chaticon');
        await expect(icon).toBeVisible();
        const box = await icon.boundingBox();
        const bar = await page.locator('.toolbar .menu_right').boundingBox();
        expect(box).not.toBeNull();
        expect(bar).not.toBeNull();
        if (box && bar) {
            expect(box.width).toBeLessThanOrEqual(56);
            expect(box.y + box.height).toBeLessThanOrEqual(bar.y);
            expect(box.x + box.width).toBeLessThanOrEqual(360);
        }
    });

    test('settings popup stays on screen on a small phone', async ({page}) => {
        await page.setViewportSize({width: 360, height: 640});
        await goToNewPad(page);
        await showSettings(page);

        const box = await page.locator('#settings').boundingBox();
        const bar = await page.locator('.toolbar .menu_right').boundingBox();
        expect(box).not.toBeNull();
        expect(bar).not.toBeNull();
        if (box && bar) {
            expect(box.y).toBeGreaterThanOrEqual(0);
            expect(box.x).toBeGreaterThanOrEqual(0);
            expect(box.x + box.width).toBeLessThanOrEqual(360);
            expect(box.y + box.height).toBeLessThanOrEqual(bar.y);
        }
    });

    test('a tap below the text puts the caret at the end and typing reaches the pad', async ({page}) => {
        await page.setViewportSize({width: 390, height: 844});
        await goToNewPad(page);
        const container = await page.locator('#editorcontainer').boundingBox();
        expect(container).not.toBeNull();
        if (!container) return;
        await page.mouse.click(container.x + container.width / 2, container.y + container.height - 80);
        const where = await page.evaluate(() => {
            const sel = window.getSelection();
            const node = sel?.anchorNode;
            const inner = document.getElementById('innerdocbody');
            return {
                focused: document.activeElement === inner,
                inLine: !!node && !!inner && inner.contains(node) && node !== inner,
            };
        });
        expect(where).toEqual({focused: true, inLine: true});
        await page.keyboard.type('XYZ');
        await expect(page.locator('#innerdocbody')).toContainText('XYZ');
    });
});
