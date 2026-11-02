"use strict";
(() => {
  const app = window.go?.main?.App;
  const names = {};
  let definitions = [], choice = "", showNudge = false, detected = false, optionState = "";
  try { choice = localStorage.getItem("omd-agent-choice") || ""; } catch {}
  let installed = {},
    state = {},
    pending = "",
    scanStarted = false,
    term,
    fit,
    timer,
    opening = false,
    generation = 0;
  const tipKey = "omd-agent-tip-dismissed";
  const dismissed = () => {
    try {
      return localStorage.getItem(tipKey) === "1";
    } catch {
      return false;
    }
  };
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
    const available = definitions.some(d => installed[d.name]);
    if (busy) $("agent-picker").open = false;
    const selected = installed[choice] ? choice : "";
    const slot = !ready ? "scan-agent-slot" : ["overview", "cleanup-tab"].includes(mode) ? "results-agent-slot" : "toolbar-agent-slot";
    const control = $("agent-control");
    if (control.parentElement !== $(slot)) $(slot).append(control);
    control.hidden = busy;
    $("ask-agent").hidden = !ready;
    $("ask-agent").disabled = !app || opening;
    $("ask-agent").title = app ? "Chat about these results" : "Available in the desktop app";
    $("agent-choice").textContent = names[selected] || "None";
    $("agent-picker").querySelector("summary").title = app ? "" : "Agents run in the desktop app";
    const signature = JSON.stringify([definitions, installed, selected]);
    const options = $("agent-options");
    if (signature !== optionState) {
      optionState = signature;
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
    }
    $("agent-check").hidden = !app;
    $("plain-scan").disabled = busy || opening;
    $("agent-tip").hidden = !app || ready || busy || available || !showNudge || !definitions.length;
  }
  async function detect() {
    $("agent-check").disabled = true;
    try {
      // The same Go registry supplies desktop and browser presentation.
      definitions = app ? await app.Agents() : await (await fetch("/api/agents")).json();
      for (const d of definitions) names[d.name] = d.label;
      if (app) installed = Object.fromEntries(definitions.map(c => [c.name, c.path]));
      if (!detected) {
        showNudge = !!app && !definitions.some(d => installed[d.name]) && !dismissed();
        detected = true;
      }
      render();
      if (!$("agent-tip").hidden) {
        try { localStorage.setItem(tipKey, "1"); } catch {}
      }
    } finally { $("agent-check").disabled = false; }
  }
  function theme() {
    const c = getComputedStyle(document.documentElement);
    return {
      background: c.getPropertyValue("--card").trim(),
      foreground: c.getPropertyValue("--label").trim(),
      cursor: c.getPropertyValue("--blue").trim(),
      selectionBackground: "#0a66e84d",
    };
  }
  function resize() {
    if (!term || $("agent-terminal").hidden) return;
    fit.fit();
    app.ResizeAgent(term.cols, term.rows).catch(error);
  }
  async function launch(n, prompt = "") {
    if (opening || !snapshot || state.busy || !installed[n]) return;
    opening = true;
    render();
    try {
      await close(false);
      const gen = ++generation;
      $("agent-terminal").hidden = false;
      $("terminal-title").textContent = names[n];
      $("terminal-status").textContent = "Starting…";
      term = new Terminal({
        fontFamily: '"JetBrains Mono", monospace',
        fontSize: 13,
        scrollback: 10000,
        theme: theme(),
        allowProposedApi: false,
      });
      fit = new FitAddon.FitAddon();
      term.loadAddon(fit);
      term.loadAddon(
        new WebLinksAddon.WebLinksAddon((event, url) => {
          if (/^https?:\/\//i.test(url)) window.runtime.BrowserOpenURL(url);
        }),
      );
      term.open($("terminal-screen"));
      fit.fit();
      term.attachCustomKeyEventHandler((e) => {
        if (
          e.type === "keydown" &&
          (e.metaKey || e.ctrlKey) &&
          e.key.toLowerCase() === "v"
        ) {
          e.preventDefault();
          $("terminal-paste").click();
          return false;
        }
        if (
          e.type === "keydown" &&
          e.metaKey &&
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
      term.onData((data) => app.WriteAgent(data).catch(error));
      $("terminal-status").textContent = "Read-only disk advisor";
      term.focus();
      async function poll() {
        try {
          const out = await app.ReadAgent();
          if (gen !== generation) return;
          if (out.data)
            term.write(Uint8Array.from(atob(out.data), (c) => c.charCodeAt(0)));
          if (out.exited) {
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
    else { $("agent-picker").open = true; $("agent-options").querySelector("button")?.focus(); }
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
  document.addEventListener("click", e => {
    if (!$("agent-picker").contains(e.target)) $("agent-picker").open = false;
  });
  document.addEventListener("keydown", e => {
    if (e.key === "Escape" && $("agent-picker").open) {
      $("agent-picker").open = false;
      $("agent-picker").querySelector("summary").focus();
    }
  });
  $("agent-check").onclick = () => detect().catch(error);
  $("agent-dismiss").onclick = () => {
    showNudge = false;
    try {
      localStorage.setItem(tipKey, "1");
    } catch {}
    $("agent-tip").hidden = true;
  };
  $("terminal-close").onclick = () => close().catch(error);
  $("terminal-copy").onclick = () =>
    app.Copy(term?.getSelection() || "").catch(error);
  $("terminal-paste").onclick = async () => {
    try {
      await app.WriteAgent(await app.Paste());
      term?.focus();
    } catch (e) {
      error(e);
    }
  };
  const handle = $("terminal-resize"),
    panel = $("agent-terminal");
  function setHeight(h) {
    panel.style.height = Math.max(180, Math.min(innerHeight - 100, h)) + "px";
    resize();
  }
  handle.onpointerdown = (e) => {
    handle.setPointerCapture(e.pointerId);
    handle.onpointermove = (ev) =>
      setHeight(panel.getBoundingClientRect().bottom - ev.clientY);
    handle.onpointerup = () => {
      handle.onpointermove = null;
    };
  };
  handle.onkeydown = (e) => {
    if (e.key === "ArrowUp" || e.key === "ArrowDown") {
      e.preventDefault();
      setHeight(panel.offsetHeight + (e.key === "ArrowUp" ? 24 : -24));
    }
  };
  if (window.ResizeObserver)
    new ResizeObserver(resize).observe($("terminal-screen"));
  window
    .matchMedia?.("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      if (term) term.options.theme = theme();
    });
  detect().catch(error);
})();
