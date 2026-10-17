// Public catalog evidence: no credentials, cookies or authenticated state.
import {createRequire} from 'node:module';
import {writeFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const base='https://slop.boutique';
const ids=['lattice-rift','lattice-phantasmagoria','tabula-workbench','lattice-deck','rag-workbench'];
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try {
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  const catalog=await context.request.get(base+'/api/catalog');assert.equal(catalog.status(),200);
  const json=await catalog.json();
  const feed=await context.request.get(base+'/apps/feed.xml');assert.equal(feed.status(),200);
  const xml=await feed.text();
  for(const id of ids){assert.ok(JSON.stringify(json).includes(id),id+' in public catalog');assert.ok(xml.includes('/apps/'+id),id+' in RSS');}
  const page=await context.newPage();
  await page.goto(base+'/apps');
  await page.screenshot({path:path.join(ticket,'various/screenshots/catalog-production.png'),fullPage:true});
  const pages=[];
  for(const id of ids){
    const response=await page.goto(base+'/apps/'+id);assert.equal(response.status(),200);
    pages.push({id,status:response.status(),title:await page.title(),tourButtons:await page.getByRole('button',{name:/try|tour/i}).allTextContents()});
    if(id==='rag-workbench')await page.screenshot({path:path.join(ticket,'various/screenshots/rag-workbench-public-page.png'),fullPage:true});
  }
  const result={at:new Date().toISOString(),base,catalogStatus:catalog.status(),feedStatus:feed.status(),allFiveInCatalogAndRSS:true,pages};
  writeFileSync(path.join(ticket,'various/public-receipt.json'),JSON.stringify(result,null,2)+'\n');
  console.log(JSON.stringify(result));
}finally{await browser.close();}
