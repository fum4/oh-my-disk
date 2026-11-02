const { chromium } = require("playwright");
const fs = require("node:fs");
const url = process.argv[2];
if (!url) throw Error("Usage: node scripts/ui-capture.cjs <fixture-ui-url>");
fs.mkdirSync(".build/screens", { recursive: true });
(async () => {
  const executablePath =
    process.env.OMD_CHROME ||
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
  const browser = await chromium.launch({
    executablePath,
    headless: true,
    args: ["--no-sandbox"],
    env: { ...process.env, HOME: process.cwd() + "/.build/chrome-home" },
  });
  const page = await browser.newPage({
      viewport: { width: 1440, height: 1080 },
    }),
    errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(url);
  await page.waitForSelector(".category-cluster");
  await page.evaluate(() => document.fonts.ready);
  for (const id of ["overview", "folders", "categories", "cleanup-tab"]) {
    await page.locator("#" + id).click();
    await page.screenshot({
      path:
        ".build/screens/" + (id === "cleanup-tab" ? "cleanup" : id) + ".png",
      fullPage: true,
    });
  }
  await page.locator("#folders").click();
  await page.locator("#treemap").click();
  await page.screenshot({ path: ".build/screens/treemap.png", fullPage: true });
  await page.locator("#cleanup-tab").click();
  await page.locator(".suggestion-card input:not(:disabled)").first().check();
  await page.locator("#trash-selected").click();
  await page.screenshot({
    path: ".build/screens/confirmation.png",
    fullPage: true,
  });
  await page.locator("#cancel-trash").click();
  await page.locator("#clear-selection").click();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator("#overview").click();
  await page.screenshot({
    path: ".build/screens/mobile-overview.png",
    fullPage: true,
  });
  if (
    await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)
  )
    throw Error("Mobile overflow");
  if (errors.length) throw Error(errors.join("\n"));
  await browser.close();
  console.log("PASS: screenshots for all views, confirmation and mobile.");
})().catch((e) => {
  console.error(e.message);
  process.exit(1);
});
