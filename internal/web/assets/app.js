"use strict";
const $ = (id) => document.getElementById(id);
let snapshot,
  root,
  current,
  mode = "overview",
  view = "sunburst",
  sortKey = "bytes",
  sortAsc = false,
  shown = 200;
const categoryLabel = (name) => name ? (/^ai models$/i.test(name) ? "AI models" : name.charAt(0).toUpperCase() + name.slice(1)) : name;
const displayName = (node) => node.category_name ? categoryLabel(node.category_name) : node.name;
const parents = new WeakMap(),
  colors = new WeakMap();
const palette = [
  "#3b6fe0",
  "#6d94e7",
  "#8eaceb",
  "#5b687c",
  "#c8cdd6",
  "#919cab",
  "#637185",
  "#b8c0cb",
  "#a1aab8",
  "#d7dce3",
  "#8490a1",
  "#718096",
];
const fmt = (b) => {
  const base = 1000;
  const units = ["kB", "MB", "GB", "TB", "PB", "EB"];
  if (b < base) return b + " B";
  let i = -1;
  do {
    b /= base;
    i++;
  } while (b >= base && i < units.length - 1);
  return b.toFixed(1) + " " + units[i];
};
const dom = (tag, cls, text) => {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined) n.textContent = text;
  return n;
};
function index(n, parent, base) {
  if (parent) parents.set(n, parent);
  colors.set(n, base || palette[0]);
  (n.children || []).forEach((c, i) =>
    index(c, n, parent ? base : palette[i % palette.length]),
  );
}
function tooltip(e, n) {
  const t = $("tooltip");
  t.replaceChildren(
    document.createTextNode(n.category_name ? categoryLabel(n.category_name) : n.path),
    dom("strong", "", fmt(n.bytes)),
    dom(
      "span",
      "",
      snapshot?.volume?.capacity
        ? ((n.bytes / snapshot.volume.capacity) * 100).toFixed(1) + "% of disk"
        : "Disk capacity unavailable",
    ),
  );
  t.hidden = false;
  t.onpointerleave = () => { t.hidden = true; };
  t.style.left = Math.max(8, Math.min(e.clientX + 15, innerWidth - t.getBoundingClientRect().width - 8)) + "px";
  t.style.top = Math.max(8, Math.min(e.clientY + 15, innerHeight - t.getBoundingClientRect().height - 8)) + "px";
}
function chartTooltip(node, label, bytes) {
  const capacity = snapshot?.volume?.capacity;
  node.title = `${label}: ${fmt(bytes)}, ${capacity ? ((bytes / capacity) * 100).toFixed(1) : "—"}% of disk`;
  node.onpointermove = (e) => tooltip(e, { path: label, bytes });
  node.onpointerleave = () => ($("tooltip").hidden = true);
}
let navigationVersion = 0;
async function loadChildren(n, offset = 0) {
  if (n.synthetic && !n.category_name) return;
  const url = n.category_name
    ? `/api/category?name=${encodeURIComponent(n.category_name)}&offset=${offset}&sort=${sortKey}&asc=${Number(sortAsc)}`
    : `/api/node?id=${n.id}&depth=3&offset=${offset}&sort=${sortKey}&asc=${Number(sortAsc)}`;
  const r = await fetch(url);
  if (!r.ok) throw Error("Unable to load folder: " + r.status);
  const data = await r.json();
  if (offset) {
    n.children = [...(n.children || []), ...(data.children || [])];
    n.child_count = data.child_count;
  } else {
    // Category API nodes carry filesystem-shaped data, but remain category views.
    const category = n.category_name;
    Object.assign(n, data);
    if (category) { n.category_name = category; n.synthetic = true; }
  }
  index(n, parents.get(n), colors.get(n));
  n.loaded = true;
}
async function navigate(n) {
  if (n.more_parent) {
    try {
      const p = n.more_parent;
      await loadChildren(p, (p.children || []).length);
      current = p;
      shown += 200;
      render();
    } catch (e) {
      $("scan-info").textContent = e.message;
    }
    return;
  }
  if (n.kind !== "directory" && !n.synthetic) return;
  const version = ++navigationVersion;
  try {
    if (!n.synthetic || n.category_name) await loadChildren(n);
    if (version !== navigationVersion) return;
    current = n;
    resetExplorerScroll();
    shown = 200;
    $("tooltip").hidden = true;
    render();
  } catch (e) {
    $("scan-info").textContent = e.message;
  }
}

function ordered(n) {
  return [...(n.children || [])].sort(
    (a, b) => b.bytes - a.bytes || a.name.localeCompare(b.name),
  );
}
function arc(r1, r2, a, b) {
  const point = (r, t) => [300 + r * Math.sin(t), 300 - r * Math.cos(t)];
  const p = point(r2, a),
    q = point(r2, b),
    s = point(r1, b),
    t = point(r1, a),
    big = b - a > Math.PI ? 1 : 0;
  return `M${p} A${r2},${r2} 0 ${big} 1 ${q} L${s} A${r1},${r1} 0 ${big} 0 ${t} Z`;
}
function canvasChart(w, h) {
  const canvas = dom("canvas");
  const scale = devicePixelRatio || 1;
  const panel = document.querySelector(".map-panel");
  const cssSize = Math.max(120, Math.min((panel?.clientWidth || 384) - 24, (panel?.clientHeight || 500) - 120));
  const drawScale = cssSize / w;
  canvas.width = cssSize * scale;
  canvas.height = cssSize * h / w * scale;
  canvas.style.width = cssSize + "px";
  canvas.style.height = cssSize * h / w + "px";
  canvas.style.maxWidth = Math.min(w, (540 * w) / h) + "px";
  canvas.style.aspectRatio = `${w} / ${h}`;
  canvas.setAttribute(
    "aria-label",
    "Space map. Use the folder table to navigate with a keyboard.",
  );
  const ctx = canvas.getContext("2d");
  ctx.scale(scale * drawScale, scale * drawScale);
  const regions = [];
  const hit = (e) => {
    const r = canvas.getBoundingClientRect();
    const x = ((e.clientX - r.left) * w) / r.width,
      y = ((e.clientY - r.top) * h) / r.height;
    return [...regions]
      .reverse()
      .find((z) => ctx.isPointInPath(z.path, x * scale * drawScale, y * scale * drawScale));
  };
  canvas.onpointermove = (e) => {
    const z = hit(e);
    canvas.style.cursor = z ? "pointer" : "default";
    if (z) tooltip(e, z.node);
    else $("tooltip").hidden = true;
  };
  canvas.onpointerleave = e => { if (!$("tooltip").contains(e.relatedTarget)) $("tooltip").hidden = true; };
  canvas.onclick = (e) => {
    const z = hit(e);
    if (z) navigate(z.node);
  };
  return { canvas, ctx, regions, textScale: 1 / drawScale };
}
function sunburst() {
  const { canvas, ctx, regions, textScale } = canvasChart(600, 600);
  let segments = 0;
  const visibleDepth = (node, depth = 0) => depth >= 5 ? depth :
    Math.max(depth, ...(node.children || []).filter(child => child.bytes > 0).map(child => visibleDepth(child, depth + 1)));
  const band = (294 - 125) / Math.max(1, visibleDepth(current));
  function ring(n, start, end, depth, base) {
    if (depth > 5 || n.bytes <= 0) return;
    let angle = start;
    for (const [i, c] of ordered(n).entries()) {
      const next = angle + ((end - start) * c.bytes) / n.bytes;
      const color = base || palette[i % palette.length];
      if (next - angle > 0.004 && segments < 1200) {
        const a = angle + 0.0018,
          b = Math.min(next - 0.0018, angle + 2 * Math.PI - 0.0001);
        if (b > a) {
          const path = new Path2D(
            arc(125 + (depth - 1) * band, 125 + depth * band - 2, a, b),
          );
          ctx.fillStyle = color;
          ctx.globalAlpha = 1 - (depth - 1) * 0.105;
          ctx.fill(path);
          regions.push({ path, node: c });
          segments++;
          ring(c, angle, next, depth + 1, color);
        }
      }
      angle = next;
    }
  }
  ring(current, 0, Math.PI * 2, 1, null);
  ctx.globalAlpha = 1;
  const center = new Path2D();
  center.arc(300, 300, 120, 0, Math.PI * 2);
  const theme = getComputedStyle(document.documentElement);
  ctx.fillStyle = theme.getPropertyValue("--card").trim() || "#fff";
  ctx.fill(center);
  ctx.strokeStyle = theme.getPropertyValue("--hair").trim();
  ctx.stroke(center);
  regions.push({ path: center, node: parents.get(current) || current });
  ctx.textAlign = "center";
  ctx.fillStyle = theme.getPropertyValue("--label").trim() || "#1d1d1f";
  ctx.font = `500 ${Math.max(25, 15 * textScale)}px system-ui`;
  ctx.fillText(fmt(current.bytes), 300, 300 - 10 * textScale);
  const label = displayName(current) || "/";
  ctx.fillStyle = theme.getPropertyValue("--label").trim();
  ctx.font = `${13 * textScale}px system-ui`;
  const maxChars = Math.max(8, Math.floor(228 / (7 * textScale)));
  ctx.fillText(label.length > maxChars ? label.slice(0, maxChars - 1) + "…" : label, 300, 300 + 13 * textScale);
  ctx.font = `${12 * textScale}px system-ui`;
  ctx.fillText(parents.has(current) ? "↑ Parent" : "Allocated", 300, 300 + 33 * textScale);
  return canvas;
}
// Squarified treemap: pack short rows to keep aspect ratios close to one.
function squarify(items, x, y, w, h) {
  const out = [];
  let rest = items.slice();
  const total = rest.reduce((s, n) => s + n.bytes, 0);
  if (!total) return out;
  rest = rest.map((n) => ({ n, a: (n.bytes / total) * w * h }));
  const worst = (row, side) => {
    if (!row.length || side <= 0) return Infinity;
    let sum = row.reduce((s, z) => s + z.a, 0),
      mx = Math.max(...row.map((z) => z.a)),
      mn = Math.min(...row.map((z) => z.a));
    return Math.max(
      (side * side * mx) / (sum * sum),
      (sum * sum) / (side * side * mn),
    );
  };
  function place(row) {
    const sum = row.reduce((s, z) => s + z.a, 0);
    if (w >= h) {
      const rw = sum / h;
      let yy = y;
      for (const z of row) {
        const rh = z.a / rw;
        out.push({ n: z.n, x, y: yy, w: rw, h: rh });
        yy += rh;
      }
      x += rw;
      w -= rw;
    } else {
      const rh = sum / w;
      let xx = x;
      for (const z of row) {
        const rw = z.a / rh;
        out.push({ n: z.n, x: xx, y, w: rw, h: rh });
        xx += rw;
      }
      y += rh;
      h -= rh;
    }
  }
  let row = [];
  while (rest.length) {
    const item = rest[0],
      side = Math.min(w, h);
    if (!row.length || worst([...row, item], side) <= worst(row, side)) {
      row.push(rest.shift());
    } else {
      place(row);
      row = [];
    }
  }
  if (row.length) place(row);
  return out;
}
function treemap() {
  const { canvas, ctx, regions, textScale } = canvasChart(600, 600);
  const items = ordered(current)
    .filter((n) => n.bytes > 0)
    .slice(0, 199);
  const missing =
    current.bytes -
    (current.own_bytes || 0) -
    items.reduce((sum, n) => sum + n.bytes, 0);
  if (missing > 0)
    items.push({
      name: "Other entries",
      path: current.path + " / Other entries",
      bytes: missing,
      kind: "directory",
      synthetic: true,
      more_parent: current,
    });
  for (const [i, r] of squarify(items, 3, 3, 594, 594).entries()) {
    if (r.w < 1 || r.h < 1) continue;
    const path = new Path2D();
    path.roundRect(
      r.x + 2,
      r.y + 2,
      Math.max(0, r.w - 4),
      Math.max(0, r.h - 4),
      5,
    );
    ctx.fillStyle = palette[i % palette.length];
    ctx.globalAlpha = 1;
    ctx.fill(path);
    ctx.globalAlpha = 1;
    regions.push({ path, node: r.n });
    if (r.w > 80 * textScale && r.h > 48 * textScale) {
      const max = Math.floor(r.w / (7 * textScale)) - 2;
      const name =
        displayName(r.n).length > max
          ? displayName(r.n).slice(0, Math.max(1, max - 1)) + "…"
          : displayName(r.n);
      const theme = getComputedStyle(document.documentElement);
      ctx.fillStyle = theme.getPropertyValue("--card").trim();
      ctx.fillRect(r.x + 6 * textScale, r.y + 6 * textScale, r.w - 12 * textScale, 42 * textScale);
      ctx.fillStyle = theme.getPropertyValue("--label").trim();
      ctx.font = `600 ${13 * textScale}px system-ui`;
      ctx.fillText(name, r.x + 12 * textScale, r.y + 23 * textScale);
      ctx.fillStyle = theme.getPropertyValue("--label").trim();
      ctx.font = `${12 * textScale}px system-ui`;
      ctx.fillText(fmt(r.n.bytes), r.x + 12 * textScale, r.y + 42 * textScale);
    }
  }
  return canvas;
}

