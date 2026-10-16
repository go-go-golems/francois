// Operator script: normal Vault-backed login, in-memory session, no cookie export.
// Run from repository root. Existing releases are checked, never overwritten.
import {createRequire} from 'node:module';
import {readFileSync,writeFileSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import {createHash} from 'node:crypto';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const base='https://slop.boutique';
const apps=[
  ['lattice-rift','Lattice Rift','An interactive WebGL instrument for melting, tearing and mending a lattice. Browser-only proof of concept.'],
  ['lattice-phantasmagoria','Lattice Phantasmagoria','A playable dual-screen canvas and WebGL experiment with cartridges and scenes. Browser-only proof of concept.'],
  ['tabula-workbench','Tabula Workbench','A programmable reactive workbook with live inputs, charts and model-assisted editing. Workbook drafts live in this browser.'],
  ['lattice-deck','Lattice Deck','A programmable presentation and live graphics workbench with scripted agents, hosted model access and collaboration rooms.'],
  ['rag-workbench','RAG Workbench','An editable retrieval pipeline with public sample documentation, Markdown uploads, lexical search and hosted model access.']
];
const receipt={base,version:'0.1.0',at:new Date().toISOString(),releases:[],checks:[]};
const save=()=>writeFileSync(path.join(ticket,'various/production-receipt.json'),JSON.stringify(receipt,null,2)+'\n');
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try {
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  const page=await context.newPage();
  // Credentials never enter command arguments, logs, receipt or storageState.
  let creds;
  try {creds=JSON.parse(execFileSync('vault',['kv','get','-address=https://vault.yolo.scapegoat.dev','-format=json','kv/apps/francois/prod/e2e-users'],{stdio:['ignore','pipe','ignore']})).data.data;}
  catch {throw new Error('Vault admin credential lookup failed');}
  try {
    await page.goto(base+'/auth/login?return=/account');
    await page.waitForURL(/zitadel\.yolo\.scapegoat\.dev/);
    await page.getByRole('textbox',{name:'Username or email'}).fill(creds.admin_username);
    await page.getByRole('button',{name:'Continue',exact:true}).click();
    await page.locator('input[type=password]').fill(creds.admin_password);
    await page.getByRole('button',{name:'Continue',exact:true}).click();
    for(let i=0;i<3&&!page.url().startsWith(base);i++) {
      const skip=page.getByRole('button',{name:/skip/i});
      await Promise.race([page.waitForURL(u=>u.href.startsWith(base),{timeout:15000}).catch(()=>{}),skip.waitFor({timeout:15000}).catch(()=>{})]);
      if(!page.url().startsWith(base)&&await skip.isVisible().catch(()=>false))await skip.click();
    }
    await page.waitForURL(u=>u.href.startsWith(base),{timeout:30000});
  }catch {throw new Error('Admin browser sign-in failed; credentials omitted');}
  creds=null;
  const s=await page.evaluate(async()=>await(await fetch('/api/session')).json());
  if(!s.authenticated||s.user?.role!=='admin')throw new Error('Expected authenticated administrator');
  const headers={'Origin':base,'X-CSRF-Token':s.csrfToken};
  async function api(method,url,data) {
    const r=await context.request.fetch(base+url,{method,headers,data});
    if(!r.ok())throw new Error(method+' '+url+' HTTP '+r.status()+': '+(await r.text()).slice(0,300));
    return await r.json();
  }
  for(const [id,title,description] of apps) {
    const version=id==='lattice-rift'?'0.1.1':receipt.version;
    const dir=path.resolve('examples/imports',id);
    const multipart={artifactId:id,title,version,description,tags:'proof-of-concept,'+(id.startsWith('lattice-')?'graphics':'workbench'),manifestJson:readFileSync(path.join(dir,'manifest.json'),'utf8'),html:{name:'index.html',mimeType:'text/html',buffer:readFileSync(path.join(dir,'release.html'))}};
    const check=await context.request.post(base+'/api/studio/releases/check',{headers,multipart});
    if(!check.ok())throw new Error(id+' validation HTTP '+check.status());
    receipt.checks.push({id,validation:await check.json()});save();
    const listed=await context.request.get(base+'/api/studio/releases?artifact='+id,{headers});
    if(!listed.ok()&&listed.status()!==404)throw new Error(id+' release listing HTTP '+listed.status());
    const existing=listed.status()===404?[]:await listed.json();
    const rows=Array.isArray(existing)?existing:existing.releases??existing.items??[];
    let release=rows.find(r=>r.version===version);
    if(!release) {
      const r=await context.request.post(base+'/api/studio/releases',{headers,multipart});
      if(!r.ok())throw new Error(id+' upload HTTP '+r.status()+': '+(await r.text()).slice(0,300));
      release=await r.json();
    }
    const digest=createHash('sha256').update(multipart.html.buffer).digest('hex');
    if(release.files?.find(f=>f.role==='html')?.sha256!==digest)throw new Error(id+' stored HTML differs; choose a new version');
    // Idempotent publish; publishing an existing draft uses the exact stored bytes.
    if(release.state!=='published')await api('POST','/api/studio/releases/'+release.id+'/publish');
    await api('PATCH','/api/studio/artifacts/'+id,{description,tags:['proof-of-concept',id.startsWith('lattice-')?'graphics':'workbench'],visibility:'listed'});
    await api('PUT','/api/studio/artifacts/'+id+'/draft',{body:'## '+title+'\n\n'+description+'\n\nThis is a working imported proof of concept. The original visual design is preserved. '+(id==='rag-workbench'?'The bundled corpus contains five public Francois guides; the original private dataset is excluded. ':'')+'\n\n'+(id==='tabula-workbench'?'Workbook draft state is browser-local. ':'')+(id==='lattice-rift'||id==='lattice-phantasmagoria'?'This app runs in your browser without model calls.':'Model calls use the shared hosting limits. Start with the default example; advanced workflows may require additional configuration.')+'\n',cover:''});
    await api('POST','/api/studio/artifacts/'+id+'/draft/publish');
    receipt.releases.push({id,releaseId:release.id,version,sha256:digest,url:base+'/apps/'+id});save();
    console.log('Published',id,release.id);
  }
  const settings=await api('GET','/api/admin/settings');
  receipt.access={demoEnabled:settings.active.document.demo.enabled,demo:settings.active.document.demo,revision:settings.active.revision};save();
  // Reuse this authenticated context for browser checks, without persisting it.
  const {verify}=await import('./05-verify-production.mjs');
  receipt.browser=await verify(context,base,ticket,receipt.releases);save();
} finally {await browser.close();}
