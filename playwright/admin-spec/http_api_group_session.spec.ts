import {expect, test} from "@playwright/test";
import {randomUUID} from "node:crypto";
import {createApiKey, loginToAdmin} from "../helper/adminhelper";

// The flow of Moodle and other integrations: with an HTTP API key from the
// admin page they create a group, an author and a session, set the
// sessionID cookie and open the group pad.
test.describe('HTTP API group pads', () => {
    test('a session from the API opens the group pad, no session does not', async ({page, request, context, browser}) => {
        const admin = await browser.newPage()
        await loginToAdmin(admin, 'admin', 'changeme1')
        const apiKey = await createApiKey(admin, 'Group pad test')
        await admin.close()

        const call = async (fn: string, params: Record<string, string>) => {
            const res = await request.get(`/api/1.3.1/${fn}`, {params: {apikey: apiKey, ...params}})
            expect(res.status()).toBe(200)
            const body = await res.json()
            expect(body.code, `${fn}: ${body.message}`).toBe(0)
            return body.data
        }
        const {groupID} = await call('createGroupIfNotExistsFor', {groupMapper: `course-${randomUUID()}`})
        const {authorID} = await call('createAuthorIfNotExistsFor', {authorMapper: `student-${randomUUID()}`, name: 'Student'})
        const {padID} = await call('createGroupPad', {groupID, padName: 'notes', text: 'Group notes from the API'})
        const validUntil = String(Math.floor(Date.now() / 1000) + 3600)
        const {sessionID} = await call('createSession', {groupID, authorID, validUntil})

        // Without a session the group pad stays closed.
        await page.goto(`/p/${encodeURIComponent(padID)}`)
        await expect(page.locator('#permissionDenied')).toBeVisible({timeout: 15000})

        await context.addCookies([{name: 'sessionID', value: sessionID, url: 'http://localhost:9001'}])
        await page.goto(`/p/${encodeURIComponent(padID)}`)
        const body = page.locator('#innerdocbody')
        await expect(body).toBeVisible({timeout: 30000})
        await expect(body).toContainText('Group notes from the API')

        await body.click()
        await page.keyboard.press('End')
        await page.keyboard.type(' and from the browser')
        await expect.poll(async () => (await call('getText', {padID})).text, {timeout: 10000})
            .toContain('and from the browser')

        // The author the API created is the one who wrote.
        const {authorIDs} = await call('listAuthorsOfPad', {padID})
        expect(authorIDs).toContain(authorID)

        // Deleting the session closes the pad again.
        await call('deleteSession', {sessionID})
        await page.goto(`/p/${encodeURIComponent(padID)}`)
        await expect(page.locator('#permissionDenied')).toBeVisible({timeout: 15000})
    })
})
