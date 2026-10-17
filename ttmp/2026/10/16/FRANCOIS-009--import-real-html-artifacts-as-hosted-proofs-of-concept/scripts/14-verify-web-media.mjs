// Verify actual published delivery bytes without credentials or new tours.
import {createRequire} from 'node:module';
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const plan=JSON.parse(readFileSync(path.join(ticket,'sources/showcase-web.json'),'utf8'));
const base='https://slop.boutique';
const receipt={at:new Date().toISOString(),base,artifacts:[]};
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try{
  const context=await browser.newContext({viewport:{width:1600,height:1100}});
  const page=await context.newPage();
  for(const app of plan.artifacts){
    assert.equal((await page.goto(base+'/apps/'+app.id)).status(),200);
    await page.evaluate(async()=>{await document.fonts.ready;await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));});
    const images=await page.locator('main img').evaluateAll(es=>es.map(e=>({path:new URL(e.src).pathname,alt:e.alt,loaded:e.complete&&e.naturalWidth>0})));
    assert.equal(images.length,app.images.length);
    for(let i=0;i<images.length;i++){
      const image=images[i];assert.ok(image.loaded);assert.equal(image.alt,app.images[i].alt);
      const response=await context.request.get(base+image.path);assert.equal(response.status(),200);
      assert.equal(response.headers()['content-type'],'image/webp');
      assert.match(response.headers()['cache-control'],/immutable/);
      const data=await response.body();
      assert.equal(createHash('sha256').update(data).digest('hex'),app.images[i].sha256);
      image.bytes=data.length;image.etag=response.headers().etag;
      const conditional=await context.request.get(base+image.path,{headers:{'If-None-Match':image.etag}});
      assert.equal(conditional.status(),304);assert.equal((await conditional.body()).length,0);
    }
    assert.equal(await page.getByRole('link',{name:'Download HTML',exact:true}).count(),1);
    receipt.artifacts.push({id:app.id,images});
    if(app.id==='lattice-deck')await page.screenshot({path:path.join(ticket,'various/showcase/deck-public-gallery.png'),fullPage:true});
    console.log('Verified WebP gallery',app.id,images.reduce((n,i)=>n+i.bytes,0));
  }
  await page.goto(base+'/apps');
  await page.evaluate(async()=>{await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));});
  await page.screenshot({path:path.join(ticket,'various/showcase/catalog-public-covers.png'),fullPage:true});
}finally{await browser.close();}
writeFileSync(path.join(ticket,'various/web-media-verification.json'),JSON.stringify(receipt,null,2)+'\n');
