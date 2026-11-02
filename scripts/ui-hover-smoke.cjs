// Browser geometry checks: painted hover growth must leave glyphs and layout fixed.
const { chromium } = require("playwright");
const assert = require("node:assert/strict");
const url = process.argv[2];
if (!url) throw Error("Usage: node scripts/ui-hover-smoke.cjs <fixture-ui-url>");
require("node:fs").mkdirSync(".build/screens", { recursive: true });
(async () => {
  const browser = await chromium.launch({
    executablePath: process.env.OMD_CHROME || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    headless: true,
    args: ["--no-sandbox"],
    env: { ...process.env, HOME: process.cwd() + "/.build/chrome-home" },
  });
  try {
    const page = await browser.newPage();
    await page.goto(url);
    await page.waitForSelector("#overview-folders .fr");
    const geometry = locator => locator.evaluate(node => {
      const rect = element => {
        const r = element.getBoundingClientRect();
        return [r.x, r.y, r.width, r.height];
      };
      const text = document.createRange();
      text.selectNodeContents(node);
      return {
        box: rect(node),
        text: [text.getBoundingClientRect().width, text.getBoundingClientRect().height],
        icons: [...node.querySelectorAll("svg")].map(rect),
        transform: getComputedStyle(node).transform,
      };
    });
    for (const theme of ["light", "dark"]) {
      await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
      for (const width of [1440, 900, 390]) {
        await page.setViewportSize({ width, height: 900 });
        await page.locator("#access-banner").evaluate(node => { node.hidden = false; });
        const close = page.locator("#dismiss-access");
        await page.mouse.move(0, 0);
        await close.evaluate(node => node.blur());
        await page.waitForTimeout(180);
        assert.equal(await close.evaluate(node => getComputedStyle(node).backgroundColor), "rgba(0, 0, 0, 0)");
        assert.equal(await close.evaluate(node => getComputedStyle(node).cursor), "pointer");
        const before = await geometry(close);
        const banner = await page.locator("#access-banner").boundingBox();
        assert.ok(Math.abs(before.box[1] + before.box[3] / 2 - banner.y - banner.height / 2) < 1);
        for (const interaction of [() => close.hover(), async () => {
          await page.mouse.move(0, 0);
          await page.locator("#full-disk-access").focus();
          await page.keyboard.press("Tab");
          assert.equal(await close.evaluate(node => node.matches(":focus-visible")), true);
        }]) {
          await interaction();
          await page.waitForTimeout(180);
          assert.notEqual(await close.evaluate(node => getComputedStyle(node).backgroundColor), "rgba(0, 0, 0, 0)");
          assert.deepEqual(await geometry(close), before);
        }
        await page.screenshot({ path: `.build/screens/round6-${theme}-${width}-warning.png` });
        await close.evaluate(node => node.blur());
        for (const selector of ["#full-disk-access", "#hub-scan", "#overview-folders .fr button"]) {
          const button = page.locator(selector).first();
          if (!await button.isVisible()) continue;
          await page.mouse.move(0, 0);
          const normal = await geometry(button);
          await button.hover();
          await page.waitForTimeout(180);
          assert.equal((await geometry(button)).transform, "none");
          assert.deepEqual(await geometry(button), normal, selector);
        }
      }
    }
    // Render the actual landing DOM without a saved snapshot.
    await page.evaluate(() => { snapshot = undefined; showMode("overview"); });
    for (const theme of ["light", "dark"]) {
      await page.emulateMedia({ colorScheme: theme });
      for (const width of [1200, 900, 390]) {
        await page.setViewportSize({ width, height: 700 });
        const segments = ["#folder-picker summary", "#scan-agent-slot summary", "#plain-scan"];
        const boxes = await Promise.all(segments.map(s => page.locator(s).boundingBox()));
        for (const box of boxes) {
          assert.equal(box.height, 44);
          assert.equal(box.y, boxes[0].y);
        }
        const bar = await page.locator(".scan-command-bar").boundingBox();
        assert.equal(bar.height, 56);
        assert.ok(bar.x >= 0 && bar.x + bar.width <= width, "Command bar fits viewport");
        for (const selector of segments) {
          const segment = page.locator(selector);
          await page.mouse.move(0, 0);
          await segment.evaluate(n => n.blur());
          const before = await geometry(segment);
          await segment.hover();
          await page.waitForTimeout(180);
          assert.deepEqual(await geometry(segment), before, selector);
          assert.equal(await segment.evaluate(n => getComputedStyle(n).cursor), "pointer");
          await segment.focus();
          await page.keyboard.press("Tab");
          await page.keyboard.press("Shift+Tab");
          assert.equal(await segment.evaluate(n => n.matches(":focus-visible")), true);
        }
        await page.locator("#folder-picker summary").click();
        await page.keyboard.press("Escape");
        assert.equal(await page.locator("#folder-picker").evaluate(n => n.open), false);
        assert.equal(await page.locator("#folder-picker summary").evaluate(n => n.matches(":focus-visible")), true);
        await page.screenshot({ path: `.build/screens/round7-${theme}-${width}-landing.png` });
      }
    }
    console.log("PASS: transparent warning close, hover/focus fill, centered stable bounds; unchanged button text/icons; command bar height, fit, hover stability and keyboard focus across light/dark and three widths.");
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });
