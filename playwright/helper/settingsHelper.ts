import {expect, Page} from "@playwright/test";
import {setEpCheckbox} from "./padHelper";

export const isSettingsShown = async (page: Page) => {
    const classes = await page.locator('#settings').getAttribute('class')
    return classes?.includes('popup-show')
}

export const showSettings = async (page: Page) => {
    if (await isSettingsShown(page)) return
    await page.locator("button[class~='buttonicon-settings']").click()
    await expect(page.locator('#settings')).toHaveClass(/popup-show/, { timeout: 5000 })
}

export const hideSettings = async (page: Page) => {
    if (!await isSettingsShown(page)) return
    await page.locator("button[title='Settings']").click()
    await expect(page.locator('#settings')).not.toHaveClass(/popup-show/, { timeout: 5000 })
}

// #options-stickychat is an <ep-checkbox>, so Playwright's native
// .isChecked()/.check()/.uncheck()/toBeChecked() do not apply.
export const enableStickyChatviaSettings = async (page: Page) => {
    const stickyChat = page.locator('#options-stickychat')
    await stickyChat.waitFor({ state: 'visible', timeout: 5000 });
    await setEpCheckbox(stickyChat, true);
}

export const disableStickyChat = async (page: Page) => {
    const stickyChat = page.locator('#options-stickychat')
    await stickyChat.waitFor({ state: 'visible', timeout: 5000 });
    await setEpCheckbox(stickyChat, false);
}
