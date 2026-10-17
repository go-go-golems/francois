// Public visitor checks. No token or administrator session is loaded.
import {createRequire} from 'node:module';
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const{chromium}=require('playwright-core');
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const plan=JSON.parse(readFileSync(path.join(ticket,'sources/showcase.json'),'utf8'));
const base='https://slop.boutique';
const receipt={base,at:new Date().toISOString(),artifacts:[]};
const save=()=>writeFileSync(path.join(ticket,'various/public-gallery-verification.json'),JSON.stringify(receipt,null,2)+'\n');
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try{
  const context=await browser.newContext({viewport:{width:1600,height:1100}});
  const page=await context.newPage();
  await page.goto(base+'/apps');
  await page.evaluate(async()=>{await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));});
  await page.screenshot({path:path.join(ticket,'various/showcase/catalog-public-covers.png'),fullPage:true});
  for(const app of plan.artifacts){
    const r=await page.goto(base+'/apps/'+app.id);assert.equal(r.status(),200);
    await page.evaluate(async()=>{await document.fonts.ready;await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));});
    const images=await page.locator('main img').evaluateAll(es=>es.map(e=>({alt:e.alt,loaded:e.complete&&e.naturalWidth>0})));
    assert.equal(images.length,app.images.length);assert.ok(images.every(i=>i.loaded));
    assert.equal(await page.getByRole('button',{name:'Try',exact:true}).count(),1);
    const download=page.getByRole('link',{name:'Download HTML',exact:true});assert.equal(await download.count(),1);
    const href=await download.getAttribute('href');
    const file=await context.request.get(base+href);assert.equal(file.status(),200);
    const sha256=createHash('sha256').update(await file.body()).digest('hex');assert.equal(sha256,app.releaseHTMLSHA256);
    receipt.artifacts.push({id:app.id,images,tryAvailable:true,anonymousDownload:true,htmlSHA256:sha256});save();
    if(app.id==='lattice-deck')await page.screenshot({path:path.join(ticket,'various/showcase/deck-public-gallery.png'),fullPage:true});
    console.log('Public page verified',app.id,images.length,'images');
  }
  // One real anonymous launch respects the three-starts-per-hour limit.
  await page.goto(base+'/apps/rag-workbench');await page.getByRole('button',{name:'Try',exact:true}).click();
  await page.waitForURL(/\/instances\/ins_/);await page.locator('iframe').waitFor();
  const frame=await(await page.locator('iframe').elementHandle()).contentFrame();
  await frame.waitForFunction(()=>window.__wb?.bundled.length===5&&!!window.__wb?.caps.sample);
  const state=await frame.evaluate(async()=>{
    const w=window.__wb;
    const r=await w.caps.sample([{role:'user',content:'Reply with exactly: pong'}],{modelTier:'quick',cache:false,signal:AbortSignal.timeout(45000)});
    return{bundledFiles:w.bundled.length,uploadedFiles:w.uploaded.length,reply:r.text,model:r.modelUsed??r.modelApplied};
  });
  assert.equal(state.uploadedFiles,0);assert.match(state.reply,/pong/i);
  receipt.tour={url:page.url(),...state};save();
  await page.screenshot({path:path.join(ticket,'various/showcase/rag-anonymous-tour.png'),fullPage:true});
  console.log('Anonymous RAG tour and model verified',JSON.stringify(state));
}finally{await browser.close();}
