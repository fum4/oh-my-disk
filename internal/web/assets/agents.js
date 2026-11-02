"use strict";
(() => {
  const app = window.go?.main?.App;
  const names = {};
  let definitions = [], choice = "", optionState = "";
  try { choice = localStorage.getItem("omd-agent-choice") || ""; } catch {}
  let installed = {},
    state = {},
    pending = "",
    scanStarted = false,
    term,
    fit,
    timer,
    opening = false,
    generation = 0,
    sessionRunning = false,
    collapsed = false,
    expandedHeight = "",
    dragged = false;
  const error = (e) => showFeedback(String(e));
  function select(name) {
    choice = name;
    try { localStorage.setItem("omd-agent-choice", name); } catch {}
    $("agent-picker").open = false;
    render();
    $("agent-picker").querySelector("summary").focus();
  }
  function render() {
    const busy = !!state.busy || document.body.classList.contains("is-scanning");
    const ready = !!snapshot;
    if (busy) $("agent-picker").open = false;
    const selected = installed[choice] ? choice : "";
    const slot = !ready ? "scan-agent-slot" : "toolbar-agent-slot";
    const control = $("agent-control");
    if (control.parentElement !== $(slot)) $(slot).append(control);
    control.hidden = busy;
    $("ask-agent").hidden = !ready;
    $("ask-agent").disabled = !app || opening;
    $("ask-agent").title = app ? "Chat about these results" : "Available in the desktop app";
    $("agent-choice").textContent = names[selected] || "None";
    $("agent-ask-label").textContent = "Ask agent";
    $("agent-selection").hidden = ready && !selected;
    const summary = $("agent-picker").querySelector("summary");
    $("agent-analyze").hidden = ready;
    control.classList.toggle("has-results", ready);
    if (ready) $("ask-agent").append($("agent-selection"));
    else summary.insertBefore($("agent-selection"), summary.querySelector(".agent-chevron"));
    $("agent-mark").replaceChildren();
    const definition = definitions.find(d => d.name === selected);
    if (definition) $("agent-mark").append(icon(definition.icon));
    summary.setAttribute("aria-label", "Choose agent, current: " + (names[selected] || "None"));
    $("ask-agent").setAttribute("aria-label", selected ? "Ask " + names[selected] : "Choose an agent to ask");
    $("agent-picker").querySelector("summary").title = app ? "" : "Agents run in the desktop app";
    const signature = JSON.stringify([definitions, installed, selected]);
    const options = $("agent-options");
    if (signature !== optionState) {
      optionState = signature;
      const hadFocus = options.contains(document.activeElement);
      options.replaceChildren();
      const add = (name, label, glyph = "") => {
        const b = dom("button", "agent-option");
        const text = dom("span", "agent-option-label", label);
        if (glyph) text.prepend(icon(glyph));
        b.append(text);
        b.type = "button";
        b.setAttribute("aria-pressed", String(selected === name));
        b.onclick = () => select(name);
        options.append(b);
      };
      add("", "None");
      for (const d of definitions) {
        if (app && installed[d.name]) add(d.name, d.label, d.icon);
        else {
          const row = dom("div", "agent-option unavailable");
          const label = dom("span", "agent-option-label", d.label);
          label.prepend(icon(d.icon));
          row.append(label);
          const link = dom("a", "", "Install ↗");
          link.href = d.installURL;
          link.target = "_blank";
          link.rel = "noopener";
          link.setAttribute("aria-label", "Install " + d.label);
          if (app) link.onclick = e => { e.preventDefault(); window.runtime.BrowserOpenURL(link.href); };
          row.append(link);
          options.append(row);
        }
      }
      if (hadFocus) options.querySelector('button[aria-pressed="true"]')?.focus({ preventScroll: true });
    }
    $("plain-scan").disabled = busy || opening;
  }
  let detection = 0;
  async function detect() {
    const request = ++detection;
    try {
      // The same Go registry supplies desktop and browser presentation.
      const found = app ? await app.Agents() : await (await fetch("/api/agents")).json();
      if (request !== detection) return;
      definitions = found;
      for (const d of definitions) names[d.name] = d.label;
      if (app) installed = Object.fromEntries(definitions.map(c => [c.name, c.path]));
      // A removed CLI falls back to a plain scan, even if installed again later.
      if (choice && !installed[choice]) {
        choice = "";
        try { localStorage.setItem("omd-agent-choice", ""); } catch {}
      }
      render();
    } catch (e) { if (request === detection) error(e); }
  }
  function theme() {
    return {
      background: "#15181e", foreground: "#e4e7ed", cursor: "#e4e7ed",
      cursorAccent: "#15181e", selectionBackground: "#637da666",
      black: "#242933", red: "#f08080", green: "#a3be8c", yellow: "#ebcb8b",
      blue: "#81a9df", magenta: "#b99aca", cyan: "#88c0d0", white: "#d8dee9",
      brightBlack: "#687487", brightRed: "#ff9a9a", brightGreen: "#b8d5a0",
      brightYellow: "#f7dfa4", brightBlue: "#a5c7f4", brightMagenta: "#d2b5e2",
      brightCyan: "#a4dce8", brightWhite: "#ffffff",
    };
  }
  function resize() {
    if (!term || $("agent-terminal").hidden || collapsed) return;
    fit.fit();
  }
  function setCollapsed(value) {
    if (value === collapsed) return;
    if (value) expandedHeight = $("agent-terminal").getBoundingClientRect().height + "px";
    collapsed = value;
    const panel = $("agent-terminal"), handle = $("terminal-resize");
    panel.classList.toggle("collapsed", value);
    panel.style.height = value ? "28px" : expandedHeight;
    handle.setAttribute("aria-expanded", String(!value));
    handle.setAttribute("aria-label", value ? "Expand terminal" : "Collapse terminal");
    handle.title = value ? "Expand terminal" : "Collapse terminal · drag to resize";
    if (!value) { resize(); term?.focus(); }
  }
  async function launch(n, prompt = "") {
    if (opening || !snapshot || state.busy || !installed[n]) return;
    opening = true;
    render();
    try {
      await close(false);
      const gen = ++generation;
      setCollapsed(false);
      $("agent-terminal").hidden = false;
      $("terminal-title").textContent = names[n];
      $("terminal-status").textContent = "Starting…";
      term = new Terminal({
        fontFamily: '"SF Mono", Menlo, Monaco, "JetBrains Mono", monospace',
        fontSize: 13,
        scrollback: 10000,
        theme: theme(),
        cursorBlink: true,
        macOptionIsMeta: true,
        allowProposedApi: true,
      });
      fit = new FitAddon.FitAddon();
      term.loadAddon(fit);
      term.loadAddon(new Unicode11Addon.Unicode11Addon());
      term.unicode.activeVersion = "11";
      term.loadAddon(
        new WebLinksAddon.WebLinksAddon((event, url) => {
          if (/^https?:\/\//i.test(url)) window.runtime.BrowserOpenURL(url);
        }),
      );
      term.open($("terminal-screen"));
      // WebGL gives box drawing and full-screen TUIs a crisp, fast canvas.
      // xterm's built-in DOM renderer remains available on GPU context loss.
      let renderer;
      try {
        renderer = new WebglAddon.WebglAddon();
        renderer.onContextLoss(() => renderer.dispose());
        term.loadAddon(renderer);
      } catch { renderer?.dispose(); }
      fit.fit();
      term.onResize(({ cols, rows }) => {
        if (sessionRunning) app.ResizeAgent(cols, rows).catch(error);
      });
      term.attachCustomKeyEventHandler((e) => {
        if (
          e.type === "keydown" &&
          (e.metaKey || (e.ctrlKey && e.shiftKey)) &&
          e.key.toLowerCase() === "v"
        ) {
          e.preventDefault();
          $("terminal-paste").click();
          return false;
        }
        if (
          e.type === "keydown" &&
          (e.metaKey || (e.ctrlKey && e.shiftKey)) &&
          e.key.toLowerCase() === "c" &&
          term.hasSelection()
        ) {
          e.preventDefault();
          $("terminal-copy").click();
          return false;
        }
        return true;
      });
      await app.StartAgent(n, prompt, term.cols, term.rows);
      if (gen !== generation) {
        await app.CloseAgent();
        return;
      }
      sessionRunning = true;
      term.onData((data) => app.WriteAgent(data).catch(error));
      document.fonts?.ready.then(() => { if (gen === generation) resize(); });
      $("terminal-status").textContent = "Read-only disk advisor";
      term.focus();
      async function poll() {
        try {
          const out = await app.ReadAgent();
          if (gen !== generation) return;
          if (out.data)
            term.write(Uint8Array.from(atob(out.data), (c) => c.charCodeAt(0)));
          if (out.exited) {
            sessionRunning = false;
            $("terminal-status").textContent = "Session ended";
            return;
          }
          timer = setTimeout(poll, 40);
        } catch (e) {
          if (gen === generation) error(e);
        }
      }
      poll();
    } catch (e) {
      await close(false);
      error(e);
    } finally {
      opening = false;
      render();
    }
  }
  async function close(restore = true) {
    generation++;
    sessionRunning = false;
    clearTimeout(timer);
    if (app) await app.CloseAgent();
    term?.dispose();
    term = null;
    $("terminal-screen").replaceChildren();
    $("agent-terminal").hidden = true;
    if (restore) $("ask-agent").focus();
  }
  $("ask-agent").onclick = () => {
    if (installed[choice]) return launch(choice);
    else $("agent-picker").open = true;
  };
  window.agentViewChanged = render;
  const scan = $("hub-scan").onclick;
  $("hub-scan").onclick = async () => {
    pending = installed[choice] ? choice : "";
    scanStarted = false;
    await scan();
  };
  window.agentScanStarted = () => {
    scanStarted = true;
  };
  window.agentScanCancelled = () => {
    pending = "";
    scanStarted = false;
  };
  window.agentScanComplete = () => {
    if (pending && scanStarted) {
      const n = pending;
      pending = "";
      scanStarted = false;
      launch(n);
    }
  };
  window.agentStateChanged = (s) => {
    state = s;
    if (!s.busy && (s.error || /Cancelled|failed/.test(s.phase)))
      window.agentScanCancelled();
    render();
  };
  $("plain-scan").onclick = () => $("hub-scan").onclick();
  floatingMenu($("agent-picker"), () => detect());
  $("terminal-close").onclick = () => close().catch(error);
  $("terminal-copy").onclick = () =>
    app.Copy(term?.getSelection() || "").catch(error);
  $("terminal-paste").onclick = async () => {
    try {
      const target = term;
      const text = await app.Paste();
      if (target && target === term) target.paste(text);
      term?.focus();
    } catch (e) {
      error(e);
    }
  };
  const handle = $("terminal-resize"),
    panel = $("agent-terminal");
  function setHeight(h) {
    if (collapsed) setCollapsed(false);
    panel.style.height = Math.max(180, Math.min(innerHeight * .65, h)) + "px";
    resize();
  }
  handle.onclick = () => { if (!dragged) setCollapsed(!collapsed); };
  handle.onpointerdown = (e) => {
    if (e.button !== 0) return;
    dragged = false;
    const start = e.clientY, height = panel.getBoundingClientRect().height;
    handle.setPointerCapture(e.pointerId);
    handle.onpointermove = (ev) => {
      if (Math.abs(ev.clientY - start) > 4) dragged = true;
      if (dragged) setHeight(height + start - ev.clientY);
    };
    handle.onpointerup = handle.onpointercancel = () => {
      handle.onpointermove = null;
    };
  };
  handle.onkeydown = (e) => {
    dragged = false;
    if (e.key === "ArrowUp" || e.key === "ArrowDown") {
      e.preventDefault();
      setHeight(panel.offsetHeight + (e.key === "ArrowUp" ? 24 : -24));
    }
  };
  if (window.ResizeObserver)
    new ResizeObserver(resize).observe($("terminal-screen"));
  detect().catch(error);
})();
