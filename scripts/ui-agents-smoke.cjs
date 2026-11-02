// Session flow smoke: mock only native terminal and agent CLIs, use real fixture APIs.
const { JSDOM, VirtualConsole } = require("jsdom");
const fs = require("fs"),
  assert = require("node:assert/strict");
const url = process.argv[2],
  token = new URL(url).searchParams.get("token");
const wait = async (fn) => {
  for (let i = 0; i < 160; i++) {
    if (fn()) return;
    await new Promise((r) => setTimeout(r, 25));
  }
  throw Error("Timed out: " + fn);
};
(async () => {
  const registry = await (await fetch(new URL("/api/agents", url), { headers: { Cookie: "omd=" + token } })).json();
  let available = process.argv.includes("--remembered"),
    launched = [],
    trashCalls = 0,
    closed = 0;
  const errors = [],
    state = {
      home: process.cwd() + "/.fixtures/home",
      busy: false,
      hasScan: false,
      revision: 0,
      phase: "",
      protected: [],
      entries: 0,
      directories: 0,
      rate: 0,
    };
  const vc = new VirtualConsole();
  vc.on("jsdomError", (e) => errors.push(e.message));
  const dom = new JSDOM(
    fs
      .readFileSync("internal/web/assets/index.html", "utf8")
      .replace(/<script[^>]*><\/script>/g, ""),
    {
      url,
      runScripts: "dangerously",
      pretendToBeVisual: true,
      virtualConsole: vc,
      beforeParse(w) {
        if (process.argv.includes("--remembered") || process.argv.includes("--missing-remembered")) {
          w.localStorage.setItem("omd-agent-choice", "claude");
          w.localStorage.setItem("omd-agent-tip-dismissed", "1");
        }
        w.HTMLDialogElement.prototype.showModal = function () { this.open = true; };
        w.HTMLDialogElement.prototype.close = function () { this.open = false; };
        w.fetch = (path, options = {}) => {
          if (path === "/api/trash") {
            trashCalls++;
            return Promise.resolve(new Response(JSON.stringify({results: JSON.parse(options.body).paths.map(path => ({path, trashed: true}))}), {headers: {"Content-Type": "application/json"}}));
          }
          return fetch(new URL(path, url), {
            ...options,
            headers: { ...options.headers, Cookie: "omd=" + token },
          });
        };
        w.Path2D = class {
          arc() {}
          roundRect() {}
        };
        w.HTMLCanvasElement.prototype.getContext = () =>
          new Proxy(
            {},
            {
              get: (o, k) => o[k] || (() => {}),
              set: (o, k, v) => ((o[k] = v), true),
            },
          );
        w.Terminal = class {
          constructor() {
            this.cols = 90;
            this.rows = 22;
          }
          loadAddon() {}
          open() {}
          onData() {}
          attachCustomKeyEventHandler() {}
          write() {}
          focus() {}
          dispose() {}
          getSelection() {
            return "";
          }
        };
        w.FitAddon = {
          FitAddon: class {
            fit() {}
          },
        };
        w.WebLinksAddon = { WebLinksAddon: class {} };
        w.go = {
          main: {
            App: {
              State: async () => ({ ...state }),
              Agents: async () => registry.map(d => ({ ...d, path: d.name === "claude" && available ? "/mock/claude" : "" })),
              Scan: async () => {
                state.busy = true;
                state.phase = "walking";
              },
              Cancel: async () => {
                state.busy = false;
                state.phase = "Cancelled";
              },
              StartAgent: async (...args) => launched.push(args),
              ReadAgent: async () => ({ data: "", exited: true }),
              CloseAgent: async () => {
                closed++;
              },
              ResizeAgent: async () => {},
              Copy: async () => {},
              Paste: async () => "",
              PickFolder: async () => state.home,
            },
          },
        };
      },
    },
  );
  const w = dom.window,
    $ = (id) => w.document.getElementById(id);
  w.eval(
    ["app.js", "locations.js", "desktop.js", "agents.js"]
      .map((file) => fs.readFileSync("internal/web/assets/" + file, "utf8"))
      .join("\n"),
  );
  const option = name => [...$("agent-options").querySelectorAll("button")].find(b => b.textContent.includes(name));
  await wait(() => $("agent-options").textContent.includes("Claude"));
  assert.equal($("ask-agent").hidden, true);
  assert.equal($("agent-control").parentElement.id, "scan-agent-slot");
  for (const id of ["folders", "categories", "cleanup-tab", "overview"]) {
    $(id).click();
    assert.equal($("ask-agent").hidden, true);
    assert.equal($("agent-control").parentElement.id, "scan-agent-slot");
    assert.equal($("launch-cards").hidden, false);
  }
  if (process.argv.includes("--remembered") || process.argv.includes("--missing-remembered")) {
    assert.equal($("agent-choice").textContent, available ? "Claude" : "None");
    assert.equal($("agent-tip").hidden, true);
    assert.equal(w.localStorage.getItem("omd-agent-choice"), "claude");
    w.close();
    console.log("PASS: remembered choice, unavailable fallback, once-only nudge, pre-scan visibility on every page.");
    return;
  }
  assert.equal($("agent-choice").textContent, "None");

  assert.equal(w.document.querySelectorAll(".launch-card").length, 1);
  assert.equal(option("Claude"), undefined);
  assert.ok($("agent-options").querySelector('a[aria-label="Install Claude"]'));
  assert.equal($("agent-tip").hidden, false);
  $("agent-dismiss").click();
  assert.equal(w.localStorage.getItem("omd-agent-tip-dismissed"), "1");
  available = true;
  await $("agent-check").onclick();
  assert.ok(option("Claude"));
  assert.equal(option("Claude").querySelector("use").getAttribute("href"), "#regular-magnifying-glass");
  assert.equal(option("Codex"), undefined);
  option("Claude").click();
  assert.equal(w.document.activeElement, $("agent-picker").querySelector("summary"));
  assert.equal(w.localStorage.getItem("omd-agent-choice"), "claude");
  await $("plain-scan").onclick();
  await wait(() => $("plain-scan").disabled);
  assert.equal($("agent-control").hidden, true);
  assert.equal(launched.length, 0);
  state.busy = false;
  state.hasScan = true;
  state.phase = "Complete";
  state.revision = 1;
  await wait(() => launched.length === 1);
  assert.equal(launched[0][0], "claude");
  assert.equal($("agent-terminal").hidden, false);
  await wait(() => $("terminal-status").textContent === "Session ended");
  await $("terminal-close").onclick();
  assert.equal($("agent-terminal").hidden, true);
  assert.equal($("ask-agent").hidden, false);
  assert.equal($("agent-control").parentElement.id, "results-agent-slot");
  $("folders").click();
  assert.equal($("agent-control").parentElement.id, "toolbar-agent-slot");
  $("cleanup-tab").click();
  assert.equal($("agent-control").parentElement.id, "results-agent-slot");
  await $("ask-agent").onclick();
  assert.equal(launched.length, 2);
  await $("terminal-close").onclick();
  // A cancelled scan cannot hand off to an agent.
  await $("plain-scan").onclick();
  $("cancel-scan").onclick();
  state.phase = "Complete";
  state.revision = 2;
  await new Promise((r) => setTimeout(r, 900));
  assert.equal(launched.length, 2);
  assert.ok(closed >= 2);
  // None scans without launching, even when a CLI is installed.
  option("None").click();
  await $("hub-scan").onclick();
  await wait(() => $("plain-scan").disabled);
  state.busy = false;
  state.phase = "Complete";
  state.revision = 3;
  await wait(() => !$("plain-scan").disabled);
  await new Promise(r => setTimeout(r, 500));
  assert.equal(launched.length, 2);
  assert.equal($("agent-choice").textContent, "None");
  // Confirm a mocked response only: no real Trash request is sent.
  const box = w.document.querySelector(".suggestion-card input:not(:disabled)");
  assert.ok(box);
  box.click();
  $("trash-selected").click();
  await $("confirm-trash").onclick();
  assert.equal(trashCalls, 1);
  assert.match($("trash-results").textContent, /moved to Trash. Review more items or scan again/);
  assert.ok($("trash-results").querySelector("details"));
  assert.equal($("selection-bar").hidden, true);
  assert.deepEqual(errors, []);
  w.close();
  console.log(
    "PASS: install states, check again, dismissal persistence, scan-to-agent handoff, terminal close/reopen, cancelled scan never launches. Native terminal mocked.",
  );
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
