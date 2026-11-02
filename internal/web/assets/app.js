"use strict";
const $ = (id) => document.getElementById(id);
let snapshot,
  root,
  current,
  mode = "folders",
  view = "sunburst",
  sortKey = "bytes",
  sortAsc = false,
  shown = 200;
const parents = new WeakMap(),
  colors = new WeakMap();
const palette = [
  "#b698f2",
  "#7197ef",
  "#5ec7cb",
  "#8bc996",
  "#dfc176",
  "#ef9b79",
  "#e388b0",
  "#9b9ddb",
  "#67b6ed",
  "#c7a3cc",
  "#80b8a9",
  "#b4bf71",
];
let binaryUnits = false;
const fmt = (b) => {
 const base = binaryUnits ? 1024 : 1000;
 const units = binaryUnits ? ["KiB","MiB","GiB","TiB","PiB","EiB"] : ["kB","MB","GB","TB","PB","EB"];
 if (b < base) return b + " B";
 let i = -1; do { b /= base; i++; } while (b >= base && i < units.length-1);
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
  );
  t.hidden = false;
  t.style.left = Math.max(8, Math.min(e.clientX + 15, innerWidth - 395)) + "px";
  t.style.top = Math.max(8, Math.min(e.clientY + 15, innerHeight - 100)) + "px";
}
let navigationVersion=0;
async function loadChildren(n,offset=0) {
  if(n.synthetic && !n.category_name) return;
  const url=n.category_name ? `/api/category?name=${encodeURIComponent(n.category_name)}&offset=${offset}&sort=${sortKey}&asc=${Number(sortAsc)}` : `/api/node?id=${n.id}&depth=3&offset=${offset}&sort=${sortKey}&asc=${Number(sortAsc)}`;
  const r=await fetch(url);if(!r.ok)throw Error("Unable to load folder: "+r.status);
  const data=await r.json();
  if(offset) {n.children=[...(n.children||[]),...(data.children||[])];n.child_count=data.child_count;}
  else Object.assign(n,data);
  index(n,parents.get(n),colors.get(n));n.loaded=true;
}
async function navigate(n) {
  if(n.more_parent) {try{const p=n.more_parent;await loadChildren(p,(p.children||[]).length);current=p;shown+=200;render();}catch(e){$("scan-info").textContent=e.message;}return;}
  if(n.kind!=="directory" && !n.synthetic) return;
  const version=++navigationVersion;
  try {
    if(!n.synthetic || n.category_name) await loadChildren(n);
    if(version!==navigationVersion) return;
    current=n;shown=200;$("tooltip").hidden=true;render();
  }catch(e){$("scan-info").textContent=e.message;}
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
  canvas.width = w * scale; canvas.height = h * scale;
  canvas.style.width = "100%"; canvas.style.maxWidth = Math.min(w, 540*w/h) + "px";
  canvas.style.aspectRatio = `${w} / ${h}`;
  canvas.setAttribute("aria-label", "Space map. Use the folder table to navigate with a keyboard.");
  const ctx = canvas.getContext("2d"); ctx.scale(scale, scale);
  const regions = [];
  const hit = (e) => {
    const r = canvas.getBoundingClientRect();
    const x = (e.clientX-r.left)*w/r.width, y = (e.clientY-r.top)*h/r.height;
    return [...regions].reverse().find(z => ctx.isPointInPath(z.path, x*scale, y*scale));
  };
  canvas.onpointermove = e => { const z = hit(e); canvas.style.cursor = z ? "pointer" : "default"; if(z) tooltip(e,z.node); else $("tooltip").hidden=true; };
  canvas.onpointerleave = () => $("tooltip").hidden=true;
  canvas.onclick = e => { const z=hit(e); if(z) navigate(z.node); };
  return {canvas, ctx, regions};
}
function sunburst() {
  const {canvas, ctx, regions} = canvasChart(600,600);
  let segments=0;
  function ring(n,start,end,depth,base) {
    if(depth>5 || n.bytes<=0) return;
    let angle=start;
    for(const [i,c] of ordered(n).entries()) {
      const next=angle+(end-start)*c.bytes/n.bytes;
      const color=base || palette[i%palette.length];
      if(next-angle>0.004 && segments<1200) {
        const a=angle+0.0018,b=Math.min(next-0.0018,angle+2*Math.PI-0.0001);
        if(b>a) {
          const path=new Path2D(arc(86+(depth-1)*39,86+depth*39-2,a,b));
          ctx.fillStyle=color; ctx.globalAlpha=1-(depth-1)*0.105; ctx.fill(path);
          regions.push({path,node:c}); segments++;ring(c,angle,next,depth+1,color);
        }
      }
      angle=next;
    }
  }
  ring(current,0,Math.PI*2,1,null);ctx.globalAlpha=1;
  const center=new Path2D();center.arc(300,300,81,0,Math.PI*2);
  ctx.fillStyle="#191f2e";ctx.fill(center);ctx.strokeStyle="#30384d";ctx.stroke(center);
  regions.push({path:center,node:parents.get(current)||current});
  ctx.textAlign="center";ctx.fillStyle="#f0ecff";ctx.font="500 25px system-ui";ctx.fillText(fmt(current.bytes),300,287);
  const label=current.name||"/";ctx.fillStyle="#8e9ab0";ctx.font="11px system-ui";ctx.fillText(label.length>21?label.slice(0,19)+"…":label,300,311);
  ctx.fillStyle="#65738d";ctx.font="9px system-ui";ctx.fillText(parents.has(current)?"↑ PARENT FOLDER":"ALLOCATED ON DISK",300,334);
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
  const {canvas,ctx,regions}=canvasChart(680,500);
  const items=ordered(current).filter(n=>n.bytes>0).slice(0,199);
  const missing=current.bytes-(current.own_bytes||0)-items.reduce((sum,n)=>sum+n.bytes,0);
  if(missing>0) items.push({name:"Other entries",path:current.path+" / Other entries",bytes:missing,kind:"directory",synthetic:true,more_parent:current});
  for(const [i,r] of squarify(items,3,3,674,494).entries()) {
    if(r.w<1 || r.h<1) continue;
    const path=new Path2D();path.roundRect(r.x+2,r.y+2,Math.max(0,r.w-4),Math.max(0,r.h-4),5);
    ctx.fillStyle=palette[i%palette.length];ctx.globalAlpha=.85;ctx.fill(path);ctx.globalAlpha=1;regions.push({path,node:r.n});
    if(r.w>65 && r.h>44) {
      const max=Math.floor(r.w/7)-2;const name=r.n.name.length>max?r.n.name.slice(0,Math.max(1,max-1))+"…":r.n.name;
      ctx.fillStyle="#101522";ctx.font="600 12px system-ui";ctx.fillText(name,r.x+12,r.y+23);
      ctx.fillStyle="#20283c";ctx.font="11px system-ui";ctx.fillText(fmt(r.n.bytes),r.x+12,r.y+42);
    }
  }
  return canvas;
}

