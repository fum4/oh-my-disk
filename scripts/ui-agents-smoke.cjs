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
    launched = [],
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
          w.localStorage.setItem("omd-agent-tip-dismissed", "1");
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
              Agents: async () => registry.map(d => ({ ...d, path: d.name === "claude" && available ? "/mock/claude" : "" })),
              Scan: async () => {
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
  const option = name => [...$("agent-options").querySelectorAll("button")].find(b => b.textContent.includes(name));
  await wait(() => $("agent-options").textContent.includes("Claude"));
  assert.equal($("ask-agent").hidden, true);
  assert.equal($("agent-control").parentElement.id, "scan-agent-slot");
  for (const id of ["folders", "categories", "cleanup-tab", "overview"]) {
    $(id).click();
    assert.equal($("ask-agent").hidden, true);
    assert.equal($("agent-control").parentElement.id, "scan-agent-slot");
    assert.equal($("launch-cards").hidden, id !== "overview");
    assert.equal($("empty-actions").hidden, id === "overview");
  }
  if (process.argv.includes("--remembered") || process.argv.includes("--missing-remembered")) {
    assert.equal($("agent-choice").textContent, available ? "Claude" : "None");
    assert.equal($("agent-tip").hidden, true);
    assert.equal(w.localStorage.getItem("omd-agent-choice"), available ? "claude" : "");
    if (!available) {
      available = true;
      await $("agent-check").onclick();
      assert.equal($("agent-choice").textContent, "None");
    }
    w.close();
    console.log("PASS: remembered choice, unavailable fallback, once-only nudge, pre-scan visibility on every page.");
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

  assert.equal(w.document.querySelectorAll(".launch-card").length, 1);
  assert.equal(option("Claude"), undefined);
  assert.ok($("agent-options").querySelector('a[aria-label="Install Claude"]'));
  assert.equal($("agent-tip").hidden, false);
  $("agent-dismiss").click();
  assert.equal(w.localStorage.getItem("omd-agent-tip-dismissed"), "1");
  available = true;
  await $("agent-check").onclick();
  assert.ok(option("Claude"));
  assert.equal(option("Claude").querySelector("use").getAttribute("href"), "#agent-claude");
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
  await wait(() => $("terminal-status").textContent === "Read-only disk advisor");
  assert.match(terminal.options.fontFamily, /SF Mono.*Menlo/);
  assert.equal(terminal.options.theme.background, "#15181e");
  assert.equal(terminal.options.cursorBlink, true);
  assert.equal(terminal.options.macOptionIsMeta, true);
  assert.equal(terminal.unicode.activeVersion, "11");
  assert.match(Buffer.from(terminal.output).toString(), /╭─ Advisor ─╮/);
  assert.ok($("ask-agent").contains($("agent-choice")));
  assert.equal($("agent-picker").querySelector("summary").textContent.trim(), "Analyze with"); // hidden label, visible chevron only
  terminal.resized({ cols: 112, rows: 35 });
  assert.deepEqual(resized.at(-1), [112, 35]);
  // Collapse keeps the same terminal, PTY, and polling loop alive; restore remembers height.
  const panel = $("agent-terminal"), handle = $("terminal-resize");
  panel.getBoundingClientRect = () => ({ height: 340 });
  const closes = closed, instance = terminal, beforeReads = reads;
  handle.click();
  assert.equal(handle.getAttribute("aria-expanded"), "false");
  assert.equal(panel.classList.contains("collapsed"), true);
  await wait(() => reads > beforeReads);
  assert.equal(closed, closes);
  assert.equal(terminal, instance);
  handle.click();
  assert.equal(handle.getAttribute("aria-expanded"), "true");
  assert.equal(panel.style.height, "340px");
  assert.equal(launched.length, 1);
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
    "PASS: install states, check again, dismissal persistence, scan-to-agent handoff, terminal collapse/restore keeps polling, resize, control keys, UTF-8 output, bracketed paste, copy, close/reopen, cancelled scan never launches. Native terminal mocked.",
  );
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
