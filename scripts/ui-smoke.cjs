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
  await wait(
    () =>
      d.querySelectorAll(".category-cluster").length > 0 &&
      d.querySelectorAll(".suggestion-card").length > 0,
  );
  assert.equal($("overview").getAttribute("aria-current"), "page");
  assert.equal($("explorer").hidden, true);
  assert.equal(
    d.querySelectorAll(".tile-grid i").length,
    d.querySelectorAll(".category-cluster").length * 24,
  );
  assert.match($("hud-scanned").textContent, /\d+\.\d{2} GB/);
  assert.ok(d.querySelector(".rail-card"));
  for (const path of [
    "/fonts/InterVariable.woff2",
    "/fonts/JetBrainsMono-Regular.woff2",
  ])
    assert.equal((await w.fetch(path)).status, 200);
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
  const category = d.querySelector("#children .filename");
  category.click();
  await wait(() => $("folder-name").textContent === category.textContent);
  assert(requests.some((x) => x.startsWith("/api/category?")));
  $("cleanup-tab").click();
  await wait(() => d.querySelectorAll(".suggestion-card").length > 0);
  assert.match(d.querySelector(".suggestion-score").textContent, /\d+\/100/);
  const box = d.querySelector(".suggestion-card input:not(:disabled)");
  box.click();
  assert.equal($("selection-bar").hidden, false);
  assert.match($("selection-total").textContent, /1 selected/);
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
  const fixture = process.cwd() + "/.fixtures/home";
  assert.ok(
    $("scan-info").textContent.startsWith(fixture),
    "Run this check only against .fixtures/home",
  );
  $("web-root").value = fixture;
  await $("web-start-scan").onclick();
  assert.equal($("web-scan-status").textContent, "Complete");
  $("overview").click();
  await $("hub-scan").onclick();
  await wait(() => $("web-scan-status").textContent === "Complete");
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
    "PASS: overview tiles, HUD decimal GB, embedded fonts, folder drill/back, global sort, treemap, category lazy load, cleanup sort/filter, selection, Trash confirmation/cancel, binary units, notes, web rescan and hub scan. No script errors; no Trash action sent.",
  );
  w.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
