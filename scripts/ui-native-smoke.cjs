const { JSDOM, VirtualConsole } = require("jsdom");
const fs = require("node:fs"),
  assert = require("node:assert/strict");
const url = process.argv[2];
if (!url)
  throw Error("Usage: node scripts/ui-native-smoke.cjs <fixture-ui-url>");
const token = new URL(url).searchParams.get("token"),
  fixture = process.cwd() + "/.fixtures/home";
const wait = async (fn) => {
  for (let i = 0; i < 120; i++) {
    if (fn()) return;
    await new Promise((r) => setTimeout(r, 30));
  }
  throw Error("Timed out: " + fn);
};
(async () => {
  const registry = await (await fetch(new URL("/api/agents", url), { headers: { Cookie: "omd=" + token } })).json();
  const errors = [],
    calls = [];
  const vc = new VirtualConsole();
  vc.on("jsdomError", (e) => errors.push(e.message));
  const empty = process.argv.includes("--empty");
  let pickedFolder = fixture;
  const state = {
    home: fixture,
    busy: false,
    hasScan: !empty,
    revision: empty ? 0 : 1,
    phase: "Cached",
    entries: 42,
    directories: 8,
    rate: 20,
    path: fixture,
    protected: [],
  };
  const app = await JSDOM.fromURL(url, {
    resources: "usable",
    runScripts: "dangerously",
    pretendToBeVisual: true,
    virtualConsole: vc,
    beforeParse(w) {
      if (process.argv.includes("--dismissed-access"))
        w.localStorage.setItem("omd-access-warning-dismissed", "1");
      w.fetch = (path, options = {}) =>
        fetch(new URL(path, url), {
          ...options,
          headers: { ...options.headers, Cookie: "omd=" + token },
        });
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
      w.go = {
        main: {
          App: {
            State: async () => ({ ...state }),
            Agents: async () => registry.map(d => ({ ...d, path: "/mock/" + d.name })),
            PickFolder: async () => pickedFolder,
            Scan: async (root) => {
              calls.push(["scan", root]);
              state.busy = true;
              state.phase = "Walking";
            },
            Cancel: async () => {
              calls.push(["cancel"]);
              state.busy = false;
              state.phase = "Canceled";
            },
            Export: async (summary) => {
              calls.push(["export", summary]);
              return "/tmp/export.json";
            },
            Reveal: async (path) => calls.push(["reveal", path]),
            Copy: async (text) => calls.push(["copy", text]),
            FullDiskAccess: async () => calls.push(["access"]),
          },
        },
      };
    },
  });
  const w = app.window,
    d = w.document,
    $ = (id) => d.getElementById(id);
  if (empty) {
    await wait(() => !$("desktop").hidden);
    assert.equal($("welcome").hidden, false);
    assert.equal($("content-grid").hidden, true);
    $("folders").click();
    assert.equal($("welcome").hidden, false);
    assert.equal($("view-title").textContent, "Folders");
    assert.equal($("scan-feedback").hidden, true);
    assert.equal(d.querySelector("details.scan-options"), null);
    assert.match($("launch-description").textContent, /folders/);
    assert.equal($("launch-cards").hidden, false);
    assert.equal($("empty-actions"), null);
    const sharedCommand = d.querySelector(".scan-command-bar");
    const sharedAgent = $("agent-control");
    assert.equal($("welcome").classList.contains("card"), false);
    assert($("access-banner").compareDocumentPosition($("welcome")) & w.Node.DOCUMENT_POSITION_FOLLOWING);
    for (const id of ["categories", "cleanup-tab", "overview"]) {
      $(id).click();
      assert.equal($(id).getAttribute("aria-current"), "page");
      assert.equal($("content-grid").hidden, true);
      assert.equal(d.querySelector(".scan-command-bar"), sharedCommand);
      assert.equal($("agent-control"), sharedAgent);
      assert.equal($("launch-cards").hidden, false);
      assert.equal($("empty-actions"), null);
      assert.equal($("ask-agent"), null);
    }
    assert.equal($("ask-agent"), null);
    assert.equal($("scan-options").hidden, true); // Minimal landing.
    assert.equal($("units-binary"), null);
    assert.equal($("binary-units"), null);
    const locationCount = $("locations").childElementCount;
    $("locations").querySelector(".location-remove").click();
    assert.equal($("locations").childElementCount, locationCount - 1);
    assert.equal($("location-toast").hidden, false);
    assert.equal(
      JSON.parse(w.localStorage.getItem("omd-locations-v1")).length,
      locationCount - 1,
    );
    $("undo-location").click();
    assert.equal($("locations").childElementCount, locationCount);
    $("location-home").click();
    await wait(() => calls.some((c) => c[0] === "scan"));
    assert.equal(calls.find((c) => c[0] === "scan")[1], fixture);
    await $("cancel-scan").onclick();
    await wait(() => !$("hub-scan").disabled);
    await $("rescan-new").onclick();
    assert.equal($("chosen-path").textContent, fixture);
    await $("hub-scan").onclick();
    await wait(() => !$("cancel-scan").hidden);
    assert.equal($("hub-scan").disabled, true);
    assert.equal(d.body.classList.contains("is-scanning"), true);
    await $("cancel-scan").onclick();
    await wait(() => $("cancel-scan").hidden);
    pickedFolder = fixture + "/Downloads";
    await $("scan-another").onclick();
    assert.equal(calls.at(-1)[1], pickedFolder);
    assert(
      JSON.parse(w.localStorage.getItem("omd-locations-v1")).includes(
        pickedFolder,
      ),
    );
    await $("cancel-scan").onclick();
    await wait(() => !$("hub-scan").disabled);
    // Reload the Locations module with the same saved storage, as on launch.
    w.eval(fs.readFileSync("internal/web/assets/locations.js", "utf8"));
    assert(
      [...d.querySelectorAll(".location-open")].some(
        (b) => b.title === "Scan " + pickedFolder,
      ),
    );
    const row = [...d.querySelectorAll(".location-row")].find(
      (n) => n.querySelector(".location-open").title === "Scan " + pickedFolder,
    );
    row.querySelector(".location-remove").click();
    w.eval(fs.readFileSync("internal/web/assets/locations.js", "utf8"));
    assert(
      ![...d.querySelectorAll(".location-open")].some(
        (b) => b.title === "Scan " + pickedFolder,
      ),
    );
    state.error = "older cache format; rescan with --refresh";
    await new Promise((r) => setTimeout(r, 450));
    assert.equal($("scan-feedback").hidden, true);
    assert(!$("scan-feedback").textContent.includes("--refresh"));
    assert.equal(errors.length, 0, errors.join("\n"));
    console.log(
      "PASS: native empty welcome, navigation before scan, split scan picker, inline settings, decimal units, favorites removal/Undo/persistence, scan progress/cancel, silent old-cache handling.",
    );
    w.close();
    return;
  }
  await wait(() => !$("open-last").hidden);
  assert.equal($("ask-agent"), null);
  assert.equal($("web-scan").hidden, true);
  assert.equal($("desktop").hidden, false);
  assert.equal($("choose-folder").hidden, false);
  assert.equal($("choose-folder").parentElement, $("scan-root").parentElement);
  $("folders").click(); // Safe before a snapshot has been opened.
  await $("open-last").onclick();
  await wait(() => d.querySelector("#overview-folders .fr"));
  assert.equal($("ask-agent"), null);
  assert.equal($("agent-control").parentElement.id, "scan-agent-slot");
  assert.match($("hub-scan").textContent, /^Rescan/);
  assert.equal($("scan-root").value, "home");
  assert.equal($("overview-panel").hidden, true);
  assert.equal($("explorer").hidden, false);
  assert.equal(d.querySelector(".view-switch").hidden, false);
  state.error = "older cache format; rescan with --refresh";
  await new Promise((r) => setTimeout(r, 450));
  assert.equal($("scan-feedback").hidden, true);
  state.error = "";
  await $("choose-folder").onclick();
  assert.equal($("scan-root").value, "custom");
  assert.equal($("chosen-path").textContent, fixture);
  await $("hub-scan").onclick();
  await wait(() => !$("cancel-scan").hidden);
  assert.equal($("hub-scan").disabled, true);
  assert.equal($("progress").textContent, "Walking folders…");
  assert.equal($("scan-target").textContent, "home");
  assert.equal($("scan-info").textContent, "Scanning home…");
  await wait(() => $("scan-count").textContent === "42");
  assert.equal($("scan-stats").textContent, "8 folders · 20/s");
  assert.equal($("scan-path").title, fixture);
  assert.equal($("scan-spinner").getAttribute("aria-valuenow"), null);
  $("categories").click();
  assert.equal($("categories").getAttribute("aria-current"), "page");
  assert.equal($("scan-info").textContent, "Scanning home…");
  state.phase = "classifying";
  await wait(() => $("progress").textContent === "Scoring cleanup…");
  state.phase = "writing cache";
  await wait(() => $("progress").textContent === "Measuring sizes…");
  await $("cancel-scan").onclick();
  await wait(() => $("cancel-scan").hidden);
  assert.equal($("hub-scan").disabled, false);
  state.phase = "Complete";
  state.revision = 2;
  await wait(() => $("progress").textContent === "Complete");
  $("folders").click();
  await wait(() => d.querySelector(".item-actions button"));
  await d.querySelector(".item-actions button").onclick();
  assert.equal(calls.at(-1)[0], "reveal");
  const copy = [...d.querySelectorAll(".item-actions button")].find(
    (b) => b.getAttribute("aria-label") === "Copy path",
  );
  await copy.onclick();
  assert.equal(calls.at(-1)[0], "copy");
  assert.equal(copy.getAttribute("aria-label"), "Copied");
  assert.ok(copy.querySelector("svg use"));
  $("cleanup-tab").click();
  await wait(() => d.querySelector(".suggestion-details .item-actions"));
  const cmd = [
    ...d.querySelectorAll(".suggestion-details .item-actions button"),
  ].find((b) => b.textContent === "Copy suggested command");
  assert(cmd);
  await cmd.onclick();
  assert.equal(calls.at(-1)[0], "copy");
  assert.equal(copy.getAttribute("aria-label"), "Copied");
  assert.ok(copy.querySelector("svg use"));
  await $("export-json").onclick();
  await $("export-summary").onclick();
  assert.equal($("scan-feedback").classList.contains("banner-success"), true);
  assert.match($("scan-feedback").textContent, /Exported to/);
  assert.deepEqual(
    calls.filter((c) => c[0] === "export"),
    [
      ["export", false],
      ["export", true],
    ],
  );
  state.protected = ["Library"];
  if (process.argv.includes("--dismissed-access")) {
    await new Promise((r) => setTimeout(r, 450));
    assert.equal($("access-banner").hidden, true);
  } else {
    await wait(() => !$("access-banner").hidden);
    await $("full-disk-access").onclick();
    assert.equal(calls.at(-1)[0], "access");
    $("dismiss-access").click();
    assert.equal($("access-banner").hidden, true);
    assert.equal(w.localStorage.getItem("omd-access-warning-dismissed"), "1");
    state.protected = [];
    await new Promise((r) => setTimeout(r, 450));
    state.protected = ["Library"];
    await new Promise((r) => setTimeout(r, 450));
    assert.equal($("access-banner").hidden, true);
  }
  assert.equal(errors.length, 0, errors.join("\n"));
  console.log(
    "PASS: native welcome/cache, navigation before scan, folder picker, hub scan, progress, cancel, completion, Reveal, copy path/command, both exports, Full Disk Access. Native methods stubbed.",
  );
  w.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
