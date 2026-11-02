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
let binaryUnits = false;
try {
  binaryUnits = localStorage.getItem("binaryUnits") === "true";
} catch {}
const fmt = (b) => {
  const base = binaryUnits ? 1024 : 1000;
  const units = binaryUnits
    ? ["KiB", "MiB", "GiB", "TiB", "PiB", "EiB"]
    : ["kB", "MB", "GB", "TB", "PB", "EB"];
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
    document.createTextNode(n.path),
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
  t.style.left = Math.max(8, Math.min(e.clientX + 15, innerWidth - 395)) + "px";
  t.style.top = Math.max(8, Math.min(e.clientY + 15, innerHeight - 100)) + "px";
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
  } else Object.assign(n, data);
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
  canvas.width = w * scale;
  canvas.height = h * scale;
  canvas.style.width = "100%";
  canvas.style.maxWidth = Math.min(w, (540 * w) / h) + "px";
  canvas.style.aspectRatio = `${w} / ${h}`;
  canvas.setAttribute(
    "aria-label",
    "Space map. Use the folder table to navigate with a keyboard.",
  );
  const ctx = canvas.getContext("2d");
  ctx.scale(scale, scale);
  const regions = [];
  const hit = (e) => {
    const r = canvas.getBoundingClientRect();
    const x = ((e.clientX - r.left) * w) / r.width,
      y = ((e.clientY - r.top) * h) / r.height;
    return [...regions]
      .reverse()
      .find((z) => ctx.isPointInPath(z.path, x * scale, y * scale));
  };
  canvas.onpointermove = (e) => {
    const z = hit(e);
    canvas.style.cursor = z ? "pointer" : "default";
    if (z) tooltip(e, z.node);
    else $("tooltip").hidden = true;
  };
  canvas.onpointerleave = () => ($("tooltip").hidden = true);
  canvas.onclick = (e) => {
    const z = hit(e);
    if (z) navigate(z.node);
  };
  return { canvas, ctx, regions };
}
function sunburst() {
  const { canvas, ctx, regions } = canvasChart(600, 600);
  let segments = 0;
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
            arc(86 + (depth - 1) * 39, 86 + depth * 39 - 2, a, b),
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
  center.arc(300, 300, 81, 0, Math.PI * 2);
  const theme = getComputedStyle(document.documentElement);
  ctx.fillStyle = theme.getPropertyValue("--card").trim() || "#fff";
  ctx.fill(center);
  ctx.strokeStyle = "#ffffff18";
  ctx.stroke(center);
  regions.push({ path: center, node: parents.get(current) || current });
  ctx.textAlign = "center";
  ctx.fillStyle = theme.getPropertyValue("--label").trim() || "#1d1d1f";
  ctx.font = "300 25px Inter";
  ctx.fillText(fmt(current.bytes), 300, 287);
  const label = current.name || "/";
  ctx.fillStyle = "#a5aebb";
  ctx.font = "12px Inter";
  ctx.fillText(label.length > 21 ? label.slice(0, 19) + "…" : label, 300, 311);
  ctx.fillStyle = "#8a93a0";
  ctx.font = "12px Inter";
  ctx.fillText(
    parents.has(current) ? "↑ Parent folder" : "Allocated on disk",
    300,
    334,
  );
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
  const { canvas, ctx, regions } = canvasChart(680, 500);
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
  for (const [i, r] of squarify(items, 3, 3, 674, 494).entries()) {
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
    if (r.w > 65 && r.h > 44) {
      const max = Math.floor(r.w / 7) - 2;
      const name =
        r.n.name.length > max
          ? r.n.name.slice(0, Math.max(1, max - 1)) + "…"
          : r.n.name;
      const lightTile =
        parseInt(palette[i % palette.length].slice(1, 3), 16) > 145;
      ctx.fillStyle = lightTile ? "#080b10" : "#ffffff";
      ctx.font = "600 12px system-ui";
      ctx.fillText(name, r.x + 12, r.y + 23);
      ctx.fillStyle = lightTile ? "#151a22" : "#e5e9f0";
      ctx.font = "12px Inter";
      ctx.fillText(fmt(r.n.bytes), r.x + 12, r.y + 42);
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
      n === root ? (mode === "categories" ? "Categories" : n.path) : n.name,
    );
    b.addEventListener("click", () => navigate(n));
    $("breadcrumbs").append(b);
  });
  $("folder-name").textContent = current.name || "/";
  $("folder-size").textContent = fmt(current.bytes);
  $("folder-actions").replaceChildren();
  if (window.go?.main?.App && !current.synthetic)
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
      s.append(dot, document.createTextNode(c.name));
      $("legend").append(s);
    });
  renderTable();
}
function renderTable() {
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
      b = dom("button"),
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
      dom("span", "filename", n.name + (n.kind === "symlink" ? " ↗" : "")),
    );
    b.title = n.path;
    b.addEventListener("click", () => navigate(n));
    name.append(selectionBox(n), b);
    const bar = dom("div", "size-bar"),
      fill = dom("i");
    fill.style.width =
      Math.min(100, (n.bytes / Math.max(1, current.bytes)) * 100) + "%";
    fill.style.background = dot.style.background;
    bar.append(fill);
    name.append(bar);
    if (window.go?.main?.App) name.append(itemActions(n.path));
    tr.append(
      name,
      dom(
        "td",
        "",
        ((n.bytes / Math.max(1, current.bytes)) * 100).toFixed(1) + "%",
      ),
      dom("td", "", fmt(n.bytes)),
      dom("td", "", Number(n.files || 0).toLocaleString()),
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

function showMode(next) {
  mode = next;
  ["overview", "folders", "categories", "cleanup-tab"].forEach((id) => {
    $(id).classList.toggle("active", id === mode);
    if (id === mode) $(id).setAttribute("aria-current", "page");
    else $(id).removeAttribute("aria-current");
  });
  $("view-title").textContent = {
    overview: "Overview",
    folders: "Folders",
    categories: "Categories",
    "cleanup-tab": "Cleanup",
  }[mode];
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
    }[mode];
    const target = window.scanTargetLabel?.() || "Home";
    $("scan-info").textContent =
      "Scan " + target + " to see what’s taking space.";
    $("disk-caption").textContent = target + " is ready to scan";
    $("empty-title").textContent = copy[0];
    $("empty-description").textContent =
      copy[1] + " Scan " + target + " to begin.";
    $("launch-cards").hidden = false;
    $("empty-scan").hidden = true;
    window.agentViewChanged?.();
    document.querySelector(".view-switch").hidden = true;
    return;
  }
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
  shown = 200;
  render();
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
    name.onclick = () => {
      details.hidden = !details.hidden;
      name.setAttribute("aria-expanded", String(!details.hidden));
    };
    details.append(
      dom("p", "", s.path),
      dom("p", "", `Rule: ${s.rule}. ${s.explanation}`),
      dom(
        "p",
        "",
        `Score ${Math.round(s.score)}: ${s.personal ? "your files; age does not boost ranking" : s.safety === "cache" ? "safe cache" : s.safety === "reinstallable" ? "reinstallable data" : "review before deleting"}, ${fmt(s.bytes)} locally allocated, unused ${s.stale_days} days.${s.personal ? " Personal files are capped at 30. Synced changes can affect all devices." : s.rule === "big-unknown" || s.incomplete ? " Unrecognized or partial items are capped at 40." : ""}`,
      ),
    );
    if (s.incomplete)
      details.append(dom("p", "", "Partial scan: review before cleaning."));
    if (s.project_path)
      details.append(dom("p", "", "Project activity: " + s.project_path));
    if (s.suggested_command)
      details.append(dom("code", "", s.suggested_command));
    if (window.go?.main?.App)
      details.append(itemActions(s.path, s.suggested_command));
    title.append(
      name,
      dom("div", "d", `${s.category}, ${s.stale_days} days unused`),
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
  updateSelection();
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
["overview", "folders", "categories", "cleanup-tab"].forEach(
  (m) => ($(m).onclick = () => showMode(m)),
);
document.querySelectorAll("[data-sort]").forEach(
  (b) =>
    (b.onclick = async () => {
      sortAsc =
        sortKey === b.dataset.sort ? !sortAsc : b.dataset.sort === "name";
      sortKey = b.dataset.sort;
      try {
        await loadChildren(current);
        shown = 200;
        render();
      } catch (e) {
        $("scan-info").textContent = e.message;
      }
    }),
);
$("details-toggle").setAttribute("aria-expanded", "false");
$("details-toggle").setAttribute("aria-controls", "notes");
$("details-toggle").onclick = () => {
  $("notes").hidden = !$("notes").hidden;
  $("details-toggle").setAttribute("aria-expanded", String(!$("notes").hidden));
};
function loadSnapshot() {
  return fetch("/api/scan")
    .then((r) => {
      if (!r.ok) throw Error("Unable to load scan: " + r.status);
      return r.json();
    })
    .then(async (s) => {
      snapshot = s;
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
      $("scan-info").textContent =
        `Last scan ${ago}, took ${seconds} ${seconds === 1 ? "second" : "seconds"}`;
      $("scan-info").title =
        s.scan.root.path + ", " + finished.toLocaleString();
      $("web-root").value = s.scan.root.path;
      $("welcome").hidden = true;
      $("content-grid").hidden = false;
      $("accounting").textContent = s.note;
      $("scan-status").textContent =
        s.skipped_count || s.scan.skipped.length
          ? `${s.skipped_count || s.scan.skipped.length} skipped paths · partial totals · local only`
          : "Local scan · no data leaves this device";
      if (s.scan.skipped.length)
        $("skipped").append(dom("h3", "", "Skipped paths"));
      for (const p of s.scan.skipped.slice(0, 500))
        $("skipped").append(dom("p", "", p.path + " — " + p.reason));
      selected.clear();
      updateSelection();
      for (const [id, values] of [
        ["suggest-category", Object.keys(s.categories)],
        ["suggest-safety", ["cache", "reinstallable", "user-data", "unknown"]],
      ]) {
        const select = $(id),
          value = select.value;
        select.replaceChildren(new Option("All", ""));
        for (const v of values) select.add(new Option(v, v));
        select.value = value;
      }
      await refreshSuggestions();
      renderOverview();
      showMode(mode);
    })
    .catch((e) => {
      $("scan-info").textContent =
        "No scan is open. Choose a folder or scan again.";
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
  $("volume-values").textContent =
    `${fmt(v.capacity)} capacity · ${fmt(v.used)} used · ${fmt(v.available)} available · ${fmt(v.outside)} outside scanned folder`;
  $("volume-note").textContent =
    snapshot.volume_note +
    (v.excess
      ? ` Scanned allocation exceeds used by ${fmt(v.excess)} (shared blocks or cloud placeholders).`
      : "");
}
$("binary-units").checked = binaryUnits;
for (const [id, on] of [
  ["units-decimal", !binaryUnits],
  ["units-binary", binaryUnits],
]) {
  $(id).classList.toggle("active", on);
  $(id).setAttribute("aria-pressed", String(on));
}
$("binary-units").onchange = (e) => {
  binaryUnits = e.target.checked;
  try {
    localStorage.setItem("binaryUnits", String(binaryUnits));
  } catch {}
  for (const [id, on] of [
    ["units-decimal", !binaryUnits],
    ["units-binary", binaryUnits],
  ]) {
    $(id).classList.toggle("active", on);
    $(id).setAttribute("aria-pressed", String(on));
  }
  if (snapshot) {
    $("total").textContent = fmt(snapshot.scan.root.bytes);
    renderVolume();
    renderOverview();
    renderSuggestions();
    render();
  }
};
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
  $("trash-confirm").showModal();
};
$("cancel-trash").onclick = () => $("trash-confirm").close();
$("confirm-trash").onclick = async () => {
  const b = $("confirm-trash");
  b.disabled = true;
  const sizes = new Map(selected);
  try {
    const result = await action("/api/trash", { paths: [...selected.keys()] });
    $("trash-confirm").close();
    await loadSnapshot();
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
    $("trash-results").replaceChildren(
      dom("span", "", moved.length ? `${fmt(bytes)} moved to Trash. Review more items or scan again.` : "No items moved. Review details and try again."), details);
    $("selection-bar").hidden = true;
  } catch (e) {
    $("trash-results").textContent = String(e);
    $("selection-bar").hidden = selected.size === 0;
    $("trash-confirm").close();
  } finally {
    b.disabled = false;
  }
};
if (!window.go?.main?.App) {
  $("web-scan").hidden = false;
  $("web-start-scan").onclick = async () => {
    const b = $("web-start-scan");
    b.disabled = true;
    $("web-scan-status").textContent = "Scanning…";
    setOptionsOpen(false);
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
  $("reclaim-total").replaceChildren(
    document.createTextNode(
      (lowerBound ? "≥ " : "") + (reclaim / 1e9).toFixed(1),
    ),
    dom("small", "", "GB"),
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
    const row = dom("button", "fr" + (n.other ? " muted" : ""));
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
    row.append(
      tile,
      dom("span", "n", n.name),
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
      showMode("folders");
      if (!n.other) navigate(n);
    };
    list.append(row);
  }
  if (!displayed.length)
    list.append(dom("p", "empty", "No folders in this scan."));
}
function showFeedback(message) {
  // Cache compatibility is an app concern, never a user-facing recovery flow.
  if (/older cache format|older format|rescan with --refresh/i.test(message))
    message = "";
  $("scan-feedback").hidden = !message;
  $("scan-feedback").replaceChildren(
    icon("fill-warning-circle"),
    dom("span", "", message),
  );
}
function scanState(busy, message = "Scanning…") {
  document.body.classList.toggle("is-scanning", busy);
  $("hub-scan").disabled = busy;
  $("hub-scan").replaceChildren(
    icon("regular-arrow-clockwise"),
    document.createTextNode(
      busy ? "Scanning…" : snapshot ? "Scan Again" : "Scan",
    ),
  );
  if (!busy) $("hub-scan").append(dom("span", "kbd", "⌘R"));
  $("scan-strip").hidden = !busy;
  $("empty-scan").disabled = busy;
  window.agentViewChanged?.();
  if (busy) $("progress").textContent = message;
  else showFeedback("");
}
$("review-cleanup").onclick = () => {
  $("suggest-category").value = "";
  $("suggest-safety").value = "";
  showMode("cleanup-tab");
  refreshSuggestions();
};
$("show-folders").onclick = () => showMode("folders");
$("hub-scan").onclick = () => {
  const target = $(window.go?.main?.App ? "start-scan" : "web-start-scan");
  if (!target.disabled) return target.onclick();
};
$("toolbar-choose").onclick = () => {
  setOptionsOpen(true);
  if (window.go?.main?.App) return $("choose-folder").onclick();
  $("web-root").focus();
  $("web-root").select();
};
$("scan-another").onclick = $("welcome-scan").onclick = () =>
  $("toolbar-choose").click();
$("empty-scan").onclick = () => $("hub-scan").click();
function setOptionsOpen(open) {
  $("scan-options").hidden = !open;
  $("options-toggle").setAttribute("aria-expanded", String(open));
  if (open) {
    const main = document
      .querySelector(".main-content")
      .getBoundingClientRect();
    const bar = document.querySelector(".bar").getBoundingClientRect();
    $("scan-options").style.top = bar.bottom - main.top + 8 + "px";
    $(window.go?.main?.App ? "scan-root" : "web-root").focus();
  }
}
$("options-toggle").onclick = () => setOptionsOpen($("scan-options").hidden);
$("options-close").onclick = () => {
  setOptionsOpen(false);
  $("options-toggle").focus();
};
document.addEventListener("pointerdown", (e) => {
  if (
    !$("scan-options").hidden &&
    !e.target.closest("#scan-options, #options-toggle, #toolbar-choose")
  )
    setOptionsOpen(false);
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !$("scan-options").hidden) {
    setOptionsOpen(false);
    $("options-toggle").focus();
  }
});
for (const [id, binary] of [
  ["units-decimal", false],
  ["units-binary", true],
]) {
  $(id).onclick = () => {
    $("binary-units").checked = binary;
    $("binary-units").dispatchEvent(new Event("change"));
    for (const [other, on] of [
      ["units-decimal", !binary],
      ["units-binary", binary],
    ]) {
      $(other).classList.toggle("active", on);
      $(other).setAttribute("aria-pressed", String(on));
    }
  };
}
// Reserve each button's layout box; only its painted surface grows by 2px.
document.addEventListener("pointerover", (e) => {
  const b = e.target.closest("button");
  if (!b || b.disabled) return;
  b.style.setProperty("--hover-x", 1 + 2 / Math.max(1, b.offsetWidth));
  b.style.setProperty("--hover-y", 1 + 2 / Math.max(1, b.offsetHeight));
});
document.addEventListener("keydown", (e) => {
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
  if (!list || !control.matches("button, input[type=checkbox]")) return;
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
  const target = rows[i].matches("button")
    ? rows[i]
    : rows[i].querySelector(
        control.matches("input") ? "input:not(:disabled)" : "button",
      );
  if (target) {
    e.preventDefault();
    target.focus();
  }
});
