"use strict";
const nativeApp = window.go?.main?.App;
function itemActions(path, command) {
  const group = dom("div", "item-actions");
  const action = (label, fn) => {
    const b = dom("button", "", label);
    b.onclick = () =>
      fn().catch((e) => {
        $("desktop-error").textContent = /older cache format/i.test(String(e))
          ? "This saved scan uses an older format. Scan again to update it."
          : String(e);
        showFeedback(String(e));
      });
    group.append(b);
  };
  action("Reveal in Finder", () => nativeApp.Reveal(path));
  action("Copy path", () => nativeApp.Copy(path));
  if (command) action("Copy suggested command", () => nativeApp.Copy(command));
  return group;
}
if (nativeApp) {
  $("desktop").hidden = false;
  let home = "",
    chosen = "",
    revision = 0,
    loading = false,
    opened = false;
  const reportError = (e) => {
    $("desktop-error").textContent = /older cache format/i.test(String(e))
      ? "This saved scan uses an older format. Scan again to update it."
      : String(e);
    showFeedback(String(e));
  };
  function showResults(show) {
    $("content-grid").hidden = !show;
    $("welcome").hidden = show;
    document.querySelector(".view-switch").hidden = true;
    if (show && snapshot) {
      chosen = snapshot.scan.root.path;
      $("scan-root").value = "custom";
      $("chosen-path").textContent = chosen;
      showMode(mode);
    }
  }
  showResults(false);
  scanState(false);
  async function pick() {
    const path = await nativeApp.PickFolder();
    if (path) {
      chosen = path;
      $("scan-root").value = "custom";
      $("chosen-path").textContent = path;
    }
  }
  $("choose-folder").onclick = () => pick().catch(reportError);
  $("scan-root").onchange = () => {
    if ($("scan-root").value === "custom") pick().catch(reportError);
    else $("chosen-path").textContent = "";
  };
  $("start-scan").onclick = async () => {
    try {
      let root = $("scan-root").value;
      if (root === "home") root = home;
      if (root === "custom") {
        if (!chosen) await pick();
        root = chosen;
      }
      if (!root) return;
      $("desktop-error").textContent = "";
      await nativeApp.Scan(root);
    } catch (e) {
      reportError(e);
    }
  };
  $("cancel-scan").onclick = () => nativeApp.Cancel().catch(reportError);
  $("open-last").onclick = async () => {
    await loadSnapshot();
    opened = true;
    showResults(true);
    document.querySelector(".scan-options").open = false;
  };
  $("full-disk-access").onclick = () =>
    nativeApp.FullDiskAccess().catch(reportError);
  for (const [id, summary] of [
    ["export-json", false],
    ["export-summary", true],
  ])
    $(id).onclick = async () => {
      try {
        const path = await nativeApp.Export(summary);
        if (path) $("desktop-error").textContent = "Exported to " + path;
      } catch (e) {
        reportError(e);
      }
    };
  async function poll() {
    try {
      const s = await nativeApp.State();
      home = s.home;
      $("start-scan").disabled = s.busy;
      $("hub-scan").disabled = s.busy;
      $("choose-folder").disabled = s.busy;
      $("scan-root").disabled = s.busy;
      $("start-scan").textContent = s.hasScan ? "Scan Again" : "Scan";
      if (document.body.classList.contains("is-scanning") !== s.busy)
        scanState(s.busy);
      if (s.busy) {
        $("scan-feedback").hidden = false;
        $("scan-feedback").textContent =
          `${s.phase}, ${s.entries.toLocaleString()} entries, ${s.directories.toLocaleString()} folders`;
        document.querySelector(".scan-options").open = true;
      }
      $("toolbar-choose").disabled = s.busy;
      if (!opened && s.hasScan && !s.busy)
        document.querySelector(".scan-options").open = true;
      $("cancel-scan").hidden =
        !s.busy || s.phase === "Opening last cached scan";
      $("scan-spinner").hidden = !s.busy;
      $("open-last").hidden = !s.hasScan || opened;
      $("export-json").disabled = !s.hasScan;
      $("export-summary").disabled = !s.hasScan;
      $("progress").textContent = s.busy
        ? `${s.phase} · ${s.entries.toLocaleString()} entries · ${s.directories.toLocaleString()} folders · ${Math.round(s.rate).toLocaleString()} entries/s · ${s.path}`
        : s.phase;
      if (s.error) reportError(s.error);
      $("access-banner").hidden = !s.protected.length;
      if (s.revision !== revision && !loading) {
        // A new result may publish just before the final phase update.
        // Wait for Complete; retain an unopened launch cache on the welcome screen.
        if (opened || s.phase === "Complete") {
          loading = true;
          try {
            await loadSnapshot();
            revision = s.revision;
            opened = true;
            showResults(true);
            document.querySelector(".scan-options").open = false;
          } finally {
            loading = false;
          }
        } else if (!s.busy) revision = s.revision;
      }
    } catch (e) {
      reportError(e);
    }
    setTimeout(poll, 400);
  }
  poll();
}
