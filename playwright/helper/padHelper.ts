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

// ep-checkbox is a Lit web component, not a native <input type="checkbox">,
// so Playwright's .check()/.uncheck()/toBeChecked() do not apply. The
// component reflects its state via the `checked` attribute on the host
// element and toggles on click. Use this helper whenever a test needs to
// set or assert the state of an <ep-checkbox>.
export const setEpCheckbox = async (locator: Locator, want: boolean) => {
    const isChecked = () => locator.evaluate((el: Element) => el.hasAttribute('checked'));
    if ((await isChecked()) !== want) {
        await locator.click({force: true});
    }
    await expect.poll(isChecked).toBe(want);
}

export const isEpCheckboxChecked = (locator: Locator): Promise<boolean> =>
    locator.evaluate((el: Element) => el.hasAttribute('checked'));

// <ep-dropdown> is a Lit web component, not a native <select>, so
// Playwright's .selectOption() / toHaveValue() do not apply.
//
// Playwright's actionability checks (visibility in particular) don't
// always cooperate with the way Lit projects slotted content into the
// shadow DOM's `.content-wrapper` — even after the dropdown is open,
// `ep-dropdown-item` is slotted through a fixed-position wrapper that
// Playwright can report as not-visible on both Chromium and Firefox.
// So we drive the component the same way `_selectItem()` does internally:
// wait for the matching item to exist, then dispatch `ep-dropdown-select`
// on the host. That's exactly what a real click would trigger, minus the
// actionability gymnastics.
export const selectEpDropdownItem = async (page: Page, dropdownSelector: string, value: string) => {
    const dropdown = page.locator(dropdownSelector);
    await dropdown.waitFor({ state: 'attached', timeout: 10000 });
    // Ensure the target <ep-dropdown-item> has been rendered into the
    // dropdown before we try to select it.
    await expect.poll(async () =>
        await dropdown.evaluate(
            (el: Element, v: string) => !!el.querySelector(`ep-dropdown-item[value="${CSS.escape(v)}"]`),
            value,
        ),
        { timeout: 10000 },
    ).toBe(true);
    // Some select handlers (e.g. #languagemenu) call location.reload(),
    // which detaches the frame mid-evaluate and causes evaluate() to reject.
    // Swallow that — callers that care about the reload wrap us in
    // Promise.all(page.waitForLoadState('load'), ...).
    await dropdown.evaluate((el: any, v: string) => {
        el.dispatchEvent(new CustomEvent('ep-dropdown-select', {
            bubbles: true,
            composed: true,
            detail: { value: v },
        }));
        if (typeof el.close === 'function') el.close();
    }, value).catch(() => {});
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

// Chat messages are rendered as <ep-chat-message> web components (author,
// time, and `own` are reflected attributes; the message body lives in the
// default slot). The old helpers looked for <p> elements from the pre-
// webcomponent chat UI — every selector here needed to move to the new tag.
export const getCurrentChatMessageCount = async (page: Page) => {
    return await page.locator('#chattext').locator('ep-chat-message').count()
}

export const getChatUserName = async (page: Page) => {
    return (await page.locator('#chattext')
        .locator('ep-chat-message')
        .first()
        .getAttribute('author')) ?? ''
}

export const getChatMessage = async (page: Page) => {
    // The slotted body contains the message text. textContent on the host
    // returns the combined light-DOM children (author/time live in shadow DOM).
    return (await page.locator('#chattext')
        .locator('ep-chat-message')
        .first()
        .textContent()) ?? ''
}

export const getChatTime = async (page: Page) => {
    return (await page.locator('#chattext')
        .locator('ep-chat-message')
        .first()
        .getAttribute('time')) ?? ''
}

export const sendChatMessage = async (page: Page, message: string) => {
    const currentChatCount = await getCurrentChatMessageCount(page)
    const chatInput = page.locator('#chatinput')
    await chatInput.click()
    await chatInput.fill(message)
    await page.keyboard.press('Enter')
    if (message === "") return
    await expect(page.locator('#chattext').locator('ep-chat-message')).toHaveCount(currentChatCount + 1, { timeout: 10000 })
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