function render() {
  if (mode === "cleanup-tab" || mode === "overview") return;
  ordered(current).forEach((n, i) =>
    colors.set(n, palette[i % palette.length]),
  );
  const chain = [];
  let n = current;
  while (n) {
    chain.unshift(n);
    n = parents.get(n);
  }
  $("breadcrumbs").replaceChildren();
  chain.forEach((n, i) => {
    if (i) $("breadcrumbs").append(dom("span", "", "/"));
    const b = dom(
      "button",
      "",
      n === root ? (mode === "categories" ? "Categories" : n.path) : displayName(n),
    );
    if (i === chain.length - 1) b.setAttribute("aria-current", "location");
    b.addEventListener("click", e => { e.stopPropagation(); navigate(n); });
    $("breadcrumbs").append(b);
  });
  $("folder-name").textContent = displayName(current) || "/";
  $("folder-size").textContent = fmt(current.bytes);
  $("folder-actions").replaceChildren();
  if (window.go?.main?.App && realPath(current))
    $("folder-actions").append(itemActions(current.path));
  $("map-label").textContent =
    mode === "categories" ? "Category map" : "Space map";
  $("chart").replaceChildren(view === "sunburst" ? sunburst() : treemap());
  $("legend").replaceChildren();
  ordered(current)
    .slice(0, 8)
    .forEach((c, i) => {
      const s = dom("span"),
        dot = dom("i", "dot");
      dot.style.background = palette[i % palette.length];
      const label = dom("b", "legend-label", displayName(c));
      label.title = displayName(c);
      s.append(dot, label);
      $("legend").append(s);
    });
  renderTable();
}
function renderTable() {
  $("explorer").classList.toggle("category-summary", mode === "categories" && current === root);
  updateSortHeaders("[data-sort]", sortKey, sortAsc);
  const children = [...(current.children || [])];
  children.sort((a, b) => {
    let val;
    if (sortKey === "name") val = a.name.localeCompare(b.name);
    else if (sortKey === "modified")
      val = new Date(a.modified) - new Date(b.modified);
    else val = (a[sortKey] || 0) - (b[sortKey] || 0);
    return (sortAsc ? 1 : -1) * val;
  });
  const body = $("children");
  body.replaceChildren();
  for (const n of children.slice(0, shown)) {
    const tr = dom("tr"),
      name = dom("td"),
      b = dom(n.kind === "directory" || n.synthetic ? "button" : "span", "file-label"),
      dot = dom("i", "dot");
    dot.style.background =
      colors.get(n) || palette[children.indexOf(n) % palette.length];
    b.append(
      icon(
        "fill-" +
          (n.kind === "directory" || n.synthetic
            ? itemIcon(n.name)
            : "file-text"),
      ),
      dom("span", "filename", displayName(n) + (n.kind === "symlink" ? " ↗" : "")),
    );
    b.title = n.category_name ? categoryLabel(n.category_name) : n.path;
    if (b.tagName === "BUTTON") {
      b.addEventListener("click", e => { e.stopPropagation(); navigate(n); });
      tr.classList.add("clickable-row");
      tr.onclick = () => navigate(n);
    }
    const entry = dom("div", "file-entry");
    entry.append(selectionBox(n), b);
    name.append(entry);
    const bar = dom("div", "size-bar"),
      fill = dom("i");
    fill.style.width =
      Math.min(100, (n.bytes / Math.max(1, current.bytes)) * 100) + "%";
    fill.style.background = dot.style.background;
    bar.append(fill);
    name.append(bar);
    if (window.go?.main?.App && realPath(n)) entry.append(itemActions(n.path));
    tr.append(
      name,
      dom(
        "td",
        "",
        ((n.bytes / Math.max(1, current.bytes)) * 100).toFixed(1) + "%",
      ),
      dom("td", "", fmt(n.bytes)),
      dom("td", "", n.files == null || n.synthetic ? "—" : Number(n.files).toLocaleString()),
      dom(
        "td",
        "",
        n.modified
          ? new Date(n.modified).toLocaleDateString(undefined, {
              month: "short",
              day: "numeric",
              year: "2-digit",
            })
          : "—",
      ),
    );
    body.append(tr);
  }
  if (children.length > shown || (current.child_count || 0) > children.length) {
    const tr = dom("tr"),
      td = dom("td"),
      b = dom(
        "button",
        "",
        `Show more (${Math.max(children.length, current.child_count || 0) - shown} remaining)`,
      );
    td.colSpan = 5;
    b.addEventListener("click", async () => {
      b.disabled = true;
      try {
        if (shown >= children.length)
          await loadChildren(current, children.length);
        shown += 200;
        render();
      } catch (e) {
        b.textContent = e.message;
        b.disabled = false;
      }
    });
    td.append(b);
    tr.append(td);
    body.append(tr);
  }
  if (!children.length) {
    const tr = dom("tr"),
      td = dom(
        "td",
        "empty",
        "No child entries. Folder metadata may still use allocated blocks.",
      );
    td.colSpan = 5;
    tr.append(td);
    body.append(tr);
  }
}
function categoryTree() {
  const r = {
    name: "All categories",
    path: "Categories",
    bytes: snapshot.scan.root.bytes,
    kind: "directory",
    synthetic: true,
    children: [],
  };
  for (const [name, bytes] of Object.entries(snapshot.categories))
    r.children.push({
      name,
      path: name,
      bytes,
      kind: "directory",
      synthetic: true,
      category_name: name,
      children: [],
    });
  index(r);
  return r;
}

