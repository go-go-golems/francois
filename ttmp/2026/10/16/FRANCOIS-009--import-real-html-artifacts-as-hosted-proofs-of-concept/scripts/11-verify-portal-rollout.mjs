// Fresh public-browser verification of the deployed page fixes. No credentials.
import {createRequire} from 'node:module';
import {readFileSync,writeFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const{chromium}=require('playwright-core');
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const plan=JSON.parse(readFileSync(path.join(ticket,'sources/showcase.json'),'utf8'));
const base='https://slop.boutique';
const result={base,at:new Date().toISOString(),pages:[],instances:[]};
const save=()=>writeFileSync(path.join(ticket,'various/portal-rollout-verification.json'),JSON.stringify(result,null,2)+'\n');
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try{
  const context=await browser.newContext({viewport:{width:1600,height:1100}});
  const page=await context.newPage();
  for(const app of plan.artifacts){
    const response=await page.goto(base+'/apps/'+app.id);assert.equal(response.status(),200);
    await page.evaluate(async()=>{await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));});
    assert.equal(await page.getByRole('link',{name:'Download HTML',exact:true}).count(),1);
    assert.equal(await page.getByRole('link',{name:'Manifest',exact:true}).count(),0);
    const images=await page.locator('main img').evaluateAll(es=>es.map(e=>({alt:e.alt,loaded:e.complete&&e.naturalWidth>0})));
    assert.equal(images.length,app.images.length);assert.ok(images.every(i=>i.loaded));
    result.pages.push({id:app.id,title:await page.title(),htmlDownloads:1,manifestLinks:0,imageCount:images.length});save();
    if(app.id==='lattice-deck')await page.screenshot({path:path.join(ticket,'various/showcase/deck-public-gallery.png'),fullPage:true});
  }
  // Two starts are within the existing per-user/address limit; no model calls.
  for(const id of ['lattice-deck','rag-workbench']){
    await page.goto(base+'/apps/'+id);const expected=await page.title();
    await page.getByRole('button',{name:'Try',exact:true}).click();await page.waitForURL(/\/instances\/ins_/);
    await page.waitForFunction(title=>document.title===title,expected);
    const detail=await page.evaluate(async()=>await(await fetch('/api'+location.pathname)).json());
    const lifetime=new Date(detail.expiresAt)-new Date(detail.createdAt);
    assert.ok(lifetime>0&&lifetime<=20*60*1000+1000);
    result.instances.push({artifact:id,path:new URL(page.url()).pathname,pageTitle:expected,instanceTitle:await page.title(),artifactTitle:detail.artifactTitle,lifetimeMs:lifetime});save();
    await page.locator('iframe').waitFor();await page.waitForTimeout(1500);
    await page.screenshot({path:path.join(ticket,'various/showcase',id+'-instance-title.png'),fullPage:true});
    await page.getByRole('link',{name:'Close application',exact:true}).click();
    await page.waitForURL(base+'/');assert.notEqual(await page.title(),expected);
    console.log('Instance title matches public page',id,expected);
  }
  await page.goto(base+'/apps');
  await page.evaluate(async()=>{await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));});
  await page.screenshot({path:path.join(ticket,'various/showcase/catalog-public-covers.png'),fullPage:true});
  console.log(JSON.stringify(result));
}finally{await browser.close();}
