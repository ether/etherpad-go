import {expect, test} from "@playwright/test";
import {goToNewPad, setEpCheckbox} from "../helper/padHelper";

// Regression guard for setEpCheckbox. <ep-checkbox> attaches click handlers to
// .track and .label only — never to the host element — so a click aimed at the
// centre of the host's box misses whenever the label is narrow enough that the
// centre falls into the 8px gap between the two. That made qr_code.spec.ts:38
// fail on some CI runners, where the rendered font is narrower than the one the
// click point was computed against.
//
// The font-size override below reproduces that layout deterministically; the
// test fails if the helper ever goes back to clicking the host.
test('setEpCheckbox toggles when the host centre falls into the track/label gap', async ({page}) => {
    await goToNewPad(page);
    await page.addStyleTag({content: 'ep-checkbox { font-size: 8px !important; }'});
    await page.locator('li[data-key="share_qr"] button').click();
    await page.locator('#share_qr').evaluate((el) =>
        Promise.all(el.getAnimations({subtree: true}).map((a) => a.finished.catch(() => {}))));

    const centreInGap = await page.evaluate(() => {
        const host = document.getElementById('qrreadonlyinput')!;
        const r = host.getBoundingClientRect();
        const track = host.shadowRoot!.querySelector('.track')!.getBoundingClientRect();
        const label = host.shadowRoot!.querySelector('.label')!.getBoundingClientRect();
        const centre = r.left + r.width / 2;
        return centre > track.right && centre < label.left;
    });
    expect(centreInGap, 'precondition: host centre must sit in the 8px gap').toBe(true);

    await setEpCheckbox(page.locator('#qrreadonlyinput'), true);
});
