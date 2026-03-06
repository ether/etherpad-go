import {expect, test} from "@playwright/test";
import {goToNewPad} from "../helper/padHelper";

test.beforeEach(async ({page}) => {
    await page.setViewportSize({width: 1280, height: 900});
    await goToNewPad(page);
});

test.describe('desktop toolbar overflow', () => {
    test('uses horizontal scrolling instead of collapsing the toolbar', async ({page}) => {
        await page.evaluate(() => {
            const menuLeft = document.querySelector('.toolbar .menu_left');
            if (!(menuLeft instanceof HTMLUListElement)) return;

            for (let i = 0; i < 40; i++) {
                const item = document.createElement('li');
                item.dataset.type = 'button';
                item.dataset.key = `overflow-${i}`;
                item.innerHTML = `<a title="Overflow ${i}" aria-label="Overflow ${i}"><button class="buttonicon" aria-label="Overflow ${i}">${i}</button></a>`;
                menuLeft.appendChild(item);
            }

            (window as any).padeditbar.checkAllIconsAreDisplayedInToolbar();
        });

        const toolbar = page.locator('.toolbar');
        const menuLeft = page.locator('.toolbar .menu_left');

        await expect(page.locator('.toolbar .menu_right')).not.toHaveCSS('position', 'fixed');
        await expect(toolbar).toHaveClass(/toolbar-scrollable/);

        const dimensionsBefore = await menuLeft.evaluate((node) => ({
            clientWidth: node.clientWidth,
            scrollWidth: node.scrollWidth,
            scrollLeft: node.scrollLeft,
        }));
        expect(dimensionsBefore.scrollWidth).toBeGreaterThan(dimensionsBefore.clientWidth);

        await menuLeft.hover();
        await page.mouse.wheel(0, 800);

        await expect.poll(async () => {
            return await menuLeft.evaluate((node) => node.scrollLeft);
        }).toBeGreaterThan(0);

        await expect(toolbar).toHaveClass(/toolbar-can-scroll-left/);

        const arrowState = await toolbar.evaluate((node) => {
            const before = getComputedStyle(node, '::before');
            const after = getComputedStyle(node, '::after');
            return {
                beforeContent: before.content,
                afterContent: after.content,
                afterOpacity: after.opacity,
            };
        });
        expect(arrowState.beforeContent).toContain('‹');
        expect(arrowState.afterContent).toContain('›');
        expect(Number(arrowState.afterOpacity)).toBeGreaterThan(0.2);
    });
});
