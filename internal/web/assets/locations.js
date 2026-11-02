"use strict";
(() => {
  const key = "omd-locations-v1";
  let locations = ["home", "/"],
    removed,
    timer;
  try {
    const saved = JSON.parse(localStorage.getItem(key));
    if (Array.isArray(saved))
      locations = [
        ...new Set(
          saved.filter(
            (p) => typeof p === "string" && (p === "home" || p.startsWith("/")),
          ),
        ),
      ];
  } catch {}
  const label = (path) =>
    path === "home"
      ? "Home"
      : path === "/"
        ? "Macintosh HD"
        : path.split("/").filter(Boolean).pop();
  function save() {
    try {
      localStorage.setItem(key, JSON.stringify(locations));
    } catch {}
  }
  function render() {
    $("locations").replaceChildren();
    $("folder-options").replaceChildren();
    $("rescan-options").replaceChildren();
    for (const path of locations) {
      const option = dom("button", "agent-option", label(path));
      option.title = path === "home" ? "Home" : path;
      option.onclick = () => {
        if (window.selectScanTarget) window.selectScanTarget(path);
        else {
          $("web-root").value = path === "home" ? snapshot?.scan.root.path || "" : path;
          $("folder-choice").textContent = label(path);
        }
        $("folder-picker").open = false;
        $("folder-picker").querySelector("summary").focus();
      };
      $("folder-options").append(option);
      const saved = option.cloneNode(true);
      saved.onclick = () => {
        option.onclick();
        $("rescan-picker").open = false;
        $("rescan-picker").querySelector("summary").focus();
      };
      $("rescan-options").append(saved);
      const row = dom("div", "location-row");
      row.setAttribute("role", "listitem");
      const b = dom("button", "nv location-open");
      if (path === "home") b.id = "location-home";
      if (path === "/") b.id = "location-disk";
      b.append(
        icon(
          path === "home"
            ? "fill-house"
            : path === "/"
              ? "fill-hard-drive"
              : "fill-folder",
        ),
        dom("span", "location-name", label(path)),
      );
      b.title = path === "home" ? "Scan Home" : "Scan " + path;
      b.onclick = async (e) => {
        e?.stopPropagation();
        if ($("hub-scan").disabled) return;
        if (window.selectScanTarget) window.selectScanTarget(path);
        else
          $("web-root").value =
            path === "home" ? snapshot?.scan.root.path || "" : path;
        await $("hub-scan").onclick();
      };
      const remove = dom("button", "location-remove icon-action");
      remove.append(icon("regular-x"));
      remove.title = "Remove from Locations";
      remove.setAttribute(
        "aria-label",
        "Remove " + label(path) + " from Locations",
      );
      remove.onclick = (e) => {
        e?.stopPropagation();
        removed = { path, index: locations.indexOf(path) };
        locations = locations.filter((p) => p !== path);
        save();
        render();
        $("location-toast").hidden = false;
        clearTimeout(timer);
        timer = setTimeout(() => {
          $("location-toast").hidden = true;
          removed = null;
        }, 6500);
        $("undo-location").focus();
      };
      row.onclick = () => b.click();
      row.append(b, remove);
      $("locations").append(row);
    }
  }
  window.addLocation = (path) => {
    // Store Home as a stable favorite rather than adding a duplicate absolute path.
    if (path === window.scanHome) path = "home";
    if (!locations.includes(path)) {
      locations.push(path);
      save();
      render();
    }
  };
  $("undo-location").onclick = () => {
    if (!removed) return;
    if (!locations.includes(removed.path))
      locations.splice(
        Math.min(removed.index, locations.length),
        0,
        removed.path,
      );
    save();
    render();
    clearTimeout(timer);
    $("location-toast").hidden = true;
    removed = null;
    $("locations").querySelector("button")?.focus();
  };
  render();
})();
