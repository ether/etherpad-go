import {expect, Frame, Locator, Page} from "@playwright/test";
import {randomUUID} from "node:crypto";
import os from "node:os";

const isMac = os.platform() === 'darwin';
const modifier = isMac ? 'Meta' : 'Control';

export const getPadOuter = async (page: Page): Promise<Frame> => {
    return page.frame('ace_outer')!;
}

export const getPadBody = async (page: Page): Promise<Locator> => {
    return page.frame('ace_inner')!.locator('#innerdocbody')
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
    await page.waitForSelector('iframe[name="ace_outer"]', { timeout: 30000 });
}

const waitForPadToLoad = async (page: Page, timeout: number = 30000) => {
    // Wait for the outer frame
    await page.waitForSelector('iframe[name="ace_outer"]', { timeout, state: 'attached' });

    // Use frameLocator to wait for inner frame content — avoids polling loop
    const innerFrame = page.frameLocator('iframe[name="ace_outer"]')
        .frameLocator('iframe[name="ace_inner"]');
    await innerFrame.locator('#innerdocbody').waitFor({ state: 'visible', timeout });
};

export const goToNewPad = async (page: Page) => {
    const padId = "FRONTEND_TESTS" + randomUUID();
    await page.goto('http://localhost:9001/p/' + padId, { waitUntil: 'load', timeout: 30000 });
    await waitForPadToLoad(page, 30000);
    return padId;
}

export const goToPad = async (page: Page, padId: string) => {
    await page.goto('http://localhost:9001/p/' + padId, { waitUntil: 'load', timeout: 30000 });
    await waitForPadToLoad(page, 30000);
}

export const clearPadContent = async (page: Page) => {
    const innerFrame = page.frame('ace_inner');
    if (!innerFrame) {
        throw new Error('Could not find ace_inner frame');
    }
    const body = innerFrame.locator('#innerdocbody');
    await body.click();
    await selectAllText(page);
    await page.keyboard.press('Backspace');
    // Wait for content to actually clear
    await expect(body.locator('div').first()).toHaveText('', { timeout: 5000 });
}

export const writeToPad = async (page: Page, text: string) => {
    const innerFrame = page.frame('ace_inner');
    if (!innerFrame) {
        throw new Error('Could not find ace_inner frame');
    }
    const body = innerFrame.locator('#innerdocbody');
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
