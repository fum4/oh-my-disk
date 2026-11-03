"use strict";
const nativeApp = window.go?.main?.App;
const accessDismissalKey = "omd-access-warning-dismissed";
let accessDismissed = false;
try {
  accessDismissed = localStorage.getItem(accessDismissalKey) === "1";
} catch (_) {}
$("dismiss-access").onclick = () => {
  accessDismissed = true;
  $("access-banner").hidden = true;
  try {
    localStorage.setItem(accessDismissalKey, "1");
  } catch (_) {}
};
function itemActions(path) {
  const group = dom("div", "item-actions");
  if (!path?.startsWith("/")) return group;
  const action = (label, fn, glyph) => {
    const b = dom(
      "button",
      glyph ? "icon-action" : "",
      glyph ? undefined : label,
    );
    b.title = label;
    b.setAttribute("aria-label", label);
    if (glyph) b.append(icon(glyph));
    b.onclick = async (e) => {
      e?.stopPropagation();
      try {
        await fn();
        if (label.startsWith("Copy")) {
          b.title = "Copied";
          b.setAttribute("aria-label", "Copied");
          b.dataset.copied = "true";
          setTimeout(() => {
            b.title = label;
            b.setAttribute("aria-label", label);
            delete b.dataset.copied;
          }, 1600);
        }
      } catch (e) {
        showFeedback(String(e));
      }
    };
    group.append(b);
  };
  action("Show in Finder", () => nativeApp.Reveal(path), "regular-arrow-square-out");
  action("Copy path", () => nativeApp.Copy(path), "regular-copy");
  return group;
}
if (nativeApp) {
  document.body.classList.add("native-app");
  $("desktop").hidden = false;
  let home = "",
    chosen = "",
    revision = 0,
    loading = false,
    opened = false;
  const reportError = (e) => showFeedback(String(e));
  window.scanTargetLabel = () =>
    $("scan-root").value === "home"
      ? "Home"
      : $("scan-root").value === "/"
        ? "Macintosh HD"
        : chosen.split("/").filter(Boolean).pop() || "Home";
  window.selectScanTarget = (path) => {
    chosen = path === "home" || path === "/" ? "" : path;
    $("scan-root").value = chosen ? "custom" : path;
    $("chosen-path").textContent = chosen;
    try { localStorage.setItem("omd-scan-target", path); } catch {}
    $("scan-root").options[2].textContent = chosen
      ? chosen.split("/").filter(Boolean).pop()
      : "Chosen folder";
    if (!snapshot) showMode(mode);
  };
  function showResults() {
    opened = true;
    $("open-last").hidden = true;
    window.selectScanTarget(
      snapshot.scan.root.path === home ? "home" : snapshot.scan.root.path,
    );
    showMode(mode);
  }
  try { const saved = localStorage.getItem("omd-scan-target"); if (saved) window.selectScanTarget(saved); } catch {}
  showMode(mode);
  scanState(false);
  async function pick() {
    const path = await nativeApp.PickFolder();
    if (path) {
      window.selectScanTarget(path);
      window.addLocation(path);
    }
    return path;
  }
  $("choose-folder").onclick = () => pick().catch(reportError);
  $("scan-another").onclick = async () => {
    try {
      if (await pick()) await $("start-scan").onclick();
    } catch (e) {
      reportError(e);
    }
  };
  $("scan-root").onchange = () => {
    if ($("scan-root").value === "custom" && !chosen) pick().catch(reportError);
    else
      window.selectScanTarget(
        $("scan-root").value === "custom" ? chosen : $("scan-root").value,
      );
  };
  $("start-scan").onclick = async () => {
    try {
      let target = $("scan-root").value;
      if (target === "home") target = home;
      if (target === "custom") {
        if (!chosen) await pick();
        target = chosen;
      }
      if (!target) {
        window.agentScanCancelled?.();
        return;
      }
      $("desktop-error").textContent = "";
      await nativeApp.Scan(target);
      scanState(true);
      window.agentScanStarted?.();
    } catch (e) {
      window.agentScanCancelled?.();
      reportError(e);
    }
  };
  $("cancel-scan").onclick = () => {
    window.agentScanCancelled?.();
    nativeApp.Cancel().catch(reportError);
  };
  $("open-last").onclick = async () => {
    try {
      await loadSnapshot();
      if (snapshot) showResults();
    } catch (e) { reportError(e); }
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
        if (path) showToast("Saved " + path.split("/").pop(), path);
      } catch (e) {
        reportError(e);
      }
    };
  async function poll() {
    try {
      const s = await nativeApp.State();
      home = s.home;
      window.scanHome = home;
      const presentingScan = s.busy || (s.phase === "Complete" && s.revision !== revision);
      window.agentStateChanged?.(s);
      for (const id of [
        "start-scan",
        "hub-scan",
        "choose-folder",
        "scan-root",
        "scan-another",
        "landing-choose",
      ])
        $(id).disabled = presentingScan;
      document
        .querySelectorAll(".location-open")
        .forEach((b) => (b.disabled = presentingScan));
      $("start-scan").textContent = s.hasScan ? "Rescan" : "Scan";
      if (document.body.classList.contains("is-scanning") !== presentingScan)
        scanState(presentingScan);
      $("scan-strip").hidden = !presentingScan;
      $("cancel-scan").hidden =
        !s.busy || s.phase === "Opening last cached scan";
      $("open-last").hidden = !s.hasScan || opened || !!snapshot;
      $("open-last").disabled = presentingScan;
      $("export-json").disabled = !s.hasScan;
      $("export-summary").disabled = !s.hasScan;
      updateScanProgress(s);
      if (s.error) reportError(s.error);
      else if (!s.busy && /Cancel[le]+d/i.test(s.phase)) showFeedback("Scan canceled. Try again or choose a folder.", "info");
      $("access-banner").hidden = accessDismissed || !s.protected.length;
      if (s.revision !== revision && !loading) {
        if (opened || s.phase === "Complete") {
          loading = true;
          try {
            await loadSnapshot();
            revision = s.revision;
            if (snapshot) {
              showResults();
              window.addLocation(snapshot.scan.root.path);
            }
            if (!s.busy) scanState(false);
            window.agentScanComplete?.();
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
