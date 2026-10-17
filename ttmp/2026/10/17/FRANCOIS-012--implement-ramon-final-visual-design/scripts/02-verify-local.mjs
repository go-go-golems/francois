// Local-only integration/evidence run. It uses a short-lived local admin
// session, publishes the tracked Markdown through Studio, and revokes that
// session at the end. It never contacts production or calls a model.
import { createRequire } from 'node:module';
import { randomBytes, createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
const require = createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const { chromium } = require('playwright-core');
const root = fileURLToPath(new URL('../../../../../../', import.meta.url));
const ticket = fileURLToPath(new URL('../', import.meta.url));
const base = 'http://localhost:18792';
const db = 'postgres://francois:francois@127.0.0.1:5435/francois_catalog?sslmode=disable';
const sql = (q) => execFileSync('psql', [db, '-At', '-v', 'ON_ERROR_STOP=1', '-c', q], {encoding:'utf8'}).trim();
const secret = randomBytes(32).toString('hex'), csrf = randomBytes(24).toString('hex');
const digest = createHash('sha256').update(secret).digest('hex');
const out = ticket + 'sources/implementation/';
await mkdir(out, {recursive:true});
sql(`INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at,id_token) SELECT decode('${digest}','hex'),id,'${csrf}',now()+interval '1 hour',now()+interval '1 hour','' FROM users WHERE role='admin' AND NOT disabled ORDER BY created_at LIMIT 1`);
const browser = await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true,args:['--host-resolver-rules=MAP *.localhost 127.0.0.1','--use-angle=swiftshader','--enable-unsafe-swiftshader']});
let originalSettings;
const result = {base, localDemoLeftEnabled:process.argv.includes('--keep-demo-enabled'), at:new Date().toISOString(),pages:[],checks:[]};
try {
 const admin = await browser.newContext({viewport:{width:1440,height:1000}});
 await admin.addCookies([{name:'francois_session_dev',value:secret,url:base,httpOnly:true,sameSite:'Lax'}]);
 const headers = {'X-CSRF-Token':csrf, Origin:base};
 const settingsResponse = await admin.request.get(base+'/api/admin/settings');assert.equal(settingsResponse.status(),200);
 const settings = (await settingsResponse.json()).active;
 if (!settings.document.demo.enabled) {
  const updated = structuredClone(settings.document);updated.demo.enabled = true;
  const applied = await admin.request.patch(base+'/api/admin/settings',{headers:{...headers,'If-Match':String(settings.revision)},data:updated});assert.equal(applied.status(),200,await applied.text());originalSettings=settings.document;
 }
 const imported=root+'ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/';
 const showcase=JSON.parse(await readFile(imported+'sources/showcase-web.json','utf8'));
 const apps = ['lattice-deck','tabula-workbench','rag-workbench','lattice-rift','lattice-phantasmagoria'];
 for (const id of apps) {
  const r=await admin.request.get(base+'/api/studio/artifacts/'+id); assert.equal(r.status(),200);
  const current=await r.json(); assert.ok(!current.artifact.pageDraft,'Preserve existing unpublished drafts');
  const body=await readFile(root+'examples/imports/'+id+'/page.md','utf8');
  const categories={'lattice-deck':['Graphics'],'tabula-workbench':['Tools'],'rag-workbench':['Tools','Agents'],'lattice-rift':['Toys','Graphics'],'lattice-phantasmagoria':['Toys','Graphics']};
  const description=body.split('\n\n')[1].split('. ')[0].replace(/\.$/,'')+'.';
  const patched=await admin.request.patch(base+'/api/studio/artifacts/'+id,{headers,data:{demoEnabled:true,description,tags:current.artifact.tags.length?current.artifact.tags:categories[id]}});assert.equal(patched.status(),200,await patched.text());
  const latest=current.releases.filter(r=>r.state==='published').sort((a,b)=>b.publishedAt.localeCompare(a.publishedAt))[0];
  if (!latest.freeSample) {
   const existing=current.releases.find(r=>r.version==='0.1.2');
   if(!existing){
    const uploaded=await admin.request.post(base+'/api/studio/releases',{headers,multipart:{artifactId:id,version:'0.1.2',freeSample:'true',manifestJson:await readFile(root+'examples/imports/'+id+'/manifest.json','utf8'),html:{name:id+'.html',mimeType:'text/html',buffer:await readFile(root+'examples/imports/'+id+'/release.html')}}});assert.equal(uploaded.status(),201,await uploaded.text());
    const rel=await uploaded.json();const pub=await admin.request.post(base+'/api/studio/releases/'+rel.id+'/publish',{headers});assert.equal(pub.status(),200,await pub.text());
   } else {assert.equal(existing.freeSample,true,'Do not rewrite an existing private release');}
  }
  if (!current.artifact.page.cover) {
   const images=showcase.artifacts.find(a=>a.id===id).images;
   const media=[];
   for(const image of images){
    const uploaded=await admin.request.post(base+'/api/studio/media',{headers,multipart:{alt:image.alt,file:{name:image.file.split('/').pop(),mimeType:'image/webp',buffer:await readFile(imported+'various/showcase/'+image.file)}}});assert.equal(uploaded.status(),201,await uploaded.text());media.push((await uploaded.json()).id);
   }
   current.artifact.page.cover=media[0];current.artifact.page.gallery=media.slice(1);
  }

  const saved=await admin.request.put(base+'/api/studio/artifacts/'+id+'/draft',{headers,data:{...current.artifact.page,body}});assert.equal(saved.status(),200,await saved.text());
  const published=await admin.request.post(base+'/api/studio/artifacts/'+id+'/draft/publish',{headers});assert.equal(published.status(),200);
 }
 result.checks.push('Published five Markdown pages through local Studio; covers/gallery preserved');
 const ap=await admin.newPage();
 for (const route of ['/account','/studio','/admin']) {
  await ap.goto(base+route,{waitUntil:'networkidle'});
  assert.equal(await ap.locator('.site-header').count(),1);
  assert.equal(await ap.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,route);
  await ap.screenshot({path:out+'staff-'+route.slice(1)+'.png',fullPage:true});
 }
 await admin.close();
 for (const width of [1440,900,390]) {
  const c=await browser.newContext({viewport:{width,height:1000}});const p=await c.newPage();let creations=0;
  p.on('request',r=>{if(r.method()==='POST'&&/\/api\/(demos|instances\/open)$/.test(new URL(r.url()).pathname))creations++;});
  for (const [name,route] of [['archive','/'],['experiment','/apps/lattice-deck'],['support','/pricing'],['account-signed-out','/account']]) {
   await p.goto(base+route,{waitUntil:'networkidle'});await p.evaluate(()=>document.fonts.ready);
   assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,name+' '+width);
   if(name==='experiment'){assert.equal(await p.getByRole('link',{name:'Get the file'}).count(),1);assert.equal(await p.locator('.editorial-label').filter({hasText:'What it is'}).count(),1);}
   result.pages.push({name,width,title:await p.title()});
   await p.screenshot({path:out+name+'-'+width+'.png',fullPage:true});
  }
  await p.goto(base+'/',{waitUntil:'networkidle'});
  const initial=await p.locator('.archive-row').evaluateAll(rows=>Object.fromEntries(rows.map(r=>[r.getAttribute('href'),r.querySelector('.archive-number').textContent])));
  if(width===1440){await p.locator('.archive-row').filter({hasText:'Lattice Deck'}).hover();await p.waitForSelector('[data-archive-preview]:not([hidden])');await p.waitForFunction(()=>document.querySelector('[data-archive-preview] img')?.complete);await p.screenshot({path:out+'archive-hover-1440.png',fullPage:true});}
  await p.getByRole('searchbox').fill('lattice');await p.getByRole('button',{name:'Search and sort experiments'}).click();await p.waitForLoadState('networkidle');
  for(const row of await p.locator('.archive-row').all()){assert.match(await row.innerText(),/Lattice/i);assert.equal(await row.locator('.archive-number').textContent(),initial[await row.getAttribute('href')]);}
  await Promise.all([p.waitForURL(u=>u.searchParams.get('sort')==='oldest'),p.locator('#archive-sort').selectOption('oldest')]);
  assert.equal(creations,0,'Browsing/search/hover must not allocate visits');
  result.checks.push(`Layout, search, ordinal identity, sort and zero visit allocation at ${width}px`);
  await c.close();
 }
 const nojs=await browser.newContext({javaScriptEnabled:false});const n=await nojs.newPage();await n.goto(base+'/?q=lattice&sort=oldest');assert.ok(await n.locator('.archive-row').count()>0);await nojs.close();result.checks.push('Search renders without JavaScript');
 const paused=await browser.newContext();const pausedPage=await paused.newPage();
 await pausedPage.route('**/api/demos',route=>route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'visits_paused',message:'Visits are temporarily paused.'}})}));
 await pausedPage.goto(base+'/apps/lattice-deck',{waitUntil:'networkidle'});await pausedPage.getByRole('button',{name:'Start experiment'}).click();await pausedPage.getByText('Visits are temporarily paused.').waitFor();assert.equal(await pausedPage.locator('[data-runner-surface] iframe').count(),0);await pausedPage.screenshot({path:out+'experiment-paused-1440.png',fullPage:true});await paused.close();result.checks.push('Mocked paused-start 503 renders service message and no iframe; backend operation switches covered by Go integration tests');
 const c=await browser.newContext({viewport:{width:1440,height:1000}});const p=await c.newPage();let visits=0, launches=0;
 p.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname==='/api/demos')visits++;if(r.method()==='POST'&&/\/api\/instances\/[^/]+\/launch$/.test(new URL(r.url()).pathname))launches++;});
 await p.goto(base+'/apps/lattice-deck',{waitUntil:'networkidle'});assert.equal(visits,0);
 await p.getByRole('button',{name:'Start experiment'}).click();
 await p.waitForFunction(()=>document.querySelector('[data-runner-state]')?.textContent==='Live',{},{timeout:30000});
 assert.equal(visits,1);
 const app=await (await p.locator('[data-runner-surface] iframe').elementHandle()).contentFrame();
 await app.waitForFunction(()=>document.body?.innerText.trim().length>100,{},{timeout:20000});
 result.checks.push('Hosted application DOM rendered after the launch handshake');
 const initialSrc=await p.locator('[data-runner-surface] iframe').getAttribute('src');
 await p.screenshot({path:out+'experiment-live-1440.png',fullPage:true});
 const before=launches;await p.getByRole('button',{name:'Reload',exact:true}).click();await p.waitForFunction(()=>document.querySelector('[data-runner-state]')?.textContent==='Live',{},{timeout:30000});
 assert.equal(visits,1);assert.ok(launches>before);assert.equal(await p.locator('[data-runner-surface] iframe').getAttribute('src'),initialSrc);
 await p.getByRole('button',{name:'Full screen',exact:true}).click();await p.waitForFunction(()=>!!document.fullscreenElement);await p.evaluate(()=>document.exitFullscreen());assert.equal(visits,1);
 result.checks.push('Actual guest startup, new guest CSRF refresh, same-instance reload and fullscreen passed');
 // Expire only this visit via the local test database, then verify denial on reload.
 const instanceId=(await p.locator('[data-runner-link]').getAttribute('href')).split('/').pop();assert.match(instanceId,/^ins_[a-f0-9]+$/);
 sql(`UPDATE instances SET expires_at=now()-interval '1 minute' WHERE id='${instanceId}' AND kind='demo'`);
 await p.getByRole('button',{name:'Reload',exact:true}).click();await p.waitForFunction(()=>document.querySelector('[data-runner-state]')?.textContent==='Unavailable',{},{timeout:30000});
 assert.equal(visits,1);await p.screenshot({path:out+'experiment-expired-1440.png',fullPage:true});result.checks.push('Expired visit shows server denial; reload does not allocate a replacement');
 await c.close();
 await writeFile(out+'browser-validation.json',JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result,null,2));
} finally {
 if(originalSettings && !process.argv.includes('--keep-demo-enabled')){
  const cleanup=await browser.newContext();await cleanup.addCookies([{name:'francois_session_dev',value:secret,url:base,httpOnly:true,sameSite:'Lax'}]);
  const current=(await (await cleanup.request.get(base+'/api/admin/settings')).json()).active;
  const restoredDocument=structuredClone(current.document);restoredDocument.demo.enabled=originalSettings.demo.enabled;
  const restored=await cleanup.request.patch(base+'/api/admin/settings',{headers:{'X-CSRF-Token':csrf,Origin:base,'If-Match':String(current.revision)},data:restoredDocument});assert.equal(restored.status(),200,await restored.text());await cleanup.close();
 }
 await browser.close();
 sql(`UPDATE sessions SET revoked_at=now() WHERE token_hash=decode('${digest}','hex')`);
}