const reducedMotion = () => !!window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
let lastScanSubtitle = "", overviewScroll = [], restoreOverviewAfterFilter = false;
function captureScroll() {
  return [document.querySelector("main"), $("content-grid"), $("overview-folders"), $("suggestion-list")].map(node => [node, node.scrollTop]);
}
function pageFade() {
  if (!reducedMotion()) $("content-grid").animate?.([{opacity: 0, transform: "translateY(4px)"}, {opacity: 1, transform: "none"}], {duration: 180, easing: "ease-out"});
}
function returnOverview() {
  const filtersChanged = !!($("suggest-category").value || $("suggest-safety").value);
  showMode("overview", false, true);
  restoreOverviewAfterFilter = filtersChanged;
  $("overview-back").hidden = true;
  for (const [node, top] of overviewScroll) node.scrollTop = top;
}
$("overview-back").onclick = () => history.state?.omdOverview ? history.back() : returnOverview();
window.addEventListener("popstate", e => {
  if (e.state?.omdOverview) { showMode(e.state.page, true, true); $("overview-back").hidden = false; }
  else if (!$("overview-back").hidden) returnOverview();
});
document.addEventListener("keydown", e => {
  if (e.key !== "Escape" || e.target.closest?.("dialog, #terminal-screen")) return;
  if ($("details-toggle").getAttribute("aria-expanded") === "true") {
    toggleNotes(false); $("details-toggle").focus(); return;
  }
  if (document.querySelector("details[open]")) return;
  if (!$("overview-back").hidden) { e.preventDefault(); $("overview-back").click(); }
});
function resetExplorerScroll() { document.querySelector(".table-scroll").scrollTop = 0; }
function showMode(next, fromOverview = false, historyNavigation = false) {
  if (next !== mode) {
    if (next !== "overview") restoreOverviewAfterFilter = false;
    if (fromOverview && mode === "overview" && !historyNavigation) {
      overviewScroll = captureScroll();
      history.pushState({ omdOverview: true, page: next }, "");
    } else if (!historyNavigation && history.state?.omdOverview) history.replaceState(null, "");
    $("overview-back").hidden = !fromOverview;
    pageFade();
    document.querySelector("main").scrollTop = 0;
    $("content-grid").scrollTop = 0;
  }
  if (next !== mode) resetExplorerScroll();
  mode = next;
  if (!document.body.classList.contains("is-scanning")) $("hub-scan").replaceChildren(
    icon("regular-arrow-clockwise"), document.createTextNode(snapshot ? "Rescan" : "Scan"), dom("span", "kbd", "⌘R"));
  $("scan-options").hidden = !snapshot;
  // Before a scan the Agent page is the shared launch hero, like every other page;
  // the session view (header, selectors, terminal) appears once there is a scan.
  const agentPage = mode === "agent" && (!!snapshot || !!window.agentSessionActive?.());
  document.body.classList.toggle("agent-page-active", agentPage);
  $("agent-page").hidden = !agentPage;
  $("scan-agent-link").hidden = !snapshot || mode === "agent";
  $("rescan-picker").open = false;
  ["overview", "folders", "categories", "cleanup-tab", "agent"].forEach((id) => {
    $(id).classList.toggle("active", id === mode);
    if (id === mode) $(id).setAttribute("aria-current", "page");
    else $(id).removeAttribute("aria-current");
  });
  renderHeading();
  if (agentPage) {
    $("welcome").hidden = true;
    $("content-grid").hidden = true;
    window.agentViewChanged?.();
    return;
  }

  if (!snapshot) {
    $("content-grid").hidden = true;
    $("welcome").hidden = false;
    const copy = {
      overview: [
        "Your disk, a little clearer.",
        "Explore your disk, on your own or with an agent.",
      ],
      folders: [
        "A place for every byte.",
        "See which folders take the most space, then explore inside them.",
      ],
      categories: [
        "See the bigger picture.",
        "Group your files by kind to understand where your space goes.",
      ],
      "cleanup-tab": [
        "Make room for what’s next.",
        "Find caches and other cleanup candidates, with guidance for every choice.",
      ],
      agent: [
        "Ask an agent about your disk.",
        "Scan first to give your agent a clear picture of your disk.",
      ],
    }[mode];
    const target = window.scanTargetLabel?.() || "Home";
    $("disk-caption").textContent = target + " is ready to scan";
    $("folder-choice").textContent = target;
    $("folder-picker").querySelector("summary").setAttribute("aria-label", "Choose scan folder, current: " + target);
    setLaunchTitle(copy[0]);
    $("launch-description").textContent = copy[1];
    $("launch-cards").hidden = false;
    window.agentViewChanged?.();
    document.querySelector(".view-switch").hidden = true;
    return;
  }
  $("welcome").hidden = true;
  $("content-grid").hidden = false;
  window.agentViewChanged?.();
  $("overview-panel").hidden = mode !== "overview";
  $("explorer").hidden = mode === "cleanup-tab" || mode === "overview";
  $("suggestions").hidden = !["cleanup-tab", "overview"].includes(mode);
  $("content-grid").classList.toggle("full-view", mode !== "overview");
  $("cleanup-filters").hidden = mode !== "cleanup-tab";
  $("review-cleanup").hidden = mode !== "overview";
  renderSuggestions();
  document.querySelector(".view-switch").hidden =
    mode === "cleanup-tab" || mode === "overview";
  if (
    mode === "overview" &&
    ($("suggest-category").value || $("suggest-safety").value)
  ) {
    $("suggest-category").value = "";
    $("suggest-safety").value = "";
    refreshSuggestions();
  }
  if (mode === "cleanup-tab" || mode === "overview") return;
  root = mode === "categories" ? categoryTree() : snapshot.scan.root;
  current = root;
  resetExplorerScroll();
  shown = 200;
  render();
  if ((root.child_count || 0) > (root.children || []).length && (sortKey !== "bytes" || sortAsc) && !root.synthetic) navigate(root);
}
function icon(name) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "i");
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS(svg.namespaceURI, "use");
  use.setAttribute("href", "#" + name);
  svg.append(use);
  return svg;
}
function itemIcon(name) {
  const n = name.toLowerCase();
  return /library/.test(n)
    ? "books"
    : /cache|gradle|poetry/.test(n)
      ? "stack"
      : /docker|container/.test(n)
        ? "archive"
        : /simulator|android/.test(n)
          ? "device-mobile"
          : /build|derived/.test(n)
            ? "hammer"
            : /node_modules/.test(n)
              ? "cube"
              : /download/.test(n)
                ? "download-simple"
                : /desktop/.test(n)
                  ? "desktop"
                  : /document/.test(n)
                    ? "file-text"
                    : /work|project/.test(n)
                      ? "briefcase"
                      : "folder";
}
let suggestionsShown = 100;
function renderSuggestions() {
  const list = $("suggestion-list");
  list.replaceChildren();
  const overview = mode === "overview";
  const items = suggestionItems.slice(0, overview ? 7 : suggestionsShown);
  for (const s of items) {
    const card = dom("div", "it suggestion-card");
    const tile = dom("span", "tile");
    tile.append(icon("fill-" + itemIcon(s.path + s.category)));
    const title = dom("div", "item-title");
    const name = dom(
      "button",
      "n item-name",
      s.path.split("/").filter(Boolean).pop() || s.path,
    );
    name.title = s.path + "\n" + s.explanation;
    const details = dom("div", "suggestion-details");
    details.hidden = true;
    const id = "suggest-detail-" + list.childElementCount;
    details.id = id;
    name.setAttribute("aria-expanded", "false");
    name.setAttribute("aria-controls", id);
    card.onclick = () => {
      const opening = name.getAttribute("aria-expanded") !== "true";
      name.setAttribute("aria-expanded", String(opening));
      details.getAnimations?.().forEach(animation => animation.cancel());
      if (opening) details.hidden = false;
      const height = details.getBoundingClientRect().height;
      if (reducedMotion() || !details.animate) details.hidden = !opening;
      else {
        const animation = details.animate(opening ? [{height:"0px",paddingTop:0,paddingBottom:0,opacity:0}, {height:height + "px",opacity:1}] : [{height:height + "px",opacity:1}, {height:"0px",paddingTop:0,paddingBottom:0,opacity:0}], {duration:180,easing:"ease-out"});
        animation.onfinish = () => { details.hidden = !opening; };
      }
    };
    const pathLine = dom("div", "detail-path");
    const pathText = dom("span", "ellipsized-path");
    const slash = s.path.lastIndexOf("/");
    pathText.append(dom("span", "path-prefix", s.path.slice(0, slash + 1)), dom("span", "path-tail", s.path.slice(slash + 1)));
    pathText.title = s.path;
    pathLine.append(pathText);
    if (window.go?.main?.App) {
      const reveal = itemActions(s.path);
      reveal.lastElementChild?.remove();
      pathLine.append(reveal);
    }
    const facts = dom("dl", "detail-facts");
    facts.append(dom("dt", "", "Why"), dom("dd", "", s.explanation || s.rule),
      dom("dt", "", "Score"), dom("dd", "num", `${Math.round(s.score)} · ${s.personal ? "your files" : s.safety === "cache" ? "safe cache" : s.safety === "reinstallable" ? "reinstallable data" : "review before cleaning"} · unused ${s.stale_days} days`));
    if (s.incomplete) facts.append(dom("dt", "", "Coverage"), dom("dd", "", "Partial scan · review before cleaning"));
    if (s.project_path) facts.append(dom("dt", "", "Project"), dom("dd", "", s.project_path));
    details.append(pathLine, facts);
    if (s.suggested_command) {
      const terminal = dom("div", "detail-command");
      const copy = dom("button", "icon-action");
      copy.title = "Copy suggested command";
      copy.setAttribute("aria-label", copy.title);
      copy.append(icon("regular-copy"));
      copy.onclick = async () => {
        try {
          if (window.go?.main?.App) await window.go.main.App.Copy(s.suggested_command);
          else await navigator.clipboard.writeText(s.suggested_command);
          copy.replaceChildren(icon("regular-check"));
          copy.setAttribute("aria-label", "Copied"); copy.title = "Copied";
          setTimeout(() => { copy.replaceChildren(icon("regular-copy")); copy.title = "Copy suggested command"; copy.setAttribute("aria-label", copy.title); }, 1600);
        } catch (e) { showFeedback(String(e)); }
      };
      terminal.append(dom("span", "command-prompt", "$"), dom("code", "", s.suggested_command), copy);
      details.append(terminal);
    }
    title.append(
      name,
      dom("div", "d", `${categoryLabel(s.category)}, ${s.stale_days} days unused`),
    );
    const safe =
      !s.personal &&
      !s.incomplete &&
      ["cache", "reinstallable"].includes(s.safety);
    const badge = dom(
      "span",
      "badge " + (s.personal ? "rev personal" : safe ? "safe" : "rev"),
    );
    badge.append(
      icon(safe ? "fill-shield-check" : "fill-eye"),
      document.createTextNode(
        s.personal ? "Your files" : safe ? "Safe" : "Review",
      ),
    );
    badge.title = s.safety + (s.badge ? ": " + s.badge : "");
    const score = dom("span", "sc num suggestion-score", Math.round(s.score));
    score.setAttribute(
      "aria-label",
      `Cleanup score ${Math.round(s.score)} out of 100`,
    );
    card.append(
      selectionBox(s),
      tile,
      title,
      badge,
      score,
      dom("span", "s num suggestion-size", fmt(s.bytes)),
    );
    name.onclick = e => { e.stopPropagation(); card.click(); };
    list.append(card, details);
  }
  if (!items.length)
    list.append(
      dom(
        "p",
        "empty",
        "No cleanup suggestions match these filters. Try another filter or scan again. Personal files appear under Largest or Unused Longest.",
      ),
    );
  if (!overview && suggestionItems.length > suggestionsShown) {
    const more = dom("button", "btn sec", "Show More Suggestions");
    more.onclick = () => {
      suggestionsShown += 100;
      renderSuggestions();
    };
    list.append(more);
  }
  if (!overview && suggestionItems.length === 500)
    list.append(
      dom(
        "p",
        "hint",
        "Showing the first 500 matches. Filter by category or safety to narrow the list.",
      ),
    );
  updateSortHeaders("[data-cleanup-sort]", cleanupSort || suggestionSort, cleanupSort ? cleanupAsc : suggestionAsc);
  renderRemembered();
  updateSelection();
  if (overview && restoreOverviewAfterFilter) {
    for (const [node, top] of overviewScroll) node.scrollTop = top;
    restoreOverviewAfterFilter = false;
  }
}
$("sunburst").onclick = () => {
  view = "sunburst";
  $("sunburst").classList.add("active");
  $("treemap").classList.remove("active");
  $("sunburst").setAttribute("aria-pressed", "true");
  $("treemap").setAttribute("aria-pressed", "false");
  render();
};
$("treemap").onclick = () => {
  view = "treemap";
  $("treemap").classList.add("active");
  $("sunburst").classList.remove("active");
  $("sunburst").setAttribute("aria-pressed", "false");
  $("treemap").setAttribute("aria-pressed", "true");
  render();
};
["overview", "folders", "categories", "cleanup-tab", "agent"].forEach(
  (m) => ($(m).onclick = () => showMode(m)),
);
function realPath(n) { return !!n.path?.startsWith("/") && !n.synthetic && !n.other && !n.more_parent; }
function middlePath(path) { return path.length > 86 ? path.slice(0, 36) + "…" + path.slice(-46) : path; }
// Shared sortable header behavior for both explorer and cleanup tables.
function updateSortHeaders(selector, key, asc) {
  for (const button of document.querySelectorAll(selector)) {
    if (!button.dataset.label) button.dataset.label = button.textContent.replace(/ [↕▴▾]$/, "");
    const active = (button.dataset.sort || button.dataset.cleanupSort) === key;
    button.replaceChildren(document.createTextNode(button.dataset.label));
    if (active) button.append(dom("span", "sort-chevron", asc ? "▴" : "▾"));
    button.closest("th").setAttribute("aria-sort", active ? (asc ? "ascending" : "descending") : "none");
  }
}
for (const button of document.querySelectorAll("[data-sort]")) button.onclick = async () => {
  sortAsc = sortKey === button.dataset.sort ? !sortAsc : button.dataset.sort === "name";
  sortKey = button.dataset.sort;
  try {
    // Fully loaded and synthetic views can be sorted without a request. Only
    // paginated filesystem/category nodes need the server's global ordering.
    if ((current.child_count || 0) > (current.children || []).length && (!current.synthetic || current.category_name)) await loadChildren(current);
    shown = 200; resetExplorerScroll(); render();
  } catch (e) { showFeedback(e.message); }
};
let cleanupSort = "", cleanupAsc = false;
for (const button of document.querySelectorAll("[data-cleanup-sort]")) button.onclick = () => {
  cleanupAsc = cleanupSort === button.dataset.cleanupSort ? !cleanupAsc : button.dataset.cleanupSort === "name";
  cleanupSort = button.dataset.cleanupSort;
  sortCleanupItems(); renderSuggestions();
};
function sortCleanupItems() {
  if (!cleanupSort) return;
  const safety = {cache:3, reinstallable:2, "user-data":1, unknown:0};
  const value = s => cleanupSort === "name" ? s.path.split("/").pop().toLocaleLowerCase() : cleanupSort === "safety" ? (s.personal || s.incomplete ? 0 : safety[s.safety] || 0) : cleanupSort === "size" ? s.bytes : s.score;
  suggestionItems.sort((a,b) => {
    const x=value(a), y=value(b);
    const cmp = typeof x === "string" ? x.localeCompare(y) : x-y;
    return (cleanupAsc ? 1 : -1)*cmp || a.path.localeCompare(b.path);
  });
}
// One label for the disclosure trigger and its accessible close action.
$("counting-slot").append($("counting-disclosure"));
const COUNTING_LABEL = "How we counted";
$("details-toggle").textContent = COUNTING_LABEL;
$("notes-close").setAttribute("aria-label", "Close " + COUNTING_LABEL);
$("details-toggle").setAttribute("aria-expanded", "false");
$("details-toggle").setAttribute("aria-controls", "notes");
let notesTimer;
function toggleNotes(open) {
  clearTimeout(notesTimer);
  const panel = $("counting-disclosure");
  $("details-toggle").setAttribute("aria-expanded", String(open));
  $("notes-close").hidden = !open;
  $("notes").inert = !open;
  if (open) {
    $("notes").hidden = false;
    applyNotesHeight();
    $("notes-body").scrollTop = 0;
    // Establish the collapsed geometry before expanding the grid track.
    panel.getBoundingClientRect();
    panel.classList.add("open");
  } else {
    panel.classList.remove("open");
    $("details-toggle").focus({preventScroll:true});
    notesTimer = setTimeout(() => { $("notes").hidden = true; }, reducedMotion() ? 0 : 180);
  }
}
// The counting panel opens upward; its top edge drags to resize, remembered across launches.
const notesHeightKey = "omd.countingPanelHeight";
function notesMaxHeight() {
  const bottom = $("counting-disclosure").querySelector(".notes-reveal").getBoundingClientRect().bottom || innerHeight;
  return Math.max(120, bottom - 64);
}
function setNotesHeight(h, save) {
  const clamped = Math.round(Math.min(notesMaxHeight(), Math.max(120, h)));
  $("notes").style.height = clamped + "px";
  $("notes").style.maxHeight = "none";
  if (save) try { localStorage.setItem(notesHeightKey, String(clamped)); } catch (_) {}
}
function applyNotesHeight() {
  let h = 0;
  try { h = Number(localStorage.getItem(notesHeightKey)) || 0; } catch (_) {}
  if (h) setNotesHeight(h, false);
  else { $("notes").style.height = ""; $("notes").style.maxHeight = ""; }
}
$("notes-resize").addEventListener("pointerdown", e => {
  if (e.button !== 0) return;
  e.preventDefault();
  const handle = e.currentTarget, startY = e.clientY, startH = $("notes").getBoundingClientRect().height;
  handle.setPointerCapture(e.pointerId);
  document.body.classList.add("notes-resizing");
  const move = ev => setNotesHeight(startH + (startY - ev.clientY), false);
  const up = ev => {
    handle.removeEventListener("pointermove", move);
    handle.removeEventListener("pointerup", up);
    handle.removeEventListener("pointercancel", up);
    document.body.classList.remove("notes-resizing");
    setNotesHeight(startH + (startY - ev.clientY), true);
  };
  handle.addEventListener("pointermove", move);
  handle.addEventListener("pointerup", up);
  handle.addEventListener("pointercancel", up);
});
$("notes-resize").addEventListener("click", e => e.stopPropagation());
addEventListener("resize", () => { if (!$("notes").hidden && $("notes").style.height) setNotesHeight(parseFloat($("notes").style.height), false); });
document.addEventListener("click", e => { if ($("counting-disclosure").classList.contains("open") && !$("counting-disclosure").contains(e.target)) toggleNotes(false); });
$("details-toggle").onclick = () => toggleNotes($("details-toggle").getAttribute("aria-expanded") !== "true");
$("notes-close").onclick = () => { toggleNotes(false); $("details-toggle").focus(); };
function noteRow(label, sentence, value) {
  const row = dom("div", "note-row");
  row.append(dom("span", "note-term", label), dom("span", "note-copy", sentence));
  if (value !== undefined) row.append(dom("span", "num note-value", value));
  return row;
}
function renderCounting() {
  const s = snapshot, v = s.volume;
  $("volume-values").replaceChildren();
  if (v) for (const [label, sentence, bytes] of [
    ["Capacity", "Total space on this filesystem.", v.capacity],
    ["Used", "Space currently allocated on the volume.", v.used],
    ["Available", "Space available to your files.", v.available],
    ["Outside scan", "System, other users, snapshots and inaccessible data.", v.outside],
  ]) $("volume-values").append(noteRow(label, sentence, fmt(bytes)));
  $("accounting").replaceChildren(
    noteRow("Files", "Allocated blocks; cloud placeholders use logical bytes.", fmt(s.scan.root.bytes)),
    noteRow("Read-only", "Nothing is removed without your Trash confirmation."));
  $("volume-note").replaceChildren(
    noteRow("Reclaim", "Shared blocks, clones and compression can change freed space."),
    noteRow("APFS", "Shared and purgeable space can differ from macOS totals."),
    noteRow("Last used", "Access times may be unavailable or affected by scanning."));
  if (v?.excess) $("volume-note").append(noteRow("Overlap", "Shared blocks or placeholders exceed volume usage.", fmt(v.excess)));
  const skipped = s.skipped_count || s.scan.skipped.length;
  $("skipped").replaceChildren(noteRow("Paths", skipped ? "Some folders could not be read; totals are partial." : "All accessible paths were counted.", skipped.toLocaleString()));
  for (const p of s.scan.skipped.slice(0, 500)) {
    const row = dom("div", "note-row skipped-row"), term = dom("span", "note-term ellipsized-path"), slash = p.path.lastIndexOf("/", p.path.length - 2);
    term.append(dom("span", "path-prefix", p.path.slice(0, slash + 1)), dom("span", "path-tail", p.path.slice(slash + 1)));
    row.append(term, dom("span", "note-copy", p.reason));
    row.title = p.path + " — " + p.reason;
    $("skipped").append(row);
  }
}
function loadSnapshot() {
  return fetch("/api/scan")
    .then((r) => {
      if (!r.ok) throw Error("Unable to load scan: " + r.status);
      return r.json();
    })
    .then(async (s) => {
      snapshot = s;
      $("open-last").hidden = true;
      $("skipped").replaceChildren();
      renderVolume();
      index(s.scan.root);
      $("total").textContent = fmt(s.scan.root.bytes);
      const finished = new Date(s.scan.finished),
        started = new Date(s.scan.started);
      const age = Math.max(0, Math.floor((Date.now() - finished) / 60000));
      const ago =
        age < 1
          ? "just now"
          : age < 60
            ? `${age} ${age === 1 ? "minute" : "minutes"} ago`
            : age < 1440
              ? `${Math.floor(age / 60)} hours ago`
              : finished.toLocaleDateString();
      const seconds = Math.max(1, Math.round((finished - started) / 1000));
      lastScanSubtitle = $("scan-info").textContent =
        `Last scan ${ago}, took ${seconds} ${seconds === 1 ? "second" : "seconds"}`;
      $("scan-info").title =
        s.scan.root.path + ", " + finished.toLocaleString();
      if (window.go?.main?.App || !$("web-root").value) $("web-root").value = s.scan.root.path;
      $("welcome").hidden = true;
      $("content-grid").hidden = false;
      renderCounting();
      const skippedPaths = s.skipped_count || s.scan.skipped.length;
      $("scan-status").textContent = skippedPaths ? `${skippedPaths.toLocaleString()} skipped ${skippedPaths === 1 ? "path" : "paths"}` : "";
      $("scan-status").hidden = !skippedPaths;
      selected.clear();
      updateSelection();
      for (const [id, values] of [
        ["suggest-category", Object.keys(s.categories)],
        ["suggest-safety", ["cache", "reinstallable", "user-data", "unknown"]],
      ]) {
        const select = $(id),
          value = select.value;
        select.replaceChildren(new Option("All", ""));
        for (const v of values) select.add(new Option(id === "suggest-category" ? categoryLabel(v) : v, v));
        select.value = value;
      }
      await refreshRemembered(true);
      await refreshSuggestions();
      renderOverview();
      showMode(mode);
    })
    .catch((e) => {
      $("content-grid").hidden = true;
      $("welcome").hidden = false;
      showFeedback(e.message);
    });
}

