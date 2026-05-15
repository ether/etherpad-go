import {expect, test} from "@playwright/test";
import {randomInt} from "node:crypto";
import {goToNewPad, sendChatMessage, setUserName, showChat, toggleUserList} from "../helper/padHelper";

test.beforeEach(async ({ page })=>{
    // create a new pad before each test run
    await goToNewPad(page);
})


test("Remembers the username after a refresh", async ({page}) => {
    await toggleUserList(page);
    await setUserName(page,'😃')
    await toggleUserList(page)

    await page.reload();
    await toggleUserList(page);
    const usernameField = page.locator("#myusernameedit");
    await expect(usernameField).toHaveValue('😃');
})


test('Own user name is shown when you enter a chat', async ({page})=> {
    const chatMessage = 'O hi';

    await toggleUserList(page);
    await setUserName(page,'😃');
    await toggleUserList(page);

    await showChat(page);
    await sendChatMessage(page,chatMessage);
    // Chat renders as <ep-chat-message> webcomponents: the author name lives
    // on the `author` attribute, and the message body is the slotted text.
    const chatMsg = page.locator('#chattext').locator('ep-chat-message').first();
    await expect(chatMsg).toBeVisible({timeout: 10000});
    const author = (await chatMsg.getAttribute('author')) ?? '';
    const body = (await chatMsg.textContent()) ?? '';
    expect(author).toContain('😃');
    expect(body).toContain(chatMessage);
});
