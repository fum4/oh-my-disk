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
          for (const node of document.querySelectorAll("th, th button, .ch, summary")) {
            const style = getComputedStyle(node);
            if (style.userSelect !== "none" && style.webkitUserSelect !== "none")
              throw Error("Selectable UI chrome: " + node.tagName);
          }
          const toolbar = document.querySelector("#scan-options");
          const target = toolbar.querySelector("#desktop[hidden] + #web-scan:not([hidden]), #desktop:not([hidden])");
          const exports = toolbar.querySelector(".export-cards").getBoundingClientRect();
          const scan = toolbar.querySelector(".scan-split").getBoundingClientRect();
          if (Math.abs(exports.y + exports.height / 2 - scan.y - scan.height / 2) > 1 || exports.right > scan.left)
            throw Error("Results actions must share one row, with exports before Rescan");
          if (target && target.getBoundingClientRect().right > exports.left) throw Error("Overlapping results actions");
          if (document.querySelector("#units-binary, #ask-agent, #terminal-resize")) throw Error("Removed drawer/unit controls returned");
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
      await page.locator("#agent").click();
      await page.evaluate(() => {
        const main = document.querySelector("main"), agent = document.querySelector("#agent-page");
        if (main.scrollHeight > main.clientHeight + 1 || agent.getBoundingClientRect().bottom > innerHeight) throw Error("Agent page exceeds the main panel");
        if (getComputedStyle(document.querySelector("#scan-options")).display !== "none") throw Error("Results toolbar visible on Agent");
      });
      await shot(`${prefix}-agent`);
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
      });
      await shot(`${prefix}-access-options`);
      await page.evaluate(() => {
        document.querySelector("#access-banner").hidden = true;
        scanState(true);
        updateScanProgress({busy: true, phase: "walking", entries: 608715, directories: 74457, rate: 60864, path: "/Users/fixture/Library/Application Support/Example/Cache"});
        document.querySelector("#cancel-scan").hidden = false;
      });
      await page.waitForFunction(() => Number(document.querySelector("#scan-count").textContent.replace(/\D/g, "")) >= 608700);
      await shot(`${prefix}-scanning`);
      await page.evaluate(() => {
        scanState(false);
        snapshot = undefined;
        showMode("overview");
      });
      await shot(`${prefix}-empty`);
      for (const id of ["folders", "categories", "cleanup-tab"]) {
        await page.locator(`#${id}`).click();
        await page.evaluate(() => {
          const description = document.querySelector("#launch-description");
          const bounds = description.getBoundingClientRect();
          const welcome = document.querySelector("#welcome").getBoundingClientRect();
          assertLayout(bounds.left >= welcome.left && bounds.right <= welcome.right,
            "Empty description exceeds its page");
          assertLayout(description.scrollWidth <= description.clientWidth,
            "Empty description overflows");
          const bar = document.querySelector(".scan-command-bar").getBoundingClientRect();
          const primary = document.querySelector("#plain-scan").getBoundingClientRect();
          assertLayout(document.querySelectorAll(".scan-command-bar").length === 1,
            "Empty pages must reuse the landing command bar");
          assertLayout(bar.left >= welcome.left && bar.right <= welcome.right,
            "Command bar exceeds its page");
          assertLayout(Math.abs(bar.left + bar.width / 2 - welcome.left - welcome.width / 2) < 2,
            "Command bar must be centered");
          assertLayout(primary.height === 44 && primary.y > bounds.bottom,
            "Scan action must be inside the command bar below its copy");
          function assertLayout(ok, message) { if (!ok) throw Error(message); }
        });
        await shot(`${prefix}-empty-${id}`);
      }
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