function renderVolume() {
  if (!snapshot?.volume) return;
  const v = snapshot.volume;
  $("volume").hidden = false;
  // The HUD always uses decimal GB so volume values stay directly comparable.
  for (const [id, bytes] of [
    ["hud-used", v.used],
    ["hud-free", v.available],
    ["hud-scanned", snapshot.scan.root.bytes],
    ["hud-outside", v.outside],
  ]) {
    $(id).textContent = (bytes / 1e9).toFixed(2) + " GB";
  }
}
if (!window.go?.main?.App) loadSnapshot();

let suggestionSort = "score",
  suggestionAsc = false,
  suggestionItems = [];
let suggestionVersion = 0;
async function refreshSuggestions() {
  const version = ++suggestionVersion;
  try {
    const r = await fetch(
      `/api/suggest?sort=${suggestionSort}&asc=${Number(suggestionAsc)}&category=${encodeURIComponent($("suggest-category").value)}&safety=${encodeURIComponent($("suggest-safety").value)}&limit=500`,
    );
    if (!r.ok) throw Error(await r.text());
    const items = await r.json();
    if (version !== suggestionVersion) return;
    suggestionItems = items || [];
    sortCleanupItems();
    suggestionsShown = 100;
    renderSuggestions();
  } catch (e) {
    $("scan-info").textContent = e.message;
  }
}
for (const b of document.querySelectorAll("[data-suggest-sort]")) {
  b.setAttribute(
    "aria-pressed",
    String(b.dataset.suggestSort === suggestionSort),
  );
  b.title = "Sort suggestions; click again to reverse the order";
  b.onclick = () => {
    suggestionAsc =
      b.dataset.suggestSort === suggestionSort ? !suggestionAsc : false;
    suggestionSort = b.dataset.suggestSort;
    cleanupSort = "";
    for (const c of document.querySelectorAll("[data-suggest-sort]")) {
      c.classList.toggle("active", c.dataset.suggestSort === suggestionSort);
      c.setAttribute(
        "aria-pressed",
        String(c.dataset.suggestSort === suggestionSort),
      );
    }
    $("sort-hint").textContent = (
      suggestionAsc
        ? {
            score: "Lower scores at the top",
            size: "Smallest items at the top",
            stale: "Most recently used at the top",
          }
        : {
            score: "Safest first; personal files in Largest",
            size: "Largest items at the top",
            stale: "Longest unused at the top",
          }
    )[suggestionSort];
    return refreshSuggestions();
  };
}
$("suggest-category").onchange = $("suggest-safety").onchange = () =>
  refreshSuggestions();
