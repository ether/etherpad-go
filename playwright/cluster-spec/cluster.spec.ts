import {expect, Page, test} from "@playwright/test";
import {
    goToNewPad, goToPad, sendChatMessage, setEpCheckbox, setUserName, showChat, toggleUserList, writeToPad,
} from "../helper/padHelper";

// Two instances, one database (doc/cluster.md): users on different instances
// see each other's edits, chat, names, pad settings and sheet edits live.

const A = process.env.EP_NODE_A ?? 'http://localhost:9001';
const B = process.env.EP_NODE_B ?? 'http://localhost:9002';

const twoUsers = async (browser: import('@playwright/test').Browser, page: Page) => {
    const padId = await goToNewPad(page, A);
    const ctx = await browser.newContext();
    const onB = await ctx.newPage();
    await goToPad(onB, padId, B);
    return {padId, onB, close: () => ctx.close()};
};

test('edits travel both ways between the instances', async ({page, browser}) => {
    const {onB, close} = await twoUsers(browser, page);
    try {
        await writeToPad(page, 'written on A ');
        await expect(onB.locator('#innerdocbody')).toContainText('written on A');
        await onB.locator('#innerdocbody').click();
        await onB.keyboard.press('Control+End');
        await onB.keyboard.type('and on B', {delay: 5});
        await expect(page.locator('#innerdocbody')).toContainText('written on A and on B');
        await expect(onB.locator('#innerdocbody')).toContainText('written on A and on B');
    } finally {
        await close();
    }
});

test('chat and user names reach the other instance', async ({page, browser}) => {
    const {onB, close} = await twoUsers(browser, page);
    try {
        const name = `Alice ${Date.now().toString(36)}`;
        await toggleUserList(page);
        await setUserName(page, name);
        await page.keyboard.press('Enter');
        await toggleUserList(page);

        await toggleUserList(onB);
        await expect(onB.locator('#userlist .name', {hasText: name})).toHaveCount(1);
        await toggleUserList(onB);

        await showChat(page);
        await sendChatMessage(page, 'hello across instances');
        await showChat(onB);
        const msg = onB.locator('#chattext ep-chat-message').last();
        await expect(msg).toContainText('hello across instances');
        await expect(msg).toHaveAttribute('author', name);
    } finally {
        await close();
    }
});

test('pad-wide settings of the creator apply on the other instance', async ({page, browser}) => {
    const {onB, close} = await twoUsers(browser, page);
    try {
        const lineNumbersShown = () => onB.locator('#sidediv')
            .evaluate((node) => !node.parentElement?.classList.contains('line-numbers-hidden'));
        await expect.poll(lineNumbersShown).toBe(true);
        await page.locator("button[class~='buttonicon-settings']").click();
        await expect(page.locator('#settings')).toHaveClass(/popup-show/);
        await setEpCheckbox(page.locator('#padsettings-options-linenoscheck'), false);
        await expect.poll(lineNumbersShown).toBe(false);
    } finally {
        await close();
    }
});

test('sheet edits travel between the instances', async ({page, browser}) => {
    const cell = (p: Page, r: number, c: number) =>
        p.locator(`.sheet-grid tbody tr:nth-child(${r + 1}) td:nth-child(${c + 2})`);
    const padId = `cluster-sheet-${Date.now()}`;
    await page.goto(`${A}/s/${padId}`);
    await page.locator('.sheet-grid').waitFor();
    const ctx = await browser.newContext();
    try {
        const onB = await ctx.newPage();
        await onB.goto(`${B}/s/${padId}`);
        await onB.locator('.sheet-grid').waitFor();

        await cell(page, 0, 0).click();
        await page.keyboard.type('21');
        await page.keyboard.press('Enter');
        await expect(cell(onB, 0, 0)).toHaveText('21');

        await cell(onB, 0, 1).click();
        await onB.keyboard.type('=A1*2');
        await onB.keyboard.press('Enter');
        await expect(cell(page, 0, 1)).toHaveText('42');
    } finally {
        await ctx.close();
    }
});
