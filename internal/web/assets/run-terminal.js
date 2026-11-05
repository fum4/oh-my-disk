"use strict";
// The floating terminal that shows a Run command's live output. It can be minimized while
// the command keeps going; closing it stops the command.
(() => {
  const POLL_MS = 60;
  const STATE_ICONS = { done: "fill-check-circle", failed: "fill-warning-circle" };
  let term, fit, timer, pending = Promise.resolve();
  let running = false, closed = true;

  const app = () => window.go.main.App;

  function ensureTerminal() {
    if (term) return;
    term = new Terminal({
      fontFamily: TERMINAL_FONT,
      fontSize: 12,
      scrollback: 5000,
      theme: terminalTheme(),
      disableStdin: true,
      allowProposedApi: true,
    });
    fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new Unicode11Addon.Unicode11Addon());
    term.unicode.activeVersion = "11";
    term.open($("run-terminal-screen"));
  }

  function setMinimized(minimized) {
    $("run-terminal").classList.toggle("minimized", minimized);
    document.body.classList.toggle("run-terminal-minimized", minimized);
    const button = $("run-terminal-minimize");
    button.title = minimized ? "Expand" : "Minimize";
    button.setAttribute("aria-label", button.title);
    button.setAttribute("aria-expanded", String(!minimized));
    button.replaceChildren(icon(minimized ? "regular-caret-up" : "regular-minus"));
  }

  function setStatus(state, text) {
    $("run-terminal").dataset.state = state;
    $("run-terminal-status").textContent = text;
    const name = STATE_ICONS[state];
    $("run-terminal-icon").replaceChildren(name ? icon(name) : dom("span", "run-spinner"));
    const close = $("run-terminal-close");
    close.title = state === "running" ? "Stop and close" : "Close";
    close.setAttribute("aria-label", close.title);
  }

  async function drain() {
    const data = await app().ReadCleanup();
    if (data) term.write(Uint8Array.from(atob(data), (c) => c.charCodeAt(0)));
  }

  function poll() {
    pending = drain().catch(() => {}).then(() => {
      if (running) timer = setTimeout(poll, POLL_MS);
    });
  }

  // Shows the terminal for a new command and returns the size the command should run at.
  function start(command) {
    closed = false;
    running = true;
    $("run-terminal").hidden = false;
    document.body.classList.add("run-terminal-open");
    setMinimized(false);
    setStatus("running", "Running…");
    const label = $("run-terminal-command");
    label.textContent = label.title = command;
    ensureTerminal();
    term.reset();
    term.write("\x1b[?25l"); // nothing to type here, so no cursor
    fit.fit();
    term.write("\x1b[2m$\x1b[0m " + command + "\r\n");
    poll();
    return { cols: term.cols, rows: term.rows };
  }

  // Reports the command's outcome, after any output still on its way.
  async function finish(state, text, detail = "") {
    running = false;
    clearTimeout(timer);
    await pending;
    await drain().catch(() => {});
    if (closed) return;
    if (detail) term.write("\x1b[31m" + detail.replace(/[\x00-\x1f\x7f]/g, " ") + "\x1b[0m\r\n");
    setStatus(state, text);
  }

  function close() {
    if (running) app().CancelCleanup();
    closed = true;
    $("run-terminal").hidden = true;
    document.body.classList.remove("run-terminal-open", "run-terminal-minimized");
  }

  $("run-terminal-minimize").onclick = (e) => {
    e.stopPropagation();
    setMinimized(!$("run-terminal").classList.contains("minimized"));
  };
  $("run-terminal-close").onclick = (e) => {
    e.stopPropagation();
    close();
  };
  $("run-terminal-header").onclick = () => {
    if ($("run-terminal").classList.contains("minimized")) setMinimized(false);
  };
  window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", () => {
    if (term) term.options.theme = terminalTheme();
  });
  window.runTerminal = { start, finish };
})();
