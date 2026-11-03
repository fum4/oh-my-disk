const { JSDOM, VirtualConsole } = require("jsdom");
const fs = require("node:fs"),
  assert = require("node:assert/strict");
const url = process.argv[2];
if (!url) throw Error("Usage: node scripts/ui-smoke.cjs <fixture-ui-url>");
const token = new URL(url).searchParams.get("token");
const wait = async (fn) => {
  for (let i = 0; i < 150; i++) {
    if (fn()) return;
    await new Promise((r) => setTimeout(r, 25));
  }
  throw Error("Timed out: " + fn);
};
(async () => {
  const fixture = process.cwd()+"/.fixtures/home";
  let remembered=[{path:fixture+"/Library/Caches/pnpm",bytes:1_200_000_000,kind:"directory",rule:"pnpm",returned:true},{path:fixture+"/node_modules",bytes:1000,kind:"directory",rule:"node-modules",returned:true},{path:fixture+"/absent",bytes:20,kind:"file",returned:false}];
  const errors = [],
    requests = [];
  const vc = new VirtualConsole();
  vc.on("jsdomError", (e) => errors.push(e.message));
  const app = await JSDOM.fromURL(url, {
    resources: "usable",
    runScripts: "dangerously",
    pretendToBeVisual: true,
    virtualConsole: vc,
    beforeParse(w) {
      w.fetch = async (path, options = {}) => {
        requests.push(path);
        if (path === "/api/remembered") {
          if (options.method === "POST") {const inData=JSON.parse(options.body); remembered = inData.forget_all ? [] : remembered.filter(x=>!inData.paths.includes(x.path));return {ok:true,json:async()=>({ok:true})};}
          return {ok:true,json:async()=>({items:remembered,matches:remembered.filter(x=>x.returned).map(x=>({...x,current_bytes:x.bytes*4,current_kind:x.kind,growth:x.bytes*3,warning:true}))})};
        }
        return fetch(new URL(path, url), {
          ...options,
          headers: {
            ...options.headers,
            Cookie: "omd=" + token,
            Origin: new URL(url).origin,
          },
        });
      };
      w.Path2D = class {
        arc() {}
        roundRect() {}
      };
      w.HTMLCanvasElement.prototype.getContext = function () {
        return new Proxy(
          {},
          {
            get: (o, k) => o[k] || (() => {}),
            set: (o, k, v) => ((o[k] = v), true),
          },
        );
      };
      w.HTMLDialogElement.prototype.showModal = function () {
        this.open = true;
      };
      w.HTMLDialogElement.prototype.close = function () {
        this.open = false;
      };
    },
  });
  const w = app.window,
    d = w.document,
    $ = (id) => d.getElementById(id);
  await wait(()=>d.querySelector("#overview-folders .fr"));
  assert.match($("remembered-overview").textContent,/2 items you cleaned before/);
  assert.equal($("selection-bar").hidden,true,"Overview mention stays quiet until Cleanup opens");
  $("cleanup-tab").click();
  assert.equal($("remembered-card").hidden,false);
  assert.match($("remembered-card").textContent,/Back since last time · 2 items/);
  assert.equal(d.querySelectorAll("#remembered-card input:checked").length,2);
  const warning=d.querySelector(".growth-warning");
  assert.match($(warning.getAttribute("aria-describedby")).textContent,/was 1.2 GB, now 4.8 GB/);
  const label=d.querySelector(".remembered-choice"); label.click();
  assert.equal(d.querySelectorAll("#remembered-card input:checked").length,1);
  d.querySelector("#remembered-card .btn.pri").click();
  assert.equal($("trash-confirm").open,true);
  assert.equal($("remember-choice").checked,true);
  assert.match($("confirm-paths").textContent,/node_modules/);
  assert.doesNotMatch($("confirm-paths").textContent,/pnpm/);
  $("cancel-trash").click();
  d.querySelector(".remembered-heading button").click();
  assert.equal($("remembered-manager").open,true);
  assert.equal(d.querySelectorAll(".remembered-managed").length,3,"Manager includes absent choices");
  await d.querySelector(".remembered-managed button").onclick();
  assert.equal(d.querySelectorAll(".remembered-managed").length,2);
  $("remembered-close").click();
  for(const key of ["name","safety","score","size"]) {
    const b=d.querySelector(`[data-cleanup-sort="${key}"]`); await b.onclick();
    assert.equal(b.closest("th").getAttribute("aria-sort"),key==="name"?"ascending":"descending");
    await b.onclick(); assert.equal(b.closest("th").getAttribute("aria-sort"),key==="name"?"descending":"ascending");
  }
  $("folders").click();
  async function checkSort(){
    for(const key of ["name","bytes","files","modified"]){
      const b=d.querySelector(`[data-sort="${key}"]`);
      await b.onclick(); const first=[...d.querySelectorAll("#children .filename")].map(n=>n.textContent);
      const direction=b.closest("th").getAttribute("aria-sort");
      await b.onclick(); const second=[...d.querySelectorAll("#children .filename")].map(n=>n.textContent);
      assert.notEqual(b.closest("th").getAttribute("aria-sort"),direction);
      if(key==="name"&&first.length>1) assert.notDeepEqual(first,second);
      assert.equal($("scan-feedback").hidden,true,"Sorting has no error");
    }
  }
  await checkSort();
  await w.eval(`(async()=>{const first=root.children.find(n=>n.kind==="directory"&&n.children?.some(c=>c.kind==="directory"));await navigate(first);const second=current.children.find(n=>n.kind==="directory"&&n.children?.length>1);if(!second)throw Error("Missing depth 2 fixture");await navigate(second)})()`);
  await checkSort();
  $("categories").click(); await checkSort();
  d.querySelector("#children .filename").click();
  await wait(()=>w.eval("current !== root")); await checkSort();
  // Synthetic buckets never request a filesystem node.
  await w.eval(`navigate({name:"Unknown",path:"Unknown",synthetic:true,children:[{name:"z",bytes:10},{name:"a",bytes:20}]})`);
  await checkSort();
  $("cleanup-tab").click(); d.querySelector(".remembered-heading button").click();
  await $("forget-all").onclick();
  assert.equal($("remembered-card").hidden,true);
  assert.match($("remembered-paths").textContent,/No remembered choices/);
  assert(!requests.includes("/api/trash"),"No Trash sent");
  assert.deepEqual(errors,[]);
  console.log("PASS: remembered selection, whole-row checkbox, growth warning, normal Trash confirmation, absent choices manager, Forget/Forget all; sortable headers at root, depth 2, category and synthetic views; no errors or Trash actions.");
  w.close();
})().catch(e=>{console.error(e);process.exit(1)});
