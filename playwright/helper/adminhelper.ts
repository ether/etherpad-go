import {expect, Page} from "@playwright/test";

export const loginToAdmin = async (page: Page, username: string, password: string) => {

    await page.goto('http://localhost:9001/admin/');

    await page.waitForSelector('input[name="username"]');
    await page.fill('input[name="username"]', username);
    await page.fill('input[name="password"]', password);
    await page.click('input[type="submit"]');
}


export const saveSettings = async (page: Page) => {
    // Click save
    await page.locator('.settings-button-bar').locator('button').first().click()
    await page.waitForSelector('.ToastRootSuccess')
}

export const restartEtherpad = async (page: Page) => {
    // Click restart
    const restartButton = page.locator('.settings-button-bar').locator('.settingsButton').nth(1)
    const settings =  page.locator('.settings');
    await expect(settings).not.toBeEmpty();
    await expect(restartButton).toBeVisible()
    await page.locator('.settings-button-bar')
        .locator('.settingsButton')
        .nth(1)
        .click()
    await page.waitForTimeout(500)
    await page.waitForSelector('.settings')
}

// issueToken issues a token on the admin page, for account with every
// scope the account has (the form's preselection), and returns its key.
export const issueToken = async (page: Page, name: string, account = 'admin', pads = ''): Promise<string> => {
    await page.getByRole('link', {name: 'Tokens'}).click()
    const form = page.getByTestId('token-issue')
    await form.getByLabel('Account').selectOption({label: account})
    await form.getByLabel('Token name').fill(name)
    if (pads) await form.getByLabel('Token pads').fill(pads)
    await form.getByRole('button', {name: 'Issue'}).click()
    const value = page.getByTestId('token-key-value')
    await expect(value).toBeVisible()
    return (await value.textContent())!.trim()
}
