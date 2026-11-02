// Render-only fixture sweep. Never confirms Trash or scans a real folder.
const { chromium } = require("playwright");
const fs = require("node:fs");
const assert = require("node:assert/strict");
const url = process.argv[2];
if (!url) throw Error("Usage: node scripts/ui-capture.cjs <fixture-ui-url>");
fs.mkdirSync(".build/screens", { recursive: true });
(async () => {
  const browser = await chromium.launch({
    executablePath:
      process.env.OMD_CHROME ||
      "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    headless: true,
    args: ["--no-sandbox"],
    env: { ...process.env, HOME: process.cwd() + "/.build/chrome-home" },
  });
  const page = await browser.newPage(),
    errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  const shot = async (name) =>
    page.screenshot({
      path: `.build/screens/polish-${name}.png`,
      fullPage: true,
    });
  await page.goto(url);
  await page.waitForSelector("#overview-folders .fr");
  await page.evaluate(() => document.fonts.ready);
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme });
    for (const width of [1440, 900, 390]) {
      await page.setViewportSize({ width, height: 900 });
      const prefix = `${theme}-${width}`;
      for (const id of ["overview", "folders", "categories", "cleanup-tab"]) {
        await page.locator(`#${id}`).click();
        await page.evaluate(() => {
          if (document.documentElement.scrollWidth > innerWidth)
            throw Error("Horizontal overflow");
          const footer = document
            .querySelector("footer")
            .getBoundingClientRect();
          if (innerWidth > 700) {
            const grid = document
              .querySelector("#content-grid")
              .getBoundingClientRect();
            if (grid.bottom > footer.top)
              throw Error("Footer overlaps content");
            const main = document.querySelector("main");
            if (main.scrollHeight > main.clientHeight + 1)
              throw Error("Page overflow");
          }
          for (const node of document.querySelectorAll(
            "button:not(:disabled), summary, select:not(:disabled), input[type=checkbox]:not(:disabled)",
          )) {
            if (getComputedStyle(node).cursor !== "pointer")
              throw Error("Missing pointer cursor: " + node.id);
          }
        });
        await shot(`${prefix}-${id}`);
        // A scroll must leave the panel heading and footer still in place.
        const list =
          id === "overview"
            ? "#overview-folders"
            : id === "cleanup-tab"
              ? "#suggestion-list"
              : ".table-scroll";
        const before = await page.locator("footer").boundingBox();
        await page.locator(list).evaluate((n) => {
          n.scrollTop = n.scrollHeight;
        });
        assert.deepEqual(await page.locator("footer").boundingBox(), before);
        await page.locator(list).evaluate((n) => {
          n.scrollTop = 0;
        });
      }
      await page.locator("#folders").click();
      await page.locator("#treemap").click();
      await shot(`${prefix}-treemap`);
      await page.locator("#sunburst").click();
      await page.locator("#cleanup-tab").click();
      await page.locator(".item-name").first().click();
      await shot(`${prefix}-details`);
      await page
        .locator(".suggestion-card input:not(:disabled)")
        .first()
        .check();
      await page.locator("#trash-selected").click();
      await shot(`${prefix}-confirm`);
      await page.locator("#cancel-trash").click();
      await page.locator("#clear-selection").click();
      // State fixtures exercise presentation without invoking native actions.
      await page.locator("#overview").click();
      await page.evaluate(() => {
        document.querySelector("#access-banner").hidden = false;
        setOptionsOpen(true);
      });
      await shot(`${prefix}-access-options`);
      await page.evaluate(() => {
        setOptionsOpen(false);
        document.querySelector("#access-banner").hidden = true;
        document.body.classList.add("is-scanning");
      });
      await shot(`${prefix}-scanning`);
      await page.evaluate(() => {
        document.body.classList.remove("is-scanning");
        snapshot = undefined;
        showMode("overview");
      });
      await shot(`${prefix}-empty`);
      await page.evaluate(() =>
        showFeedback("Unable to load scan. Choose a folder or scan again."),
      );
      await shot(`${prefix}-error`);
      await page.reload();
      await page.waitForSelector("#overview-folders .fr");
    }
  }
  assert.deepEqual(errors, []);
  await browser.close();
  console.log(
    "PASS: light/dark × wide/narrow/mobile, all pages, maps, details, confirmation, access/options, scanning, empty/error, cursors and scroll boundaries.",
  );
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