let rememberedItems = [], rememberedMatches = [], rememberedSelectionPending = false;
async function refreshRemembered(preselect = false) {
  try {
    const r = await fetch("/api/remembered");
    if (!r.ok) throw Error(await r.text());
    const data = await r.json();
    rememberedItems = data.items || []; rememberedMatches = data.matches || [];
    if (preselect) rememberedSelectionPending = true;
    renderRemembered();
  } catch (e) { showFeedback("Could not load remembered choices: " + e.message); }
}
async function forgetRemembered(path, all = false) {
  try {
    await action("/api/remembered", {paths:path ? [path] : [], forget_all:all});
    if (all) for (const item of rememberedMatches) selected.delete(item.path);
    else selected.delete(path);
    await refreshRemembered(); renderRememberedManager(); updateSelection();
  } catch (e) { showFeedback(String(e)); }
}
function renderRemembered() {
  if (mode === "cleanup-tab" && rememberedSelectionPending) {
    for (const item of rememberedMatches) selected.set(item.path, item.current_bytes);
    rememberedSelectionPending = false;
  }
  document.querySelectorAll(".growth-tooltip").forEach(tip => tip.remove());
  const card = $("remembered-card");
  card.hidden = mode !== "cleanup-tab" || !rememberedMatches.length;
  card.replaceChildren();
  $("remembered-overview").hidden = mode !== "overview" || !rememberedMatches.length;
  $("remembered-overview").textContent = `${rememberedMatches.length} items you cleaned before are back →`;
  $("manage-remembered-empty").hidden = !rememberedItems.length || rememberedMatches.length > 0 || mode !== "cleanup-tab";
  if (!rememberedMatches.length) return;
  const heading = dom("div", "remembered-heading");
  heading.append(dom("strong", "", `Back since last time · ${rememberedMatches.length} items · ${fmt(rememberedMatches.reduce((sum,item) => sum + item.current_bytes,0))}`));
  const manage = dom("button", "link", "Manage remembered"); manage.onclick = openRememberedManager;
  heading.append(manage); card.append(heading);
  for (const [i,item] of rememberedMatches.entries()) {
    const row = dom("div", "remembered-row");
    const label = dom("label", "remembered-choice");
    const text = dom("span", "remembered-name");
    text.append(dom("strong", "", item.path.split("/").pop()), dom("span", "remembered-path", middlePath(item.path)));
    text.title = item.path;
    label.append(selectionBox({...item,bytes:item.current_bytes,kind:item.current_kind}), text, dom("span", "num", fmt(item.current_bytes)), dom("span", "remembered-growth num", (item.growth >= 0 ? "+" : "−") + fmt(Math.abs(item.growth))));
    row.append(label);
    if (item.warning) {
      const warning = dom("span", "growth-warning"); warning.tabIndex = 0;
      warning.setAttribute("aria-label", "Much bigger than last time");
      warning.append(icon("fill-warning-circle"));
      const tip = dom("span", "growth-tooltip", `This folder is much bigger than when you cleaned it (was ${fmt(item.bytes)}, now ${fmt(item.current_bytes)}). Check it before cleaning.`);
      tip.id = "growth-warning-" + i; tip.setAttribute("role", "tooltip");
      warning.setAttribute("aria-describedby", tip.id);
      tip.hidden = true; document.body.append(tip);
      const positionTip = () => {
        tip.hidden = false;
        const r = warning.getBoundingClientRect();
        tip.style.left = Math.max(8, Math.min(r.right - 280, innerWidth - 288)) + "px";
        tip.style.top = Math.max(8, r.top - 100) + "px";
      };
      warning.onpointerenter = warning.onfocus = positionTip;
      warning.onpointerleave = () => { if (document.activeElement !== warning) tip.hidden = true; };
      warning.onblur = () => { tip.hidden = true; };
      row.append(warning);
    }
    const forget = dom("button", "link", "Forget"); forget.setAttribute("aria-label", "Forget " + item.path); forget.onclick = () => forgetRemembered(item.path); row.append(forget); card.append(row);
  }
  const clean = dom("button", "btn pri", "Clean again");
  clean.disabled = !rememberedMatches.some(item => selected.has(item.path));
  clean.onclick = () => {
    // The confirmation contains only the checked returning items.
    const checked = rememberedMatches.filter(item => selected.has(item.path));
    selected.clear(); checked.forEach(item => selected.set(item.path,item.current_bytes));
    updateSelection(); $("trash-selected").click();
  };
  card.append(clean);
}
function renderRememberedManager() {
  const list = $("remembered-paths"); list.replaceChildren();
  for (const item of rememberedItems) {
    const row = dom("div", "remembered-managed");
    const path = dom("span", "", item.path); path.title = item.path;
    const forget = dom("button", "link", "Forget"); forget.setAttribute("aria-label", "Forget " + item.path); forget.onclick = () => forgetRemembered(item.path);
    row.append(path,forget); list.append(row);
  }
  if (!rememberedItems.length) list.append(dom("p", "", "No remembered choices."));
  $("forget-all").disabled = !rememberedItems.length;
}
function openRememberedManager() { renderRememberedManager(); $("remembered-manager").showModal(); }
$("manage-remembered-empty").onclick = openRememberedManager;
$("remembered-close").onclick = () => $("remembered-manager").close();
$("forget-all").onclick = () => forgetRemembered("",true);
$("remembered-overview").onclick = () => showMode("cleanup-tab",true);
const selected = new Map();
function selectionBox(n) {
  const box = dom("input");
  box.type = "checkbox";
  box.dataset.selectionPath = n.path;
  box.setAttribute("aria-label", "Select " + n.path);
  box.checked = selected.has(n.path);
  box.disabled =
    !!n.synthetic ||
    n.kind === "symlink" ||
    n.path === snapshot?.scan.root.path;
  box.onclick = e => e.stopPropagation();
  box.onchange = () => {
    if (box.checked) {
      for (const p of selected.keys())
        if (n.path.startsWith(p + "/") || p.startsWith(n.path + "/")) {
          box.checked = false;
          $("trash-results").textContent =
            "Select separate items without parent/child overlaps.";
          return;
        }
      selected.set(n.path, n.bytes);
    } else selected.delete(n.path);
    updateSelection();
  };
  return box;
}
function updateSelection() {
  const cleanAgain = $("remembered-card").querySelector(".btn.pri");
  if (cleanAgain) cleanAgain.disabled = !rememberedMatches.some(item => selected.has(item.path));
  for (const box of document.querySelectorAll("[data-selection-path]"))
    box.checked = selected.has(box.dataset.selectionPath);
  $("selection-bar").hidden = selected.size === 0;
  // Keep the shared selection action available in folder and category views.
  const destination = ["folders", "categories"].includes(mode)
    ? $("explorer")
    : $("suggestions");
  destination.append($("selection-bar"), $("trash-results"));
  $("trash-selected").disabled = selected.size === 0;
  $("selection-total").textContent =
    `${selected.size} ${selected.size === 1 ? "item" : "items"} selected, ${fmt([...selected.values()].reduce((a, b) => a + b, 0))}`;
}
async function action(path, data) {
  const r = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-OMD-Action": "1" },
    body: JSON.stringify(data),
  });
  if (!r.ok) throw Error(await r.text());
  return r.json();
}
$("clear-selection").onclick = () => {
  selected.clear();
  updateSelection();
  renderSuggestions();
  render();
};
$("trash-selected").onclick = () => {
  $("confirm-total").textContent = $("selection-total").textContent;
  $("confirm-paths").textContent = [...selected.keys()].join("\n");
  $("remember-choice").checked = true;
  $("trash-confirm").showModal();
};
$("cancel-trash").onclick = () => $("trash-confirm").close();
$("confirm-trash").onclick = async () => {
  const b = $("confirm-trash");
  b.disabled = true;
  const sizes = new Map(selected);
  try {
    const result = await action("/api/trash", { paths: [...selected.keys()], remember: $("remember-choice").checked });
    $("trash-confirm").close();
    await loadSnapshot();
    if (result.memory_error) showFeedback("Items moved, but choices could not be remembered: " + result.memory_error);
    const message =
      (result.results || [])
        .map(
          (r) =>
            r.path +
            ": " +
            (r.trashed ? "Moved to Trash" : "Failed: " + r.error) +
            (r.trashed && r.error ? " · " + r.error : ""),
        )
        .join("\n") +
      (result.scan_error
        ? "\nTotals could not refresh: " + result.scan_error
        : "");
    const moved = (result.results || []).filter(r => r.trashed);
    const bytes = moved.reduce((sum, r) => sum + (sizes.get(r.path) || 0), 0);
    const details = dom("details", "trash-details");
    details.append(dom("summary", "", "Details"), dom("pre", "", message));
    $("trash-results").className = "banner " + (moved.length === (result.results || []).length && !result.scan_error ? "banner-success" : "banner-warning");
    $("trash-results").replaceChildren(
      dom("span", "", moved.length ? `${fmt(bytes)} moved to Trash. Review more items or scan again.` : "No items moved. Review details and try again."), details);
    $("selection-bar").hidden = true;
  } catch (e) {
    $("trash-results").replaceChildren();
    showFeedback(String(e));
    $("selection-bar").hidden = selected.size === 0;
    $("trash-confirm").close();
  } finally {
    b.disabled = false;
  }
};
if (!window.go?.main?.App) {
  let target = "home";
  try { target = localStorage.getItem("omd-scan-target") || "home"; } catch {}
  window.scanTargetLabel = () => target === "home" ? "Home" : target === "/" ? "Macintosh HD" : target.split("/").filter(Boolean).pop() || "Home";
  window.selectScanTarget = path => {
    target = path;
    $("web-root").value = path === "home" ? snapshot?.scan.root.path || "" : path;
    $("folder-choice").textContent = window.scanTargetLabel();
    try { localStorage.setItem("omd-scan-target", path); } catch {}
    if (!snapshot) showMode(mode);
  };
  $("web-root").addEventListener("change", () => window.selectScanTarget($("web-root").value));
  window.selectScanTarget(target);
  $("web-scan").hidden = false;
  $("web-start-scan").onclick = async () => {
    const b = $("web-start-scan");
    b.disabled = true;
    $("web-scan-status").textContent = "Scanning…";
    scanState(true);
    try {
      const result = await action("/api/rescan", { root: $("web-root").value });
      if (result.scan_error) throw Error(result.scan_error);
      await loadSnapshot();
      $("web-scan-status").textContent = "Complete";
      scanState(false);
    } catch (e) {
      $("web-scan-status").textContent = String(e);
      scanState(false);
      showFeedback(String(e));
    } finally {
      b.disabled = false;
    }
  };
}

