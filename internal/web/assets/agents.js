"use strict";
(() => {
  const app = window.go?.main?.App;
  const names = { claude: "Claude", codex: "Codex" };
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
  function render() {
    for (const [n, label] of Object.entries(names)) {
      const found = !!installed[n];
      $("ask-" + n).disabled =
        !app || !found || !state.hasScan || state.busy || opening;
      $("ask-" + n).title = app
        ? found
          ? "Chat about the displayed scan"
          : "Install " + label + " and check again"
        : "Available in the desktop app";
      const b = $("scan-" + n);
      b.disabled = !app || !found || state.busy || opening;
      b.textContent = !app
        ? "Desktop app required"
        : found
          ? "Scan with " + label
          : "Install " + label;
      $("install-" + n).hidden = !app || found;
      $("tip-install-" + n).hidden = !app || found;
    }
    $("plain-scan").disabled = !!state.busy;
    $("agent-check").hidden = !app;
    $("agent-tip-text").textContent = !app
      ? "Available in the desktop app"
      : "Install Claude Code or Codex CLI to chat about your disk right in the app. Install either CLI, then Check again.";
    $("agent-tip").hidden =
      dismissed() || (app && installed.claude && installed.codex);
  }
  async function detect() {
    if (app) {
      $("agent-check").disabled = true;
      try {
        installed = Object.fromEntries(
          (await app.Agents()).map((c) => [c.name, c.path]),
        );
      } finally {
        $("agent-check").disabled = false;
      }
    }
    render();
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
    if (opening) return;
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
    if (restore) $("ask-claude").focus();
  }
  for (const n of Object.keys(names)) {
    $("ask-" + n).onclick = () => launch(n);
    $("scan-" + n).onclick = async () => {
      pending = n;
      scanStarted = false;
      await $("start-scan").onclick();
    };
  }
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
  $("plain-scan").onclick = () => {
    pending = "";
    $("hub-scan").click();
  };
  $("agent-check").onclick = () => detect().catch(error);
  $("agent-dismiss").onclick = () => {
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
  if (app)
    for (const n of Object.keys(names))
      for (const prefix of ["install-", "tip-install-"])
        $(prefix + n).onclick = (e) => {
          e.preventDefault();
          window.runtime.BrowserOpenURL(e.currentTarget.href);
        };
  detect().catch(error);
})();
