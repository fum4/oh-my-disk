const { JSDOM, VirtualConsole } = require("jsdom");
const fs = require("node:fs"),
  assert = require("node:assert/strict");
const url = process.argv[2];
if (!url) throw Error("Usage: node scripts/ui-smoke.cjs <fixture-ui-url>");
const token = new URL(url).searchParams.get("token");
const wait = async (fn) => {
  for (let i = 0; i < 150; i++) {
    if (fn()) return;
    await new Promise((r) => setTimeout(r, 25));
  }
  throw Error("Timed out: " + fn);
};
(async () => {
  const errors = [],
    requests = [];
  const vc = new VirtualConsole();
  vc.on("jsdomError", (e) => errors.push(e.message));
  const app = await JSDOM.fromURL(url, {
    resources: "usable",
    runScripts: "dangerously",
    pretendToBeVisual: true,
    virtualConsole: vc,
    beforeParse(w) {
      w.fetch = async (path, options = {}) => {
        requests.push(path);
        return fetch(new URL(path, url), {
          ...options,
          headers: {
            ...options.headers,
            Cookie: "omd=" + token,
            Origin: new URL(url).origin,
          },
        });
      };
      w.Path2D = class {
        arc() {}
        roundRect() {}
      };
      w.HTMLCanvasElement.prototype.getContext = function () {
        return new Proxy(
          {},
          {
            get: (o, k) => o[k] || (() => {}),
            set: (o, k, v) => ((o[k] = v), true),
          },
        );
      };
      w.HTMLDialogElement.prototype.showModal = function () {
        this.open = true;
      };
      w.HTMLDialogElement.prototype.close = function () {
        this.open = false;
      };
    },
  });
  const w = app.window,
    d = w.document,
    $ = (id) => d.getElementById(id);
  if (process.argv.includes("--large")) {
    await wait(() => d.querySelector("#overview-folders .fr"));
    assert.match($("scan-info").title, /\.fixtures\/large/);
    const more = () =>
      [...d.querySelectorAll("#children button")].find((b) =>
        b.textContent.startsWith("Show more"),
      );
    $("folders").click();
    assert.equal(d.querySelectorAll("#children .filename").length, 200);
    more().click();
    await wait(() => d.querySelectorAll("#children .filename").length === 400);
    await d.querySelector('[data-sort="name"]').onclick();
    assert.equal(d.querySelectorAll("#children .filename").length, 200);
    assert(requests.at(-1).includes("sort=name"));
    $("categories").click();
    const scan = await (await w.fetch("/api/scan")).json();
    const largest = Object.entries(scan.categories).sort(
      (a, b) => b[1] - a[1],
    )[0][0];
    [...d.querySelectorAll("#children .filename")]
      .find((n) => n.textContent.toLowerCase() === largest.toLowerCase())
      .click();
    await wait(() => d.querySelectorAll("#children .filename").length === 200);
    more().click();
    await wait(() => d.querySelectorAll("#children .filename").length === 400);
    $("treemap").click();
    assert.ok(d.querySelector("#chart canvas"));
    assert.equal(errors.length, 0, errors.join("\n"));
    assert(!requests.some((x) => x === "/api/trash" || x === "/api/rescan"));
    console.log(
      "PASS: large fixture folder/category pagination 200 to 400, global name sort reset, treemap; no script errors or mutations.",
    );
    w.close();
    return;
  }
  await wait(
    () =>
      d.querySelectorAll("#overview-folders .fr").length > 0 &&
      d.querySelectorAll(".suggestion-card").length > 0,
  );
  assert.equal($("overview").getAttribute("aria-current"), "page");
  assert.equal($("explorer").hidden, true);
  assert.equal(d.querySelectorAll("#disk-legend .li").length, 4);
  assert.ok(d.querySelector("#ringsvg circle"));
  assert.equal($("full-disk-access").textContent, "Open settings");
  assert.equal(
    d.querySelector("[data-suggest-sort=score]").textContent.trim(),
    "Best",
  );
  assert.ok(d.querySelector(".map-panel > .view-switch"));
  assert.match(
    d.querySelector("#ringsvg circle title").textContent,
    /% of disk/,
  );
  d.querySelector("#ringsvg circle").onpointermove({
    clientX: 100,
    clientY: 100,
  });
  assert.equal($("tooltip").hidden, false);
  assert.match($("tooltip").textContent, /% of disk/);
  d.querySelector("#ringsvg circle").onpointerleave();
  assert.equal($("tooltip").hidden, true);
  assert.match($("disk-free").textContent, /\d+\.\d GB/);
  assert.ok(d.querySelector(".badge svg use"));
  for (const use of d.querySelectorAll("use"))
    assert(
      d.querySelector(use.getAttribute("href")),
      "Missing bundled icon " + use.getAttribute("href"),
    );
  for (const path of [
    "/fonts/InterVariable.woff2",
    "/fonts/JetBrainsMono-Regular.woff2",
  ])
    assert.equal((await w.fetch(path)).status, 200);
  const overviewName = d.querySelector(".item-name");
  overviewName.click();
  assert.equal(overviewName.getAttribute("aria-expanded"), "true");
  assert.equal(d.querySelector(".suggestion-details").hidden, false);
  assert.match(
    d.querySelector(".suggestion-details").textContent,
    /Rule: .+Score \d+:/,
  );
  overviewName.click();
  const firstOverviewRow = d.querySelector("#overview-folders .fr");
  firstOverviewRow.focus();
  firstOverviewRow.dispatchEvent(
    new w.KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }),
  );
  assert.equal(d.activeElement, firstOverviewRow.nextElementSibling);
  // Overview links are navigation, with a reversible history entry.
  $("overview-folders").scrollTop = 48;
  $("show-folders").click();
  assert.equal($("view-title").textContent, "Folders");
  assert.equal($("folders").getAttribute("aria-current"), "page");
  assert.equal($("overview-back").hidden, false);
  assert.match($("scan-info").textContent, /Explore your folders/);
  $("overview-back").click();
  await wait(() => $("view-title").textContent === "Overview");
  assert.equal($("overview-folders").scrollTop, 48);
  $("review-cleanup").click();
  assert.equal($("view-title").textContent, "Cleanup");
  assert.equal($("cleanup-tab").getAttribute("aria-current"), "page");
  d.dispatchEvent(new w.KeyboardEvent("keydown", {key: "Escape"}));
  await wait(() => $("view-title").textContent === "Overview");
  assert.equal($("overview-back").hidden, true);
  $("folders").click();
  assert.ok(d.querySelector("#chart canvas"));
  assert.equal($("overview-panel").hidden, true);
  const first = d.querySelector("#children .filename");
  first.click();
  await wait(() => $("folder-name").textContent === first.textContent);
  assert(requests.some((x) => x.startsWith("/api/node?")));
  d.querySelector("#breadcrumbs button").click();
  await wait(() => $("folder-name").textContent === "home");
  await d.querySelector('[data-sort="name"]').onclick();
  assert(requests.at(-1).includes("sort=name"));
  $("treemap").click();
  assert.ok(d.querySelector("#chart canvas"));
  assert.equal($("treemap").getAttribute("aria-pressed"), "true");
  $("categories").click();
  assert.equal($("explorer").classList.contains("category-summary"), true);
  for (const label of d.querySelectorAll("#children .filename")) assert.match(label.textContent, /^[A-Z]/);
  const category = d.querySelector("#children .filename");
  category.click();
  await wait(() => $("folder-name").textContent === category.textContent);
  assert(requests.some((x) => x.startsWith("/api/category?")));
  $("cleanup-tab").click();
  await wait(() => d.querySelectorAll(".suggestion-card").length > 0);
  assert.match(d.querySelector(".suggestion-score").textContent, /\d+/);
  const box = d.querySelector(".suggestion-card input:not(:disabled)");
  box.click();
  assert.equal($("selection-bar").hidden, false);
  assert.match($("selection-total").textContent, /1 item selected/);
  $("folders").click();
  assert.equal($("selection-bar").parentElement, $("explorer"));
  $("cleanup-tab").click();
  assert.equal($("selection-bar").parentElement, $("suggestions"));
  $("trash-selected").click();
  assert.equal($("trash-confirm").open, true);
  assert.ok($("confirm-paths").textContent.includes(".fixtures/home"));
  $("cancel-trash").click();
  assert.equal($("trash-confirm").open, false);
  await d.querySelector('[data-suggest-sort="size"]').onclick();
  await wait(() => requests.some((x) => x.includes("/api/suggest?sort=size")));
  const categoryValue = $("suggest-category").options[1].value;
  $("suggest-category").value = categoryValue;
  await $("suggest-category").onchange();
  await wait(() =>
    requests.at(-1).includes("category=" + encodeURIComponent(categoryValue)),
  );
  $("suggest-category").value = "";
  await $("suggest-category").onchange();
  $("binary-units").click();
  assert.match($("total").textContent, /MiB|GiB/);
  assert.match($("hud-scanned").textContent, /GB/);
  $("clear-selection").click();
  assert.equal($("selection-bar").hidden, true);
  $("details-toggle").click();
  assert.equal($("notes").hidden, false);
  assert.ok($("accounting").textContent);
  assert.equal($("details-toggle").textContent, "How we counted");
  assert.equal($("details-toggle").getAttribute("aria-expanded"), "true");
  assert.equal($("counting-disclosure").classList.contains("open"), true);
  assert.ok(d.querySelectorAll("#notes .note-row").length >= 5);
  $("notes-close").click();
  await wait(() => $("notes").hidden);
  $("details-toggle").click();
  d.dispatchEvent(new w.KeyboardEvent("keydown", {key: "Escape"}));
  await wait(() => $("notes").hidden);
  $("details-toggle").click();
  $("details-toggle").click();
  await wait(() => $("notes").hidden);
  const fixture = process.cwd() + "/.fixtures/home";
  assert.ok(
    $("scan-info").title.startsWith(fixture),
    "Run this check only against .fixtures/home",
  );
  $("web-root").value = fixture;
  await $("web-start-scan").onclick();
  assert.equal($("web-scan-status").textContent, "Complete");
  $("overview").click();
  d.dispatchEvent(
    new w.KeyboardEvent("keydown", {
      key: "r",
      metaKey: true,
      bubbles: true,
      cancelable: true,
    }),
  );
  await wait(() => $("web-scan-status").textContent === "Complete");
  assert.equal(d.body.classList.contains("is-scanning"), false);
  assert(
    !requests.some((x) => x === "/api/trash"),
    "No Trash action should be sent",
  );
  assert.equal(errors.length, 0, errors.join("\n"));
  fs.mkdirSync(".build/screens", { recursive: true });
  fs.writeFileSync(
    ".build/screens/overview.dom.html",
    d.documentElement.outerHTML,
  );
  console.log(
    "PASS: Calm ring, decimal GB, Phosphor badges, embedded fonts, folder drill/back, global sort, treemap, category lazy load, cleanup sort/filter, selection, Trash confirmation/cancel, binary units, notes, web rescan and hub scan. No script errors; no Trash action sent.",
  );
  w.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