const gb = (bytes) => (bytes / 1e9).toFixed(1) + " GB";
function renderOverview() {
  if (!snapshot) return;
  const scan = snapshot.scan.root,
    v = snapshot.volume;
  const suggestions = snapshot.suggestions || [];
  const reclaim = suggestions
    .filter(
      (s) =>
        !s.personal &&
        !s.incomplete &&
        ["cache", "reinstallable"].includes(s.safety),
    )
    .reduce((sum, s) => sum + s.bytes, 0);
  const lowerBound = suggestions.length === 100;
  const capacity = v?.capacity || 0,
    free = v?.available || 0;
  const clean = Math.min(reclaim, scan.bytes, v?.used ?? scan.bytes);
  const files = Math.max(
    0,
    Math.min(scan.bytes, v?.used ?? scan.bytes) - clean,
  );
  const outside = Math.max(0, (v?.used || scan.bytes) - files - clean);
  $("disk-capacity").textContent = v
    ? "Macintosh HD, " + gb(capacity)
    : "Disk capacity unavailable";
  $("disk-free").textContent = v ? gb(free) : "—";
  const percent = capacity ? Math.round((1 - free / capacity) * 100) : 0;
  $("disk-full").className = "pill" + (percent >= 90 ? " warn" : "");
  $("disk-full").replaceChildren(
    icon(percent >= 90 ? "fill-warning-circle" : "fill-hard-drive"),
    document.createTextNode(v ? percent + "% full" : "Unavailable"),
  );
  const segments = [
    [files, "files"],
    [clean, "clean"],
    [outside, "sys"],
  ];
  const C = 2 * Math.PI * 88;
  let offset = 0;
  const svg = $("ringsvg");
  if (
    svg.dataset.finished !== snapshot.scan.finished &&
    svg.animate &&
    !window.matchMedia?.("(prefers-reduced-motion: reduce)").matches
  )
    svg.animate([{ opacity: 0.25 }, { opacity: 1 }], { duration: 150 });
  svg.dataset.finished = snapshot.scan.finished;
  svg.replaceChildren();
  function circle(value, color, start, base = false) {
    const c = document.createElementNS("http://www.w3.org/2000/svg", "circle");
    for (const [k, val] of Object.entries({
      cx: 100,
      cy: 100,
      r: 88,
      fill: "none",
      stroke: `var(--c-${color})`,
      "stroke-width": 20,
    }))
      c.setAttribute(k, val);
    if (!base) {
      c.setAttribute(
        "stroke-dasharray",
        `${Math.max(0, (value / capacity) * C - 2.5)} ${C}`,
      );
      c.setAttribute("stroke-dashoffset", -start);
      c.setAttribute("transform", "rotate(-90 100 100)");
    }
    const label = {
      files: "Your files",
      clean: "Can be cleaned",
      sys: "System and apps",
      free: "Free",
    }[color];
    const bytes = base ? free : value;
    c.onpointermove = (e) => tooltip(e, { path: label, bytes });
    c.onpointerleave = () => ($("tooltip").hidden = true);
    const title = document.createElementNS(
      "http://www.w3.org/2000/svg",
      "title",
    );
    title.textContent = `${label}: ${gb(bytes)}, ${capacity ? ((bytes / capacity) * 100).toFixed(1) : "—"}% of disk`;
    c.append(title);
    svg.append(c);
  }
  circle(1, "free", 0, true);
  if (capacity)
    for (const [value, color] of segments) {
      if (value) circle(value, color, offset);
      offset += (value / capacity) * C;
    }
  svg.setAttribute(
    "aria-label",
    v
      ? `${gb(free)} free, ${percent}% full of ${gb(capacity)}`
      : "Disk capacity unavailable",
  );
  const legend = $("disk-legend");
  legend.replaceChildren();
  for (const [label, desc, value, color] of [
    ["Your files", "Kept in the scanned folder", files, "files"],
    ["Can be cleaned", "Caches, build output, old data", clean, "clean"],
    [
      "System and apps",
      "Everything outside the scanned folder",
      outside,
      "sys",
    ],
    ["Free", "", free, "free"],
  ]) {
    const row = dom("div", "li"),
      sw = dom("span", "sw"),
      text = dom("div");
    sw.style.background = `var(--c-${color})`;
    text.append(dom("div", "n", label));
    if (desc) text.append(dom("div", "d", desc));
    row.append(sw, text, dom("span", "v num", v ? gb(value) : "—"));
    legend.append(row);
  }
  $("disk-bar").replaceChildren();
  if (capacity)
    for (const [value, color] of segments) {
      const bar = dom("i");
      bar.style.width = (value / capacity) * 100 + "%";
      bar.style.background = `var(--c-${color})`;
      chartTooltip(
        bar,
        {
          files: "Your files",
          clean: "Can be cleaned",
          sys: "System and apps",
        }[color],
        value,
      );
      $("disk-bar").append(bar);
    }
  $("disk-caption").textContent = v
    ? `${gb(free)} free of ${gb(capacity)}`
    : "Disk capacity unavailable";
  $("nav-folders").textContent = fmt(scan.bytes);
  $("nav-categories").textContent = Object.keys(snapshot.categories).length;
  $("nav-cleanup-tab").textContent = (lowerBound ? "≥ " : "") + fmt(reclaim);
  $("home-total").textContent = scan.name + ", " + fmt(scan.bytes);
  const reclaimDisplay = reclaim > 0 && reclaim < 1e8 ? fmt(reclaim).split(" ") : [(reclaim / 1e9).toFixed(1), "GB"];
  $("reclaim-total").replaceChildren(
    document.createTextNode(
      (lowerBound ? "≥ " : "") + reclaimDisplay[0],
    ),
    dom("small", "", reclaimDisplay[1]),
  );
  $("reclaim-total").title = lowerBound
    ? "At least this much among the top 100 suggestions; actual reclaimed space can differ."
    : "Allocated bytes in disjoint suggestions; actual reclaimed space can differ.";
  $("rail-count").textContent =
    suggestions.length + (lowerBound ? "+" : "") + " suggestions";
  $("free-now").textContent = v ? gb(free) : "—";
  $("free-after").textContent = v
    ? (lowerBound ? "≥ " : "") + gb(Math.min(capacity, free + clean))
    : "—";
  $("bar-now").style.width = (capacity ? (free / capacity) * 100 : 0) + "%";
  $("bar-after").style.left = $("bar-now").style.width;
  $("bar-after").style.width = (capacity ? (clean / capacity) * 100 : 0) + "%";
  chartTooltip($("bar-now"), "Free now", free);
  chartTooltip($("bar-after"), "Potential cleanup", clean);
  const folders = ordered(scan),
    list = $("overview-folders");
  list.replaceChildren();
  const displayed = folders.slice(0, 7);
  const rest = scan.bytes - displayed.reduce((sum, n) => sum + n.bytes, 0);
  if (rest > 0)
    displayed.push({ name: "Everything else", bytes: rest, other: true });
  for (const n of displayed) {
    const row = dom("div", "fr" + (n.other ? " muted" : ""));
    row.tabIndex = 0; row.setAttribute("role", "button");
    row.onkeydown = e => { if (e.target === row && ["Enter", " "].includes(e.key)) { e.preventDefault(); row.click(); } };
    const tile = dom("span", "tile"),
      track = dom("span", "track"),
      fill = dom("i");
    tile.append(
      icon("fill-" + (n.other ? "dots-three-circle" : itemIcon(n.name))),
    );
    fill.style.width =
      Math.min(
        100,
        (n.bytes / Math.max(1, folders[0]?.bytes || scan.bytes)) * 100,
      ) + "%";
    track.append(fill);
    const label = dom("span", "n");
    label.append(dom("span", "nt", n.name));
    if (window.go?.main?.App && realPath(n)) label.append(itemActions(n.path));
    row.append(
      tile,
      label,
      track,
      dom(
        "span",
        "p num",
        ((n.bytes / Math.max(1, scan.bytes)) * 100).toFixed(1) + "%",
      ),
      dom("span", "s num", fmt(n.bytes)),
    );
    row.title = n.other ? "Explore all folders" : n.path;
    row.onclick = () => {
      showMode("folders", true);
      if (!n.other) navigate(n);
    };
    list.append(row);
  }
  if (!displayed.length)
    list.append(dom("p", "empty", "No folders in this scan."));
  fitOverviewFolders();
}
// The Overview list ends on a whole row; anything beyond scrolls inside the list.
function fitOverviewFolders() {
  const list = $("overview-folders"), card = list.closest(".folder-card"), row = list.querySelector(".fr");
  if (!row) return list.style.removeProperty("max-height");
  list.style.removeProperty("max-height");
  const ls = getComputedStyle(list), inner = parseFloat(ls.paddingTop) + parseFloat(ls.paddingBottom);
  const avail = card.getBoundingClientRect().bottom - (parseFloat(getComputedStyle(card).paddingBottom) || 0) - list.getBoundingClientRect().top;
  const step = row.getBoundingClientRect().height, height = Math.max(1, Math.floor((avail - inner) / step)) * step + inner;
  if (step && height < list.scrollHeight) list.style.maxHeight = height + "px";
}
let feedbackKey = "", dismissedFeedback = "", feedbackTimer;
function showFeedback(message, kind = "error") {
  if (/older cache format|older format|rescan with --refresh/i.test(message)) message = "";
  const key = kind + ":" + message;
  if (message && (key === dismissedFeedback || key === feedbackKey)) return;
  clearTimeout(feedbackTimer);
  feedbackKey = message ? key : "";
  const banner = $("scan-feedback");
  banner.className = "banner banner-" + kind;
  banner.hidden = !message;
  banner.replaceChildren();
  if (!message) return;
  banner.append(icon(kind === "info" ? "fill-info" : kind === "success" ? "fill-shield-check" : "fill-warning-circle"), dom("span", "feedback-copy", message));
  if (kind === "error" || /Scan canceled/.test(message)) {
    const retry = dom("button", "link", "Try Again");
    retry.onclick = () => { dismissedFeedback = ""; showFeedback(""); $("hub-scan").click(); };
    const choose = dom("button", "link", "Choose Folder…");
    choose.onclick = chooseScanFolder;
    banner.append(retry, choose);
  }
  const close = dom("button", "icon-action", "×");
  close.setAttribute("aria-label", "Dismiss status");
  close.onclick = () => { dismissedFeedback = key; showFeedback(""); };
  banner.append(close);
  if (kind !== "error") feedbackTimer = setTimeout(() => { dismissedFeedback = key; showFeedback(""); }, 6000);
}
// Quiet bottom-right confirmations for finished exports; errors keep the banner.
function showToast(message, path = "") {
  let stack = $("toasts");
  if (!stack) { stack = dom("div"); stack.id = "toasts"; stack.setAttribute("role", "status"); stack.setAttribute("aria-live", "polite"); document.body.append(stack); }
  const toast = dom("div", "toast");
  toast.append(icon("fill-check-circle"), dom("span", "toast-copy", message));
  toast.querySelector(".toast-copy").title = path || message;
  const app = window.go?.main?.App;
  if (path?.startsWith("/") && app?.Reveal) {
    const reveal = dom("button", "toast-action", "Show in Finder");
    reveal.onclick = () => app.Reveal(path).catch(e => showFeedback(String(e)));
    toast.append(reveal);
  }
  const close = dom("button", "icon-action toast-close");
  close.setAttribute("aria-label", "Dismiss");
  close.append(icon("regular-x"));
  let timer;
  const dismiss = () => {
    clearTimeout(timer);
    if (!toast.isConnected) return;
    if (reducedMotion() || !toast.animate) { toast.remove(); return; }
    toast.animate([{opacity: 1, transform: "none"}, {opacity: 0, transform: "translateY(6px)"}], {duration: 160, easing: "ease-in"}).onfinish = () => toast.remove();
  };
  const arm = () => { clearTimeout(timer); timer = setTimeout(dismiss, 5000); };
  close.onclick = dismiss;
  toast.onmouseenter = () => clearTimeout(timer);
  toast.onmouseleave = arm;
  toast.append(close);
  stack.append(toast);
  if (!reducedMotion()) toast.animate?.([{opacity: 0, transform: "translateY(8px)"}, {opacity: 1, transform: "none"}], {duration: 200, easing: "ease-out"});
  arm();
  return toast;
}
const scanProgressStrip = $("scan-strip");
let scanTarget = "Home", scanCounterTimer, displayedEntries = 0, desiredEntries = 0;
function scanningHeader() { if (snapshot) $("scan-info").textContent = "Scanning " + scanTarget + "…"; }
// The heading is the scanned folder; before any results it is empty and takes no space.
// Overview's hero title gently alternates "clearer" / "cleaner": only the fifth
// letter changes, crossfading inside a fixed-width cell so nothing else moves.
function setLaunchTitle(text) {
  const title = $("launch-title");
  title.setAttribute("aria-label", text);
  const swap = text === "Your disk, a little clearer.";
  title.classList.toggle("has-swap", swap);
  if (!swap) { title.textContent = text; return; }
  if (title.querySelector(".swap-letter")) return;
  const cell = dom("span", "swap-letter");
  cell.append(dom("span", "swap-r", "r"), dom("span", "swap-n", "n"));
  const visual = dom("span", "");
  visual.setAttribute("aria-hidden", "true");
  visual.append("Your disk, a little clea", cell, "er.");
  title.replaceChildren(visual);
}
document.addEventListener("visibilitychange", () => document.body.classList.toggle("page-hidden", document.hidden));
function renderHeading() {
  const root = snapshot?.scan?.root?.path || "";
  document.querySelector(".bar.intro").hidden = !root;
  const title = $("view-title");
  if (!root) { title.replaceChildren(); title.removeAttribute("title"); $("scan-info").textContent = ""; return; }
  const home = window.scanHome || "";
  const shown = home && root === home ? "~/" : home && root.startsWith(home + "/") ? "~" + root.slice(home.length) : root;
  const cut = shown.length > 1 ? shown.replace(/\/$/, "").lastIndexOf("/") : -1;
  const head = cut > 0 ? shown.slice(0, cut + 1) : "", tail = cut > 0 ? shown.slice(cut + 1) : shown;
  title.replaceChildren(dom("span", "path-head", head), dom("span", "path-tail", tail));
  title.title = root;
  $("scan-info").textContent = document.body.classList.contains("is-scanning") ? "Scanning " + scanTarget + "…" : lastScanSubtitle;
}
function updateScanProgress(state) {
  const phase = (state.phase || "walking").toLowerCase();
  $("progress").textContent = !state.busy ? state.phase :
    phase.includes("open") ? "Opening your scan…" :
    /class|scor/.test(phase) ? "Scoring cleanup…" :
    /aggreg|measur|siz|cache|writ/.test(phase) ? "Measuring sizes…" : "Walking folders…";
  desiredEntries = Math.max(desiredEntries, Number(state.entries) || 0);
  $("scan-spinner").setAttribute("aria-label", $("progress").textContent);
  $("scan-stats").textContent = `${Number(state.directories || 0).toLocaleString()} folders · ${new Intl.NumberFormat(undefined, {notation: "compact", maximumFractionDigits: 0}).format(state.rate || 0)}/s`;
  const path = state.path || "";
  $("scan-path").title = path;
  $("scan-path").textContent = path.length > 72 ? path.slice(0, 32) + "…" + path.slice(-37) : path;
  if (state.busy) scanningHeader();
}
function scanState(busy) {
  document.body.classList.toggle("is-scanning", busy);
  if (busy) {
    (snapshot ? $("scan-options") : document.querySelector(".launch-card")).append(scanProgressStrip);
    toggleNotes(false);
    dismissedFeedback = "";
    showFeedback("");
    scanTarget = window.scanTargetLabel?.() || $("folder-choice").textContent || "Home";
    $("scan-target").textContent = scanTarget;
    scanningHeader();
    displayedEntries = desiredEntries = 0;
    $("scan-count").textContent = "0";
    $("scan-stats").textContent = "0 folders · 0/s";
    $("scan-path").textContent = "";
    clearInterval(scanCounterTimer);
    scanCounterTimer = setInterval(() => {
      displayedEntries = reducedMotion() ? desiredEntries : Math.min(desiredEntries, displayedEntries + Math.max(1, Math.ceil((desiredEntries - displayedEntries) * .5)));
      $("scan-count").textContent = displayedEntries.toLocaleString();
    }, 100);
    $("folder-picker").open = false;
    $("rescan-picker").open = false;
  }
  $("hub-scan").disabled = busy;
  $("hub-scan").replaceChildren(
    icon("regular-arrow-clockwise"),
    document.createTextNode(
      snapshot ? "Rescan" : "Scan",
    ),
  );
  $("hub-scan").append(dom("span", "kbd", "⌘R"));
  $("scan-strip").hidden = !busy;
  window.agentViewChanged?.();
  if (busy) $("progress").textContent = "Walking folders…";
  else {
    clearInterval(scanCounterTimer);
    showMode(mode);
    pageFade();
  }
}
$("review-cleanup").onclick = () => {
  $("suggest-category").value = "";
  $("suggest-safety").value = "";
  showMode("cleanup-tab", true);
  refreshSuggestions();
};
$("show-folders").onclick = () => showMode("folders", true);
$("hub-scan").onclick = () => {
  const target = $(window.go?.main?.App ? "start-scan" : "web-start-scan");
  if (!target.disabled) return target.onclick();
};
async function chooseScanFolder() {
  if (window.go?.main?.App) return $("choose-folder").onclick();
  $("scan-options").hidden = false;
  $("web-root").focus();
  $("web-root").select();
}
$("scan-another").onclick = chooseScanFolder;
$("landing-choose").onclick = () => {
  $("folder-picker").open = false;
  return chooseScanFolder();
};
$("rescan-new").onclick = () => {
  $("rescan-picker").open = false;
  return chooseScanFolder();
};
// Portals keep menus above panels and out of scroll/overflow containers.
function floatingMenu(picker, onOpen = () => {}) {
  const menu = picker.querySelector(".agent-menu");
  const anchor = picker.querySelector("summary");
  menu.hidden = true;
  menu.id ||= picker.id + "-menu";
  anchor.setAttribute("aria-controls", menu.id);
  anchor.setAttribute("aria-expanded", "false");
  anchor.setAttribute("aria-haspopup", "true");
  function position() {
    if (!picker.open) return;
    const target = picker.id === "agent-picker" ? $("agent-control")
      : picker.id === "rescan-picker" ? picker.parentElement : anchor;
    const r = target.getBoundingClientRect(), gap = 8, edge = 12;
    menu.style.maxHeight = Math.max(80, innerHeight - edge * 2) + "px";
    const m = menu.getBoundingClientRect();
    const below = innerHeight - r.bottom - gap - edge;
    const above = r.top - gap - edge;
    const up = m.height > below && above > below;
    menu.style.maxHeight = Math.max(40, up ? above : below) + "px";
    const height = Math.min(m.height, Math.max(40, up ? above : below));
    menu.style.left = Math.max(edge, Math.min(r.left, innerWidth - m.width - edge)) + "px";
    menu.style.top = Math.max(edge, up ? r.top - gap - height : r.bottom + gap) + "px";
  }
  picker.addEventListener("toggle", () => {
    anchor.setAttribute("aria-expanded", String(picker.open));
    if (picker.open) {
      for (const other of document.querySelectorAll("#folder-picker, #rescan-picker, #agent-picker, #session-picker")) if (other !== picker) other.open = false;
      menu.classList.add("floating-menu");
      document.body.append(menu);
      menu.hidden = false;
      position();
      menu.querySelector('button[aria-pressed="true"], button:not(:disabled), a')?.focus({ preventScroll: true });
      onOpen();
    } else {
      menu.hidden = true;
      picker.append(menu);
    }
  });
  if (window.ResizeObserver) new ResizeObserver(position).observe(menu);
  window.addEventListener("resize", position);
  document.addEventListener("scroll", position, true);
  document.addEventListener("click", e => {
    if (!picker.contains(e.target) && !menu.contains(e.target)) picker.open = false;
  });
  document.addEventListener("keydown", e => {
    if (e.key === "Escape" && picker.open) {
      picker.open = false;
      anchor.focus();
    }
  });
}
floatingMenu($("folder-picker"));
floatingMenu($("rescan-picker"));
document.addEventListener("keydown", (e) => {
  if (e.target.closest?.("#terminal-screen")) return;
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "r") {
    e.preventDefault();
    if (!$("hub-scan").disabled) $("hub-scan").click();
  }
});
if (window.matchMedia)
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      if (snapshot) render();
    });

