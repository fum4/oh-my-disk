"use strict";
(() => {
  const app = window.go?.main?.App;
  const names = {};
  let definitions = [], choice = "";
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
    sessionChoice = "",
    activeAgent = "",
    contextRevision = null;
  const error = (e) => { showFeedback(String(e)); if (mode === "agent") $("terminal-status").textContent = String(e); };
  function select(name) {
    choice = name;
    try { localStorage.setItem("omd-agent-choice", name); } catch {}
    $("agent-picker").open = false;
    render();
    $("agent-picker").querySelector("summary").focus();
  }
  function renderOptions(picker, options, selected, allowNone, choose) {
    const signature = JSON.stringify([definitions, installed, selected]);
    if (options.dataset.state === signature) return;
    options.dataset.state = signature;
    options.replaceChildren();
    const add = (name, label, glyph) => {
      const b = dom("button", "agent-option");
      const text = dom("span", "agent-option-label", label);
      if (glyph) text.prepend(icon(glyph));
      b.append(text);
      b.type = "button";
      b.setAttribute("aria-pressed", String(selected === name));
      b.onclick = () => choose(name);
      options.append(b);
    };
    if (allowNone) add("", "None");
    for (const d of definitions) {
      if (app && installed[d.name]) add(d.name, d.label, d.icon);
      else {
        const row = dom("div", "agent-option unavailable");
        const label = dom("span", "agent-option-label", d.label);
        label.prepend(icon(d.icon));
        const link = dom("a", "", "Install ↗");
        link.href = d.installURL; link.target = "_blank"; link.rel = "noopener";
        link.setAttribute("aria-label", "Install " + d.label);
        if (app) link.onclick = e => { e.preventDefault(); window.runtime.BrowserOpenURL(link.href); };
        row.append(label, link); options.append(row);
      }
    }
  }
  function mark(id, name) {
    $(id).replaceChildren();
    const d = definitions.find(d => d.name === name);
    if (d) $(id).append(icon(d.icon));
  }
  function render() {
    const busy = !!state.busy || document.body.classList.contains("is-scanning");
    const ready = !!snapshot;
    if (busy) $("agent-picker").open = false;
    if (opening) $("session-picker").open = false;
    $("session-picker").querySelector("summary").setAttribute("aria-disabled", String(opening));
    const selected = installed[choice] ? choice : "";
    const control = $("agent-control");
    if (control.parentElement !== $("scan-agent-slot")) $("scan-agent-slot").append(control);
    control.hidden = busy;
    $("agent-choice").textContent = names[selected] || "None";
    mark("agent-mark", selected);
    $("agent-picker").querySelector("summary").setAttribute("aria-label", "Choose agent, current: " + (names[selected] || "None"));
    renderOptions($("agent-picker"), $("agent-options"), selected, true, select);
    renderOptions($("session-picker"), $("session-options"), sessionChoice, false, async name => {
      if (opening) return;
      if (name === sessionChoice) { $("session-picker").open = false; return; }
      if (name !== sessionChoice && sessionRunning && !await confirmEnd("Switching agents ends the current conversation.")) return;
      const hadSession = !!term;
      if (name !== sessionChoice && hadSession) await close(false);
      sessionChoice = name;
      $("session-picker").open = false;
      render();
      $("session-picker").querySelector("summary").focus();
      if (hadSession) launch(name);
    });
    $("session-choice").textContent = names[sessionChoice] || "Choose agent";
    mark("session-mark", sessionChoice);
    $("agent-running-dot").hidden = !sessionRunning;
    $("terminal-close").disabled = !sessionRunning || opening;
    $("terminal-restart").disabled = !app || !installed[sessionChoice] || opening;
    $("terminal-restart").hidden = !term;
    $("agent-empty").hidden = !!term;
    $("agent-nudge").textContent = ready ? "Your scan is ready. Ask what’s taking space or where to start." : "Scan first to give your agent a clear picture of your disk.";
    $("agent-start").textContent = ready ? "Start session" : "Start without a scan";
    $("agent-start").disabled = !app || !installed[sessionChoice] || opening;
    $("agent-desktop-hint").hidden = !!app;
    $("scan-agent-link").textContent = "Ask " + (names[sessionChoice || selected] || "agent") + " about this scan →";
    $("plain-scan").disabled = busy || opening;
    if (mode === "agent") requestAnimationFrame(resize);
  }
  function confirmEnd(message) {
    $("agent-confirm-message").textContent = message;
    return new Promise(resolve => {
      const dialog = $("agent-confirm");
      const done = value => { dialog.close(); resolve(value); };
      $("agent-keep").onclick = () => done(false);
      $("agent-replace").onclick = () => done(true);
      dialog.oncancel = e => { e.preventDefault(); done(false); };
      dialog.showModal();
    });
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
      if (!sessionChoice || !installed[sessionChoice]) sessionChoice = choice || definitions.find(d => installed[d.name])?.name || "";
      render();
    } catch (e) { if (request === detection) error(e); }
  }
  function theme() {
    if (!window.matchMedia?.("(prefers-color-scheme: dark)").matches) return {
      background: "#fafbfc", foreground: "#262d38", cursor: "#262d38", cursorAccent: "#fafbfc",
      selectionBackground: "#adc8ec80", black: "#262d38", red: "#b53032", green: "#267a43",
      yellow: "#8b6300", blue: "#245cb5", magenta: "#80529a", cyan: "#227b86", white: "#d3d8e0",
      brightBlack: "#697586", brightRed: "#c83c3e", brightGreen: "#247c42", brightYellow: "#926d13",
      brightBlue: "#2868c9", brightMagenta: "#8b54aa", brightCyan: "#25818b", brightWhite: "#f3f5f8",
    };
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
    if (!term || mode !== "agent" || $("agent-terminal").hidden) return;
    fit.fit();
  }
  async function openScanAgent(n) {
    showMode("agent");
    if (!installed[n]) { $("session-picker").open = true; return; }
    const revision = snapshot?.scan?.started || state.revision;
    if (term && activeAgent === n && contextRevision === revision) { resize(); term.focus(); return; }
    if (sessionRunning && !await confirmEnd("Load this scan in a new session? The running conversation will end.")) return;
    sessionChoice = n;
    await launch(n);
  }
  async function launch(n, prompt = "") {
    if (opening || !app || !installed[n]) return;
    opening = true;
    render();
    try {
      await close(false);
      const gen = ++generation;
      showMode("agent");
      activeAgent = sessionChoice = n;
      contextRevision = snapshot?.scan?.started || state.revision;
      $("agent-terminal").hidden = false;
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
      $("terminal-status").textContent = "Running";
      term.focus();
      async function poll() {
        try {
          const out = await app.ReadAgent();
          if (gen !== generation) return;
          if (out.data)
            term.write(Uint8Array.from(atob(out.data), (c) => c.charCodeAt(0)));
          if (out.exited) {
            sessionRunning = false;
            $("terminal-status").textContent = "Exited";
            render();
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
    $("terminal-status").textContent = "Stopped";
    render();
    if (restore) $("agent-start").focus();
  }
  $("scan-agent-link").onclick = () => openScanAgent(sessionChoice || choice).catch(error);
  $("agent-start").onclick = () => launch(sessionChoice);
  $("terminal-restart").onclick = async () => {
    if (sessionRunning && !await confirmEnd("Restarting ends the current conversation and loads your latest scan.")) return;
    await launch(sessionChoice);
  };
  window.agentViewChanged = render;
  const scan = $("hub-scan").onclick;
  $("hub-scan").onclick = async () => {
    if (mode === "agent") showMode("overview");
    pending = !snapshot && installed[choice] ? choice : "";
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
      openScanAgent(n).catch(error);
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
  floatingMenu($("session-picker"), () => detect());
  $("session-picker").querySelector("summary").onclick = e => { if (opening) e.preventDefault(); };
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
  if (window.ResizeObserver)
    new ResizeObserver(resize).observe($("terminal-screen"));
  window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", () => { if (term) term.options.theme = theme(); });
  detect().catch(error);
})();
