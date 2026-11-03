// Fixture-only Round 14 checks. Remembered responses are mocked; never trashes files.
const { chromium } = require("playwright");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const url = process.argv[2];
if (!url) throw Error("Usage: node scripts/ui-round14-smoke.cjs <fixture-ui-url>");
(async () => {
 const browser = await chromium.launch({executablePath:process.env.OMD_CHROME || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",headless:true,args:["--no-sandbox"],env:{...process.env,HOME:process.cwd()+"/.build/chrome-home"}});
 try {
  const page = await browser.newPage({viewport:{width:1280,height:900}}), errors=[];
  page.on("pageerror",e=>errors.push(e.message));
  const fixture=process.cwd()+"/.fixtures/home";
  let items=[{path:fixture+"/Library/Caches/pnpm",bytes:1_200_000_000,kind:"directory",rule:"pnpm",trashed_at:"2026-11-01T10:00:00Z"},{path:fixture+"/node_modules",bytes:1000,kind:"directory",rule:"node-modules",trashed_at:"2026-11-01T10:00:00Z"}];
  const matches=()=>items.map((x,i)=>({...x,current_bytes:i===0?4_800_000_000:1500,current_kind:"directory",growth:i===0?3_600_000_000:500,warning:i===0}));
  await page.route("**/api/remembered",async route=>{
   if(route.request().method()==="POST") {const data=route.request().postDataJSON();items=data.forget_all?[]:items.filter(x=>!data.paths.includes(x.path)); await route.fulfill({json:{ok:true}});}
   else await route.fulfill({json:{items,matches:matches()}});
  });
  await page.route("**/api/trash",()=>{throw Error("Smoke must not send Trash")});
  await page.goto(url); await page.waitForSelector("#overview-folders .fr"); await page.evaluate(()=>document.fonts.ready);
  await page.locator("#cleanup-tab").click();
  await page.locator("#remembered-card").waitFor({state:"visible"});
  assert.match(await page.locator("#remembered-card").innerText(),/Back since last time · 2 items/);
  assert.equal(await page.locator("#remembered-card input:checked").count(),2);
  const warning=page.locator(".growth-warning"); await warning.focus();
  const described=await warning.getAttribute("aria-describedby");
  assert.match(await page.locator("#"+described).innerText(),/was 1.2 GB, now 4.8 GB/);
  await page.getByRole("button",{name:"Clean again",exact:true}).click();
  assert.equal(await page.locator("#remember-choice").isChecked(),true);
  assert.match(await page.locator("#confirm-paths").innerText(),/pnpm/);
  await page.locator("#cancel-trash").click();
  await page.getByRole("button",{name:"Manage remembered",exact:true}).click();
  assert.equal(await page.locator(".remembered-managed").count(),2);
  await page.locator("#remembered-close").click();
  for(const theme of ["dark","light"]) {
   await page.emulateMedia({colorScheme:theme,reducedMotion:"reduce"});
   const colors=await page.evaluate(()=>[".badge.safe",".gain","#remembered-card"].map(s=>getComputedStyle(document.querySelector(s)).backgroundColor));
   assert.equal(new Set(colors).size,1,"Safe badge/banner/card share success tint");
   for(const width of [1440,900,600,390]) {
    await page.setViewportSize({width,height:900});
    await page.evaluate(()=>document.querySelector("#suggestion-list").scrollTop=0);
    const positions=await page.evaluate(()=>{
     const rect=n=>{const r=n.getBoundingClientRect();return {left:r.left,right:r.right};};
     return [...document.querySelectorAll(".suggestion-card")].map(row=>{
      const cells=[row.querySelector(".item-title"),row.querySelector(".badge"),row.querySelector(".suggestion-score"),row.querySelector(".suggestion-size")];
      return cells.map((cell,i)=>{const h=document.querySelectorAll(".cleanup-head th")[i+2];return [rect(h),rect(cell),i];});
     });
    });
    for(const row of positions) for(const [h,c,i] of row) assert.ok(Math.abs((i<2?h.left:h.right)-(i<2?c.left:c.right))<1,"Cleanup header/cell alignment");
    const before=await page.locator("#details-toggle").boundingBox();
    const meta=await page.locator("#scan-status").boundingBox();
    await page.locator("#details-toggle").click();
    assert.deepEqual(await page.locator("#details-toggle").boundingBox(),before,"Counting trigger stays fixed");
    assert.deepEqual(await page.locator("#scan-status").boundingBox(),meta,"Meta stays fixed");
    const panel=await page.locator("#notes").boundingBox(); assert.ok(panel.y+panel.height<=before.y+2,"Panel opens above trigger");
    await page.keyboard.press("Escape");
    assert.equal(await page.locator("#details-toggle").evaluate(n=>n===document.activeElement),true);
   }
   await page.setViewportSize({width:1280,height:900});
   await page.locator(".item-name").first().click();
   await page.screenshot({path:`.build/screens/round14-${theme}-cleanup.png`});
   await page.locator(".item-name").first().click();
  }
  for(const key of ["name","safety","score","size"]) {
   await page.locator(`[data-cleanup-sort="${key}"]`).click();
   assert.equal(await page.locator(".cleanup-head th[aria-sort]:not([aria-sort=none])").count(),1);
  }
  // All explorer headers toggle at the root, depth 2, category summary and category view.
  await page.locator("#folders").click();
  async function checkSort() {
   for(const key of ["name","bytes","files","modified"]) {
    const b=page.locator(`[data-sort="${key}"]`);
    if (!await b.isVisible()) continue;
    await b.click(); await page.waitForTimeout(80);
    const first=await page.locator("#children .filename").allTextContents();
    const direction=await b.locator("..").getAttribute("aria-sort");
    await b.click(); await page.waitForTimeout(80);
    assert.notEqual(await b.locator("..").getAttribute("aria-sort"),direction);
    const second=await page.locator("#children .filename").allTextContents();
    if(key==="name" && first.length>1) assert.notDeepEqual(first,second,"Name visibly reverses order");
    const sorted=await page.evaluate(()=>{
     // Assert rendered rows use the same comparison as the active header.
     const nodes=[...(current.children||[])].sort((a,b)=>{const x=sortKey==="name"?a.name:sortKey==="modified"?+new Date(a.modified):a[sortKey]||0,y=sortKey==="name"?b.name:sortKey==="modified"?+new Date(b.modified):b[sortKey]||0;return (sortAsc?1:-1)*(typeof x==="string"?x.localeCompare(y):x-y)});
     return nodes.slice(0,shown).map(displayName);
    });
    assert.deepEqual(second,sorted);
    assert.equal(await page.locator("#scan-feedback.banner-error:not([hidden])").count(),0,"No sort error banner");
   }
  }
  await checkSort();
  await page.evaluate(async()=>{
   const first=root.children.find(n=>n.kind==="directory" && n.children?.some(c=>c.kind==="directory"));
   await navigate(first);
   const second=current.children.find(n=>n.kind==="directory" && n.children?.length>1);
   if(!second) throw Error("Fixture needs depth-2 folder with children");
   await navigate(second);
  });
  await checkSort();
  await page.locator("#categories").click(); await checkSort();
  await page.locator("#children .filename").first().click(); await page.waitForTimeout(100); await checkSort();
  await page.evaluate(()=>showFeedback("Error: invalid path"));
  await page.getByRole("button",{name:"Dismiss status",exact:true}).click();
  assert.equal(await page.locator("#scan-feedback").isHidden(),true);
  // Same command bar on the pre-scan Agent page, centered and fully in bounds.
  await page.evaluate(()=>{snapshot=undefined;showMode("agent");});
  for(const width of [1440,900,600,390]) {
   await page.setViewportSize({width,height:800});
   const bar=await page.locator(".scan-command-bar").boundingBox();
   const content=await page.locator("#agent-empty").boundingBox();
   assert.ok(bar.x>=content.x-1 && bar.x+bar.width<=content.x+content.width+1);
   assert.ok(Math.abs(bar.x+bar.width/2-content.x-content.width/2)<2,"Agent command bar centered");
   const start=await page.locator("#agent-start").boundingBox();
   assert.ok(start.y>=bar.y+bar.height && Math.abs(start.x+start.width/2-bar.x-bar.width/2)<2,"Secondary action centered below bar");
   for(const s of ["#folder-picker summary","#scan-agent-slot summary","#plain-scan"]) {const b=await page.locator(s).boundingBox();assert.ok(b.x>=bar.x && b.x+b.width<=bar.x+bar.width+1,s+" fits bar");}
  }
  await page.screenshot({path:".build/screens/round14-agent-390.png"});
  await page.reload(); await page.waitForSelector("#overview-folders .fr"); await page.locator("#cleanup-tab").click();
  await page.getByRole("button",{name:"Manage remembered",exact:true}).click();
  await page.locator("#forget-all").click();
  await page.waitForFunction(()=>document.querySelector("#remembered-paths").textContent.includes("No remembered choices"));
  await page.locator("#remembered-close").click();
  assert.equal(await page.locator("#remembered-card").isHidden(),true);
  assert.deepEqual(errors,[]);
  console.log("PASS: remembered card/selection/confirm/forget, growth tooltip, shared success tint, column alignment, stable counting overlay, root/depth-2/category sorting, error close, responsive Agent command bar.");
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});
