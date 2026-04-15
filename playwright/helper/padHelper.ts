import {expect, Locator, Page} from "@playwright/test";
import {randomUUID} from "node:crypto";
import os from "node:os";

const isMac = os.platform() === 'darwin';
const modifier = isMac ? 'Meta' : 'Control';

// After the WebComponents migration (ui/src/js/ace.ts) the editor no longer
// uses nested iframes — #outerdocbody and #innerdocbody are regular divs in
// the main document. getPadOuter is kept for backwards compatibility with
// specs that only needed a scope; it now returns the page itself as a
// Locator-returning helper.
export const getPadOuter = async (page: Page): Promise<Page> => {
    return page;
}

export const getPadBody = async (page: Page): Promise<Locator> => {
    return page.locator('#innerdocbody');
}

export const selectAllText = async (page: Page) => {
    await page.keyboard.down(modifier);
    await page.keyboard.press('a');
    await page.keyboard.up(modifier);
}

export const toggleUserList = async (page: Page) => {
    await page.locator("button[class~='buttonicon-showusers']").click()
}

export const setUserName = async (page: Page, userName: string) => {
    await page.waitForSelector('[class="popup popup-show"]')
    await page.click("#myusernameedit");
    await page.keyboard.type(userName);
}

export const showChat = async (page: Page) => {
    const chatIcon = page.locator("#chaticon")
    const classes = await chatIcon.getAttribute('class')
    if (classes && !classes.includes('visible')) return
    await chatIcon.click()
    await expect(chatIcon).not.toHaveClass(/visible/, { timeout: 5000 })
}

export const getCurrentChatMessageCount = async (page: Page) => {
    return await page.locator('#chattext').locator('p').count()
}

export const getChatUserName = async (page: Page) => {
    return await page.locator('#chattext')
        .locator('p')
        .locator('b')
        .innerText()
}

export const getChatMessage = async (page: Page) => {
    return (await page.locator('#chattext')
        .locator('p')
        .textContent({}))!
        .split(await getChatTime(page))[1]
}

export const getChatTime = async (page: Page) => {
    return await page.locator('#chattext')
        .locator('p')
        .locator('.time')
        .innerText()
}

export const sendChatMessage = async (page: Page, message: string) => {
    const currentChatCount = await getCurrentChatMessageCount(page)
    const chatInput = page.locator('#chatinput')
    await chatInput.click()
    await chatInput.fill(message)
    await page.keyboard.press('Enter')
    if (message === "") return
    await expect(page.locator('#chattext').locator('p')).toHaveCount(currentChatCount + 1, { timeout: 10000 })
}

export const isChatBoxShown = async (page: Page) => {
    const classes = await page.locator('#chatbox').getAttribute('class')
    return classes?.includes('visible')
}

export const isChatBoxSticky = async (page: Page): Promise<boolean> => {
    const classes = await page.locator('#chatbox').getAttribute('class')
    return classes !== null && classes.includes('stickyChat')
}

export const hideChat = async (page: Page) => {
    if (!await isChatBoxShown(page) || await isChatBoxSticky(page)) return
    await page.locator('#titlecross').click()
    await expect(page.locator('#chatbox')).not.toHaveClass(/stickyChat/, { timeout: 5000 })
}

export const enableStickyChatviaIcon = async (page: Page) => {
    if (await isChatBoxSticky(page)) return
    await page.locator('#titlesticky').click()
    await expect(page.locator('#chatbox')).toHaveClass(/stickyChat/, { timeout: 5000 })
}

export const disableStickyChatviaIcon = async (page: Page) => {
    if (!await isChatBoxSticky(page)) return
    await page.locator('#titlecross').click()
    await expect(page.locator('#chatbox')).not.toHaveClass(/stickyChat/, { timeout: 5000 })
}

export const appendQueryParams = async (page: Page, queryParameters: Record<string, string>) => {
    const searchParams = new URLSearchParams(page.url().split('?')[1]);
    Object.keys(queryParameters).forEach((key) => {
        searchParams.append(key, queryParameters[key]);
    });
    await page.goto(page.url() + "?" + searchParams.toString());
    await page.locator('#innerdocbody').waitFor({ state: 'visible', timeout: 30000 });
}

const PAD_TIMEOUT = process.env.CI && os.arch() === 'arm64' ? 60000 : 30000;

const waitForPadToLoad = async (page: Page, timeout: number = PAD_TIMEOUT) => {
    await page.locator('#innerdocbody').waitFor({ state: 'visible', timeout });
};

const navigateToPad = async (page: Page, padId: string) => {
    const padUrl = `http://localhost:9001/p/${padId}`;
    for (let attempt = 0; attempt < 3; attempt++) {
        try {
            await page.goto(padUrl, { waitUntil: 'domcontentloaded', timeout: PAD_TIMEOUT });
            await waitForPadToLoad(page);
            return;
        } catch (error) {
            const isInterruptedNavigation = error instanceof Error &&
                error.message.includes('interrupted by another navigation');
            if (isInterruptedNavigation) {
                await page.waitForURL((url) =>
                    decodeURIComponent(url.pathname).endsWith(`/${padId}`), { timeout: PAD_TIMEOUT });
                await waitForPadToLoad(page);
                return;
            }
            if (attempt === 2) throw error;
        }
    }
};

export const goToNewPad = async (page: Page) => {
    const padId = "FRONTEND_TESTS" + randomUUID();
    await navigateToPad(page, padId);
    return padId;
}

export const goToPad = async (page: Page, padId: string) => {
    await navigateToPad(page, padId);
}

export const clearPadContent = async (page: Page) => {
    const body = page.locator('#innerdocbody');
    await body.click();
    await selectAllText(page);
    await page.keyboard.press('Backspace');
    // Wait for content to actually clear
    await expect(body.locator('div').first()).toHaveText('', { timeout: 5000 });
}

export const writeToPad = async (page: Page, text: string) => {
    const body = page.locator('#innerdocbody');
    await body.click();
    await page.keyboard.type(text, { delay: 5 });
}

export const clearAuthorship = async (page: Page) => {
    await page.locator("button[class~='buttonicon-clearauthorship']").click()
}

export const undoChanges = async (page: Page) => {
    await page.keyboard.down(modifier);
    await page.keyboard.press('z');
    await page.keyboard.up(modifier);
}

export const pressUndoButton = async (page: Page) => {
    await page.locator('.buttonicon-undo').click()
}