// Arrow keys move between adjacent rows; Tab still reaches every action.
document.addEventListener("keydown", (e) => {
  if (
    !["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key) ||
    e.altKey ||
    e.metaKey ||
    e.ctrlKey
  )
    return;
  const control = e.target;
  const list = control.closest(
    "#suggestion-list, #overview-folders, #children",
  );
  if (!list || !control.matches("button, [role=button], input[type=checkbox]")) return;
  const rows = [...list.querySelectorAll(".it, .fr, tr")];
  const row = control.closest(".it, .fr, tr");
  let i = rows.indexOf(row);
  if (i < 0) return;
  i =
    e.key === "Home"
      ? 0
      : e.key === "End"
        ? rows.length - 1
        : Math.max(
            0,
            Math.min(rows.length - 1, i + (e.key === "ArrowDown" ? 1 : -1)),
          );
  const target = rows[i].matches("button, [role=button]")
    ? rows[i]
    : rows[i].querySelector(
        control.matches("input") ? "input:not(:disabled)" : "button",
      );
  if (target) {
    e.preventDefault();
    target.focus();
  }
});

if (window.ResizeObserver) {
  let mapSize = "", mapFrame;
  new ResizeObserver(entries => {
    const r = entries[0].contentRect, size = `${Math.round(r.width)}:${Math.round(r.height)}`;
    if (size === mapSize || !r.width || !r.height) return;
    mapSize = size;
    cancelAnimationFrame(mapFrame);
    mapFrame = requestAnimationFrame(() => { if (snapshot && !$("explorer").hidden) render(); });
  }).observe(document.querySelector(".map-panel"));
}

if (window.ResizeObserver) new ResizeObserver(entries => {
  $("scan-options").style.setProperty("--scan-split-width", entries[0].target.offsetWidth + "px");
}).observe(document.querySelector("#scan-options .scan-split"));
if (window.ResizeObserver) {
  let fitFrame;
  new ResizeObserver(() => { cancelAnimationFrame(fitFrame); fitFrame = requestAnimationFrame(fitOverviewFolders); }).observe(document.querySelector(".folder-card"));
}

if (window.ResizeObserver) new ResizeObserver(() => {
  const list = $("suggestion-list");
  $("suggestions").style.setProperty("--cleanup-gutter", (list.offsetWidth - list.clientWidth + parseFloat(getComputedStyle(list).paddingRight)) + "px");
}).observe($("suggestion-list"));

// The Folders/Categories scrollbar track starts below the sticky header.
if (window.ResizeObserver) {
  const scroller = document.querySelector(".table-scroll"), head = scroller && scroller.querySelector("thead");
  if (head) new ResizeObserver(() => {
    scroller.style.setProperty("--scroll-head", Math.ceil(head.getBoundingClientRect().height + parseFloat(getComputedStyle(scroller).paddingTop)) + "px");
  }).observe(head);
}
