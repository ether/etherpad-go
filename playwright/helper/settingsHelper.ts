import {expect, Page} from "@playwright/test";

export const isSettingsShown = async (page: Page) => {
    const classes = await page.locator('#settings').getAttribute('class')
    return classes?.includes('popup-show')
}


export const showSettings = async (page: Page) => {
    if(await isSettingsShown(page)) return
    await page.waitForSelector('iframe[name="ace_outer"]');
    await page.locator("button[class~='buttonicon-settings']").click()
    await page.waitForFunction(`document.querySelector('#settings').classList.contains('popup-show')`)
}

export const hideSettings = async (page: Page) => {
    if(!await isSettingsShown(page)) return
    await page.locator("button[title='Settings']").click()
    await page.waitForFunction(`!document.querySelector('#settings').classList.contains('popup-show')`)
}

export const enableStickyChatviaSettings = async (page: Page) => {
    const stickyChat = page.locator('#options-stickychat')
    await stickyChat.waitFor({ state: 'visible', timeout: 5000 });
    const checked = await stickyChat.isChecked()
    if(checked) return
    await stickyChat.check({force: true})
    await expect(stickyChat).toBeChecked({ timeout: 5000 });
}

export const disableStickyChat = async (page: Page) => {
    const stickyChat = page.locator('#options-stickychat')
    await stickyChat.waitFor({ state: 'visible', timeout: 5000 });
    const checked = await stickyChat.isChecked()
    if(!checked) return
    await stickyChat.uncheck({force: true})
    await expect(stickyChat).not.toBeChecked({ timeout: 5000 });
}
