import {Frame, Locator, Page} from "@playwright/test";
import {randomUUID} from "node:crypto";
import os from "node:os";

const isMac = os.platform() === 'darwin';
const modifier = isMac ? 'Meta' : 'Control';

export const getPadOuter =  async (page: Page): Promise<Frame> => {
    return page.frame('ace_outer')!;
}

export const getPadBody =  async (page: Page): Promise<Locator> => {
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
    await page.waitForFunction(`!document.querySelector('#chaticon').classList.contains('visible')`)
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
    let currentChatCount = await getCurrentChatMessageCount(page)

    const chatInput = page.locator('#chatinput')
    await chatInput.click()
    await page.keyboard.type(message)
    await page.keyboard.press('Enter')
    if(message === "") return
    await page.waitForFunction(`document.querySelector('#chattext').querySelectorAll('p').length >${currentChatCount}`)
}

export const isChatBoxShown = async (page: Page):Promise<boolean> => {
    const classes = await page.locator('#chatbox').getAttribute('class')
    return classes !==null && classes.includes('visible')
}

export const isChatBoxSticky = async (page: Page):Promise<boolean> => {
    const classes = await page.locator('#chatbox').getAttribute('class')
    console.log('Chat', classes && classes.includes('stickyChat'))
    return classes !==null && classes.includes('stickyChat')
}

export const hideChat = async (page: Page) => {
    if(!await isChatBoxShown(page)|| await isChatBoxSticky(page)) return
    await page.locator('#titlecross').click()
    await page.waitForFunction(`!document.querySelector('#chatbox').classList.contains('stickyChat')`)

}

export const enableStickyChatviaIcon = async (page: Page) => {
    if(await isChatBoxSticky(page)) return
    await page.locator('#titlesticky').click()
    await page.waitForFunction(`document.querySelector('#chatbox').classList.contains('stickyChat')`)
}

export const disableStickyChatviaIcon = async (page: Page) => {
    if(!await isChatBoxSticky(page)) return
    await page.locator('#titlecross').click()
    await page.waitForFunction(`!document.querySelector('#chatbox').classList.contains('stickyChat')`)
}


export const appendQueryParams = async (page: Page, queryParameters: Record<string, string>) => {
    const searchParams = new URLSearchParams(page.url().split('?')[1]);
    Object.keys(queryParameters).forEach((key) => {
        searchParams.append(key, queryParameters[key]);
    });
    await page.goto(page.url()+"?"+ searchParams.toString());
    await page.waitForSelector('iframe[name="ace_outer"]');
}

export const goToNewPad = async (page: Page) => {
    // create a new pad before each test run
    const padId = "FRONTEND_TESTS"+randomUUID();
    await page.goto('http://localhost:9001/p/'+padId);
    // Wait for the editor to be fully loaded
    await page.waitForSelector('iframe[name="ace_outer"]', { timeout: 30000 });
    // Wait for the inner frame to be ready
    const innerFrame = page.frame('ace_inner');
    if (innerFrame) {
        await innerFrame.waitForSelector('#innerdocbody', { timeout: 30000 });
        // Wait for text content to appear (the default "Welcome to Etherpad!" message)
        await innerFrame.waitForFunction(
            () => {
                const body = document.querySelector('#innerdocbody');
                return body && body.textContent && body.textContent.length > 0;
            },
            { timeout: 10000 }
        );
    }
    // Give the editor a moment to stabilize
    await page.waitForTimeout(500);
    return padId;
}

export const goToPad = async (page: Page, padId: string) => {
    await page.goto('http://localhost:9001/p/'+padId);
    await page.waitForSelector('iframe[name="ace_outer"]', { timeout: 30000 });
    // Wait for the inner frame to be ready
    const innerFrame = page.frame('ace_inner');
    if (innerFrame) {
        await innerFrame.waitForSelector('#innerdocbody', { timeout: 30000 });
        // Wait for text content to appear
        await innerFrame.waitForFunction(
            () => {
                const body = document.querySelector('#innerdocbody');
                return body && body.textContent && body.textContent.length > 0;
            },
            { timeout: 10000 }
        );
    }
    // Give the editor a moment to stabilize
    await page.waitForTimeout(500);
}


export const clearPadContent = async (page: Page) => {
    const innerFrame = page.frame('ace_inner');
    if (!innerFrame) {
        throw new Error('Could not find ace_inner frame');
    }
    const body = innerFrame.locator('#innerdocbody');

    // Click to focus
    await body.click();
    // Small delay to ensure focus
    await page.waitForTimeout(100);

    // Select all and delete
    await page.keyboard.down(modifier);
    await page.keyboard.press('a');
    await page.keyboard.up(modifier);
    await page.keyboard.press('Backspace');

    // Wait for content to be cleared
    await page.waitForTimeout(200);
}

export const writeToPad = async (page: Page, text: string) => {
    const innerFrame = page.frame('ace_inner');
    if (!innerFrame) {
        throw new Error('Could not find ace_inner frame');
    }
    const body = innerFrame.locator('#innerdocbody');

    // Click to focus the editor
    await body.click();
    // Small delay to ensure focus
    await page.waitForTimeout(100);

    // Type the text
    await page.keyboard.type(text, { delay: 20 });

    // Wait for text to be rendered
    await page.waitForTimeout(200);
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
