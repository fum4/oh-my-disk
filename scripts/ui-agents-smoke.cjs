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
  let available = ["--remembered", "--remembered-none", "--fresh-installed"].some(flag => process.argv.includes(flag)),
    launched = [], scanned = [],
    trashCalls = 0,
    closed = 0,
    terminal, webgl, sessionEnded = false, resized = [], written = [], copied = [], reads = 0;
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
        }
        if (process.argv.includes("--remembered-none")) w.localStorage.setItem("omd-agent-choice", "");
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
          constructor(options) {
            terminal = this;
            this.options = options;
            this.unicode = {};
            this.cols = 90;
            this.rows = 22;
          }
          loadAddon() {}
          open() {}
          onData(fn) { this.input = fn; }
          onResize(fn) { this.resized = fn; }
          attachCustomKeyEventHandler(fn) { this.key = fn; }
          paste(text) { this.pasted = text; this.input("\x1b[200~" + text + "\x1b[201~"); }
          hasSelection() { return true; }
          write(data) { this.output = data; }
          focus() {}
          dispose() {}
          getSelection() {
            return "selected output";
          }
        };
        w.FitAddon = {
          FitAddon: class {
            fit() {}
          },
        };
        w.WebLinksAddon = { WebLinksAddon: class {} };
        w.Unicode11Addon = { Unicode11Addon: class {} };
        w.WebglAddon = { WebglAddon: class {
          constructor() {
            if (process.argv.includes("--no-webgl")) throw Error("WebGL unavailable for smoke");
            webgl = this;
          }
          onContextLoss(fn) { this.lost = fn; }
          dispose() { this.disposed = true; }
        } };
        w.go = {
          main: {
            App: {
              State: async () => ({ ...state }),
              Agents: async () => registry.map(d => ({ ...d, path: (d.name === "claude" || (d.name === "codex" && process.argv.includes("--both"))) && available ? "/mock/claude" : "" })),
              Scan: async target => {
                scanned.push(target);
                state.busy = true;
                state.phase = "walking";
              },
              Cancel: async () => {
                state.busy = false;
                state.phase = "Cancelled";
              },
              StartAgent: async (...args) => { sessionEnded = false; launched.push(args); },
              ReadAgent: async () => {
                reads++;
                return { data: Buffer.from("\x1b[38;2;120;180;240m╭─ Advisor ─╮\x1b[0m").toString("base64"), exited: sessionEnded };
              },
              CloseAgent: async () => {
                closed++;
              },
              ResizeAgent: async (...size) => resized.push(size),
              WriteAgent: async data => written.push(data),
              Copy: async text => copied.push(text),
              Paste: async () => "first line\nsecond line",
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
  const refreshAgents = async () => {
    $("agent-picker").open = false;
    await new Promise(r => setTimeout(r, 10));
    $("agent-picker").open = true;
    await new Promise(r => setTimeout(r, 30));
  };
  const option = name => [...$("agent-options").querySelectorAll("button")].find(b => b.textContent.includes(name));
  await wait(() => $("agent-options").textContent.includes("Claude"));
  assert.equal($("ask-agent"), null);
  assert.equal($("agent-control").parentElement.id, "scan-agent-slot");
  for (const id of ["folders", "categories", "cleanup-tab", "overview"]) {
    $(id).click();
    assert.equal($("ask-agent"), null);
    assert.equal($("agent-control").parentElement.id, "scan-agent-slot");
    assert.equal($("launch-cards").hidden, false);
    assert.equal($("empty-actions"), null);
  }
  if (process.argv.includes("--remembered") || process.argv.includes("--missing-remembered")) {
    assert.equal($("agent-choice").textContent, available ? "Claude" : "None");
    assert.equal($("agent-tip"), null);
    assert.equal(w.localStorage.getItem("omd-agent-choice"), available ? "claude" : "");
    if (!available) {
      available = true;
      await refreshAgents();
      assert.equal($("agent-choice").textContent, "None");
    }
    w.close();
    console.log("PASS: remembered choice, unavailable fallback, minimal landing, pre-scan visibility on every page.");
    return;
  }
  assert.equal($("agent-choice").textContent, "None");
  if (process.argv.includes("--fresh-installed") || process.argv.includes("--remembered-none")) {
    assert.ok(option("Claude"));
    assert.equal(option("None").getAttribute("aria-pressed"), "true");
    await $("plain-scan").onclick();
    state.busy = false;
    state.hasScan = true;
    state.phase = "Complete";
    state.revision = 1;
    await wait(() => !$("plain-scan").disabled);
    assert.equal(launched.length, 0);
    assert.equal($("agent-choice").textContent, "None");
    w.close();
    console.log("PASS: fresh/remembered None stays a plain scan with an installed agent.");
    return;
  }

  if (process.argv.includes("--before-scan")) {
    available = true;
    await refreshAgents();
    $("agent").click();
    assert.equal($("agent-page").hidden, false);
    assert.equal($("launch-cards").parentElement.id, "agent-scan-slot");
    assert.match($("agent-nudge").textContent, /Scan first/);
    await $("agent-start").onclick();
    assert.equal(launched.length, 1);
    assert.equal(state.hasScan, false);
    assert.equal($("agent-running-dot").hidden, false);
    $("overview").click(); $("agent").click();
    assert.equal(launched.length, 1);
    const restarting = $("terminal-restart").onclick();
    assert.equal($("agent-confirm").open, true);
    $("agent-replace").click(); await restarting;
    assert.equal(launched.length, 2);
    await $("terminal-close").onclick();
    assert.equal($("agent-running-dot").hidden, true);
    assert.deepEqual(errors, []);
    w.close();
    console.log("PASS: start before scan, shared command bar, navigation persistence, confirmed restart and stop.");
    return;
  }
  assert.equal(w.document.querySelectorAll(".launch-card").length, 1);
  assert.equal(option("Claude"), undefined);
  assert.ok($("agent-options").querySelector('a[aria-label="Install Claude"]'));
  assert.equal($("agent-tip"), null);
  assert.equal($("folder-choice").textContent, "Home");
  assert.equal($("toolbar-choose"), null);
  assert.equal($("options-toggle"), null);
  assert.equal($("scan-another").textContent.trim(), "Scan Folder…");
  const savedOptions = () => [...$("rescan-options").querySelectorAll("button")];
  assert.deepEqual(savedOptions().map(b => b.textContent), [...$("locations").querySelectorAll(".location-name")].map(b => b.textContent));
  savedOptions().find(b => b.textContent === "Macintosh HD").click();
  assert.equal($("scan-root").value, "/");
  assert.equal(state.busy, false);
  const folderOptions = () => [...$("folder-options").querySelectorAll("button")];
  assert.deepEqual(folderOptions().map(b => b.textContent), [...$("locations").querySelectorAll(".location-name")].map(b => b.textContent));
  folderOptions().find(b => b.textContent === "Macintosh HD").click();
  assert.equal($("folder-choice").textContent, "Macintosh HD");
  assert.equal($("scan-root").value, "/");
  assert.equal(state.busy, false); // Selecting a target never starts a scan.
  folderOptions().find(b => b.textContent === "Home").click();
  await $("landing-choose").onclick();
  assert.equal($("folder-choice").textContent, "home");
  assert.equal($("scan-options").hidden, true); // Native picker leaves Options closed.
  folderOptions().find(b => b.textContent === "Home").click();
  w.addLocation("/tmp/Command bar sample");
  assert.ok(folderOptions().some(b => b.textContent === "Command bar sample"));
  w.document.querySelector('[aria-label="Remove Command bar sample from Locations"]').click();
  assert.ok(!folderOptions().some(b => b.textContent === "Command bar sample"));
  $("undo-location").click();
  assert.ok(folderOptions().some(b => b.textContent === "Command bar sample"));
  $("folder-picker").open = true;
  w.document.dispatchEvent(new w.KeyboardEvent("keydown", {key: "Escape"}));
  assert.equal($("folder-picker").open, false);
  assert.equal(w.document.activeElement, $("folder-picker").querySelector("summary"));
  available = true;
  await refreshAgents();
  assert.equal($("agent-check"), null);
  assert.equal($("agent-options").closest(".floating-menu").parentElement, w.document.body);
  const unchanged = option("Claude");
  await refreshAgents();
  assert.equal(option("Claude"), unchanged); // No redraw when detection is unchanged.
  assert.ok(option("Claude"));
  assert.equal(option("Claude").querySelector("use").getAttribute("href"), "#agent-claude");
  assert.equal(!!option("Codex"), process.argv.includes("--both"));
  option("Claude").click();
  assert.equal(w.document.activeElement, $("agent-picker").querySelector("summary"));
  assert.equal(w.localStorage.getItem("omd-agent-choice"), "claude");
  await $("plain-scan").onclick();
  await wait(() => $("plain-scan").disabled);
  assert.equal(scanned[0], state.home);
  assert.equal($("agent-control").hidden, true);
  assert.equal(launched.length, 0);
  state.busy = false;
  state.hasScan = true;
  state.phase = "Complete";
  state.revision = 1;
  await wait(() => launched.length === 1);
  assert.equal(launched[0][0], "claude");
  assert.equal($("agent-terminal").hidden, false);
  await wait(() => $("terminal-status").textContent === "Running");
  assert.match(terminal.options.fontFamily, /SF Mono.*Menlo/);
  assert.equal(terminal.options.theme.background, "#fafbfc");
  assert.equal(terminal.options.cursorBlink, true);
  assert.equal(terminal.options.macOptionIsMeta, true);
  assert.equal(terminal.unicode.activeVersion, "11");
  assert.match(Buffer.from(terminal.output).toString(), /╭─ Advisor ─╮/);
  assert.equal($("agent").getAttribute("aria-current"), "page");
  assert.equal($("agent-page").hidden, false);
  assert.equal($("agent-running-dot").hidden, false);
  terminal.resized({ cols: 112, rows: 35 });
  assert.deepEqual(resized.at(-1), [112, 35]);
  const closes = closed, instance = terminal, beforeReads = reads;
  $("overview").click();
  await $("scan-agent-link").onclick();
  assert.equal(launched.length, 1);
  const selectedSession = [...$("session-options").querySelectorAll("button")].find(b => b.getAttribute("aria-pressed") === "true");
  selectedSession.click();
  assert.equal(launched.length, 1);
  for (const page of ["folders", "categories", "cleanup-tab", "overview", "agent"]) $(page).click();
  await wait(() => reads > beforeReads);
  assert.equal(closed, closes);
  assert.equal(terminal, instance);
  assert.equal(launched.length, 1);
  if (process.argv.includes("--both")) {
    const codex = [...$("session-options").querySelectorAll("button")].find(b => b.textContent.includes("Codex"));
    codex.click();
    assert.equal($("agent-confirm").open, true);
    $("agent-keep").click();
    await new Promise(r => setTimeout(r, 10));
    assert.equal(closed, closes);
    assert.equal($("session-choice").textContent, "Claude");
    codex.click();
    $("agent-replace").click();
    await wait(() => launched.length === 2);
    assert.equal(launched.at(-1)[0], "codex");
    assert.equal($("session-choice").textContent, "Codex");
    await $("terminal-close").onclick();
    assert.deepEqual(errors, []);
    w.close();
    console.log("PASS: running agent switch requires confirmation; cancel preserves session, accept replaces it once.");
    return;
  }
  // Preserve terminal control keys; only desktop copy/paste shortcuts are intercepted.
  for (const e of [
    { key: "c", ctrlKey: true }, { key: "v", ctrlKey: true },
    { key: "ArrowUp" }, { key: "ArrowLeft", altKey: true },
    { key: "r", ctrlKey: true },
  ]) assert.equal(terminal.key({ type: "keydown", ...e }), true);
  for (const input of ["\x03", "\x1b[A", "\x1bb", "\x1b[1;5D"]) {
    terminal.input(input);
    assert.equal(written.at(-1), input);
  }
  await $("terminal-paste").onclick();
  assert.equal(terminal.pasted, "first line\nsecond line");
  assert.equal(written.at(-1), "\x1b[200~first line\nsecond line\x1b[201~");
  await $("terminal-copy").onclick();
  assert.equal(copied.at(-1), "selected output");
  let prevented = false;
  assert.equal(terminal.key({ type: "keydown", key: "c", metaKey: true,
    preventDefault() { prevented = true; } }), false);
  assert.equal(prevented, true);
  const input = w.document.createElement("textarea");
  $("terminal-screen").append(input);
  const ctrlR = new w.KeyboardEvent("keydown", { key: "r", ctrlKey: true, bubbles: true, cancelable: true });
  input.dispatchEvent(ctrlR);
  assert.equal(ctrlR.defaultPrevented, false);
  if (webgl) {
    webgl.lost();
    assert.equal(webgl.disposed, true);
    assert.equal(closed, closes);
  }
  sessionEnded = true;
  await wait(() => $("terminal-status").textContent === "Exited");
  await $("terminal-close").onclick();
  assert.equal($("agent-terminal").hidden, true);
  assert.equal($("agent-running-dot").hidden, true);
  assert.equal($("agent-page").hidden, false);
  $("overview").click();
  await $("scan-agent-link").onclick();
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
  assert.equal(w.localStorage.getItem("omd-agent-choice"), "");
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
  const row = box.closest(".suggestion-card"), details = row.nextElementSibling;
  row.click();
  assert.equal(details.hidden, false);
  box.click();
  assert.equal(details.hidden, false);
  $("trash-selected").click();
  await $("confirm-trash").onclick();
  assert.equal(trashCalls, 1);
  assert.match($("trash-results").textContent, /moved to Trash. Review more items or scan again/);
  assert.ok($("trash-results").querySelector("details"));
  assert.equal($("selection-bar").hidden, true);
  assert.deepEqual(errors, []);
  w.close();
  console.log(
    "PASS: install states, detect on open, command-bar folder selection, scan-to-agent handoff, Agents page navigation keeps polling, resize, control keys, UTF-8 output, bracketed paste, copy, stop/reopen, cancelled scan never launches. Native terminal mocked.",
  );
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