function render() {
  if (mode === "cleanup-tab") return;
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
  if(window.go?.main?.App && !current.synthetic) $("folder-actions").append(itemActions(current.path));
  $("map-label").textContent =
    mode === "categories" ? "CATEGORY MAP" : "SPACE MAP";
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
      dot,
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
  if (children.length > shown || (current.child_count||0)>children.length) {
    const tr = dom("tr"),
      td = dom("td"),
      b = dom("button", "", `Show more (${Math.max(children.length,current.child_count||0) - shown} remaining)`);
    td.colSpan = 4;
    b.addEventListener("click", async () => {
      b.disabled=true;
      try {if(shown>=children.length) await loadChildren(current,children.length);shown+=200;render();}
      catch(e){b.textContent=e.message;b.disabled=false;}
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
    td.colSpan = 4;
    tr.append(td);
    body.append(tr);
  }
}
function categoryTree() {
  const r={name:"All categories",path:"Categories",bytes:snapshot.scan.root.bytes,kind:"directory",synthetic:true,children:[]};
  for(const [name,bytes] of Object.entries(snapshot.categories)) r.children.push({name,path:name,bytes,kind:"directory",synthetic:true,category_name:name,children:[]});
  index(r);return r;
}

function showMode(next) {
  mode = next;
  ["folders", "categories", "cleanup-tab"].forEach((id) =>
    $(id).classList.toggle("active", id === mode),
  );
  $("explorer").hidden = mode === "cleanup-tab";
  $("suggestions").hidden = mode !== "cleanup-tab";
  document.querySelector(".view-switch").hidden = mode === "cleanup-tab";
  if (mode === "cleanup-tab") return;
  root = mode === "categories" ? categoryTree() : snapshot.scan.root;
  current = root;
  shown = 200;
  render();
}
function renderSuggestions() {
  const list = $("suggestion-list");
  list.replaceChildren();
  suggestionItems.forEach((s, i) => {
    const card = dom("tr", "suggestion-card"),
      body = dom("td"),
      badges = dom("div", "badges");
    body.append(selectionBox(s), dom("h3", "", s.path));
    badges.append(
      dom("span", "badge " + s.safety, s.safety.toUpperCase()),
      dom("span", "badge", s.category),
      dom("span", "badge", s.badge),
    );
    if (s.incomplete)
      badges.append(dom("span", "badge user-data", "PARTIAL SCAN"));
    body.append(badges, dom("p", "", s.explanation));
    if (s.project_path)
      body.append(dom("p", "", "Project activity: " + s.project_path));
    if (s.suggested_command) body.append(dom("code", "", s.suggested_command));
    if (window.go?.main?.App) body.append(itemActions(s.path, s.suggested_command));
    card.append(body,dom("td","suggestion-score",Math.round(s.score)+"/100"),dom("td","suggestion-size",fmt(s.bytes)),dom("td","suggestion-stale",s.stale_days+" days"));
    list.append(card);
  });
  if (!suggestionItems.length)
    {const row=dom("tr"),cell=dom("td","empty","No cleanup suggestions match these filters above the scan threshold.");cell.colSpan=4;row.append(cell);list.append(row)}
}
$("sunburst").onclick = () => {
  view = "sunburst";
  $("sunburst").classList.add("active");
  $("treemap").classList.remove("active");
  render();
};
$("treemap").onclick = () => {
  view = "treemap";
  $("treemap").classList.add("active");
  $("sunburst").classList.remove("active");
  render();
};
["folders", "categories", "cleanup-tab"].forEach(
  (m) => ($(m).onclick = () => showMode(m)),
);
document.querySelectorAll("[data-sort]").forEach(
  (b) =>
    (b.onclick = async () => {
      sortAsc =
        sortKey === b.dataset.sort ? !sortAsc : b.dataset.sort === "name";
      sortKey = b.dataset.sort;
      try{await loadChildren(current);shown=200;render();}catch(e){$("scan-info").textContent=e.message;}
    }),
);
$("details-toggle").onclick = () => ($("notes").hidden = !$("notes").hidden);
function loadSnapshot() { return fetch("/api/scan")
  .then((r) => {
    if (!r.ok) throw Error("Unable to load scan: " + r.status);
    return r.json();
  })
  .then((s) => {
    snapshot = s;
    $("skipped").replaceChildren();
    renderVolume();
    index(s.scan.root);
    $("total").textContent = fmt(s.scan.root.bytes);
    $("scan-info").textContent =
      s.scan.root.path +
      " · " +
      Number(s.scan.root.files).toLocaleString() +
      " files · " +
      new Date(s.scan.finished).toLocaleString();
    $("accounting").textContent = s.note;
    $("scan-status").textContent = (s.skipped_count || s.scan.skipped.length)
      ? `${s.skipped_count || s.scan.skipped.length} skipped paths · partial totals · local only`
      : "Local scan · no data leaves this device";
    if (s.scan.skipped.length)
      $("skipped").append(dom("h3", "", "Skipped paths"));
    for (const p of s.scan.skipped.slice(0, 500))
      $("skipped").append(dom("p", "", p.path + " — " + p.reason));
    selected.clear(); updateSelection();
    for(const [id,values] of [["suggest-category",Object.keys(s.categories)],["suggest-safety",["cache","reinstallable","user-data","unknown"]]]) {
     const select=$(id),value=select.value;select.replaceChildren(new Option("All", ""));for(const v of values)select.add(new Option(v,v));select.value=value;
    }
    refreshSuggestions();
    showMode("folders");
  })
  .catch((e) => {
    $("scan-info").textContent = e.message;
  });
}

function renderVolume() {
 if (!snapshot?.volume) return;
 const v = snapshot.volume;
 $("volume").hidden = false;
 $("volume-values").textContent = `${fmt(v.capacity)} capacity · ${fmt(v.used)} used · ${fmt(v.available)} available · ${fmt(v.outside)} outside scanned folder`;
 $("volume-note").textContent = snapshot.volume_note + (v.excess ? ` Scanned allocation exceeds used by ${fmt(v.excess)} (shared blocks or cloud placeholders).` : "");
}
$("binary-units").checked = binaryUnits;
$("binary-units").onchange = (e) => {
 binaryUnits = e.target.checked; localStorage.setItem("binaryUnits",String(binaryUnits));
 if(snapshot){$("total").textContent=fmt(snapshot.scan.root.bytes);renderVolume();renderSuggestions();render();}
};
if (!window.go?.main?.App) loadSnapshot();

let suggestionSort="score",suggestionAsc=false,suggestionItems=[];
let suggestionVersion=0;
async function refreshSuggestions(){
 const version=++suggestionVersion;
 try{const r=await fetch(`/api/suggest?sort=${suggestionSort}&asc=${Number(suggestionAsc)}&category=${encodeURIComponent($("suggest-category").value)}&safety=${encodeURIComponent($("suggest-safety").value)}&limit=500`);if(!r.ok)throw Error(await r.text());const items=await r.json();if(version!==suggestionVersion)return;suggestionItems=items;renderSuggestions();}
 catch(e){$("scan-info").textContent=e.message}
}
for(const b of document.querySelectorAll("[data-suggest-sort]"))b.onclick=()=>{suggestionAsc=b.dataset.suggestSort===suggestionSort?!suggestionAsc:false;suggestionSort=b.dataset.suggestSort;for(const c of document.querySelectorAll("[data-suggest-sort]"))c.classList.toggle("active",c.dataset.suggestSort===suggestionSort);refreshSuggestions()};
$("suggest-category").onchange=$("suggest-safety").onchange=()=>refreshSuggestions();
const selected=new Map();
function selectionBox(n){
 const box=dom("input");box.type="checkbox";box.dataset.selectionPath=n.path;box.setAttribute("aria-label","Select "+n.path);box.checked=selected.has(n.path);
 box.disabled=!!n.synthetic||n.kind==="symlink"||n.path===snapshot?.scan.root.path;
 box.onchange=()=>{
  if(box.checked){for(const p of selected.keys())if(n.path.startsWith(p+"/")||p.startsWith(n.path+"/")){box.checked=false;$("trash-results").textContent="Select separate items without parent/child overlaps.";return}selected.set(n.path,n.bytes)}else selected.delete(n.path);
  updateSelection();
 };return box;
}
function updateSelection(){
 for(const box of document.querySelectorAll("[data-selection-path]"))box.checked=selected.has(box.dataset.selectionPath);
 $("selection-bar").hidden=selected.size===0;$("selection-total").textContent=`${selected.size} selected · ${fmt([...selected.values()].reduce((a,b)=>a+b,0))}`;
}
async function action(path,data){const r=await fetch(path,{method:"POST",headers:{"Content-Type":"application/json","X-OMD-Action":"1"},body:JSON.stringify(data)});if(!r.ok)throw Error(await r.text());return r.json()}
$("clear-selection").onclick=()=>{selected.clear();updateSelection();renderSuggestions();render()};
$("trash-selected").onclick=()=>{$("confirm-total").textContent=$("selection-total").textContent;$("confirm-paths").textContent=[...selected.keys()].join("\n");$("trash-confirm").showModal()};
$("cancel-trash").onclick=()=>$("trash-confirm").close();
$("confirm-trash").onclick=async()=>{
 const b=$("confirm-trash");b.disabled=true;
 try{const result=await action("/api/trash",{paths:[...selected.keys()]});$("trash-confirm").close();await loadSnapshot();
  const message=(result.results||[]).map(r=>r.path+": "+(r.trashed?"Moved to Trash":"Failed: "+r.error)+(r.trashed&&r.error?" · "+r.error:"")).join("\n")+(result.scan_error?"\nTotals could not refresh: "+result.scan_error:"");
  $("trash-results").textContent=message;$("selection-bar").hidden=false;
 }catch(e){$("trash-results").textContent=String(e);$("selection-bar").hidden=false;$("trash-confirm").close()}finally{b.disabled=false}
};
if(!window.go?.main?.App){
 $("web-scan").hidden=false;
 $("web-start-scan").onclick=async()=>{const b=$("web-start-scan");b.disabled=true;$("web-scan-status").textContent="Scanning…";try{const result=await action("/api/rescan",{root:$("web-root").value});if(result.scan_error)throw Error(result.scan_error);await loadSnapshot();$("web-scan-status").textContent="Complete"}catch(e){$("web-scan-status").textContent=String(e)}finally{b.disabled=false}};
}
