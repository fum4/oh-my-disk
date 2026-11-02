// Session flow smoke: mock only native terminal and agent CLIs, use real fixture APIs.
const {JSDOM,VirtualConsole}=require('jsdom');
const fs=require('fs'),assert=require('node:assert/strict');
const url=process.argv[2],token=new URL(url).searchParams.get('token');
const wait=async fn=>{for(let i=0;i<160;i++){if(fn())return;await new Promise(r=>setTimeout(r,25));}throw Error('Timed out: '+fn)};
(async()=>{
 let available=false,launched=[],closed=0;
 const errors=[],state={home:process.cwd()+'/.fixtures/home',busy:false,hasScan:false,revision:0,phase:'',protected:[],entries:0,directories:0,rate:0};
 const vc=new VirtualConsole();vc.on('jsdomError',e=>errors.push(e.message));
 const dom=new JSDOM(fs.readFileSync('internal/web/assets/index.html','utf8').replace(/<script[^>]*><\/script>/g,''),{url,runScripts:'dangerously',pretendToBeVisual:true,virtualConsole:vc,beforeParse(w){
 w.fetch=(path,options={})=>fetch(new URL(path,url),{...options,headers:{...options.headers,Cookie:'omd='+token}});
 w.Path2D=class{arc(){}roundRect(){}};w.HTMLCanvasElement.prototype.getContext=()=>new Proxy({},{get:(o,k)=>o[k]||(()=>{}),set:(o,k,v)=>((o[k]=v),true)});
 w.Terminal=class{constructor(){this.cols=90;this.rows=22;}loadAddon(){}open(){}onData(){}attachCustomKeyEventHandler(){}write(){}focus(){}dispose(){}getSelection(){return ''}};
 w.FitAddon={FitAddon:class{fit(){}}};w.WebLinksAddon={WebLinksAddon:class{}};
 w.go={main:{App:{State:async()=>({...state}),Agents:async()=>[{name:'claude',path:available?'/mock/claude':''},{name:'codex',path:''}],Scan:async()=>{state.busy=true;state.phase='walking';},Cancel:async()=>{state.busy=false;state.phase='Cancelled';},StartAgent:async(...args)=>launched.push(args),ReadAgent:async()=>({data:'',exited:true}),CloseAgent:async()=>{closed++},ResizeAgent:async()=>{},Copy:async()=>{},Paste:async()=>'',PickFolder:async()=>state.home}}};
 }});
 const w=dom.window,$=id=>w.document.getElementById(id);
 w.eval(["app.js","desktop.js","agents.js"].map(file=>fs.readFileSync("internal/web/assets/"+file,"utf8")).join("\n"));
 await wait(()=>$('scan-claude').textContent==='Install Claude');
 assert.equal($('scan-claude').disabled,true);assert.equal($('install-claude').hidden,false);assert.equal($('tip-install-codex').hidden,false);
 $('agent-dismiss').click();assert.equal(w.localStorage.getItem('omd-agent-tip-dismissed'),'1');
 available=true;await $('agent-check').onclick();assert.equal($('scan-claude').disabled,false);assert.equal($('scan-codex').disabled,true);
 await $('scan-claude').onclick();await wait(()=>$('plain-scan').disabled);assert.equal(launched.length,0);
 state.busy=false;state.hasScan=true;state.phase='Complete';state.revision=1;
 await wait(()=>launched.length===1);assert.equal(launched[0][0],'claude');assert.equal($('agent-terminal').hidden,false);
 await wait(()=>$('terminal-status').textContent==='Session ended');await $('terminal-close').onclick();assert.equal($('agent-terminal').hidden,true);
 await $('ask-claude').onclick();assert.equal(launched.length,2);await $('terminal-close').onclick();
 // A cancelled scan cannot hand off to an agent.
 await $('scan-claude').onclick();$('cancel-scan').onclick();state.phase='Complete';state.revision=2;
 await new Promise(r=>setTimeout(r,900));assert.equal(launched.length,2);assert.ok(closed>=2);
 assert.deepEqual(errors,[]);w.close();console.log('PASS: install states, check again, dismissal persistence, scan-to-agent handoff, terminal close/reopen, cancelled scan never launches. Native terminal mocked.');
})().catch(e=>{console.error(e);process.exit(1)});
