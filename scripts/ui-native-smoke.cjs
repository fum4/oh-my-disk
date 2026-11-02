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
  const errors = [],
    calls = [];
  const vc = new VirtualConsole();
  vc.on("jsdomError", (e) => errors.push(e.message));
  const state = {
    home: fixture,
    busy: false,
    hasScan: true,
    revision: 1,
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
            PickFolder: async () => fixture,
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
  await wait(() => !$("open-last").hidden);
  assert.equal($("web-scan").hidden, true);
  assert.equal($("desktop").hidden, false);
  $("folders").click(); // Safe before a snapshot has been opened.
  await $("open-last").onclick();
  await wait(() => d.querySelector(".category-cluster"));
  assert.equal($("overview-panel").hidden, false);
  assert.equal($("explorer").hidden, true);
  assert.equal(d.querySelector(".view-switch").hidden, true);
  await $("choose-folder").onclick();
  assert.equal($("scan-root").value, "custom");
  assert.equal($("chosen-path").textContent, fixture);
  await $("hub-scan").onclick();
  await wait(() => !$("cancel-scan").hidden);
  assert.equal($("hub-scan").disabled, true);
  assert.match($("progress").textContent, /Walking · 42 entries/);
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
    (b) => b.textContent === "Copy path",
  );
  await copy.onclick();
  assert.equal(calls.at(-1)[0], "copy");
  $("cleanup-tab").click();
  await wait(() => d.querySelector(".suggestion-card .item-actions"));
  const cmd = [
    ...d.querySelectorAll(".suggestion-card .item-actions button"),
  ].find((b) => b.textContent === "Copy suggested command");
  assert(cmd);
  await cmd.onclick();
  assert.equal(calls.at(-1)[0], "copy");
  await $("export-json").onclick();
  await $("export-summary").onclick();
  assert.deepEqual(
    calls.filter((c) => c[0] === "export"),
    [
      ["export", false],
      ["export", true],
    ],
  );
  state.protected = ["Library"];
  await wait(() => !$("access-banner").hidden);
  await $("full-disk-access").onclick();
  assert.equal(calls.at(-1)[0], "access");
  assert.equal(errors.length, 0, errors.join("\n"));
  console.log(
    "PASS: native welcome/cache, navigation before scan, folder picker, hub scan, progress, cancel, completion, Reveal, copy path/command, both exports, Full Disk Access. Native methods stubbed.",
  );
  w.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
