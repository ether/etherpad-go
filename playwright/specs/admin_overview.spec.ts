import {expect, test} from "@playwright/test";

// Admin tests need auth — skip if no SSO configured
test.describe('admin overview', () => {
    test('admin page loads', async ({page}) => {
        const resp = await page.goto('/admin/');
        expect(resp?.status()).toBeLessThan(400);
        // Should have the root div for the React app
        await expect(page.locator('#root')).toBeAttached();
    })

    test('admin returns config endpoint', async ({request}) => {
        const resp = await request.get('/admin/config');
        expect(resp.status()).toBe(200);
        const json = await resp.json();
        // Should have oidc field (null or object)
        expect(json).toHaveProperty('oidc');
    })
})
