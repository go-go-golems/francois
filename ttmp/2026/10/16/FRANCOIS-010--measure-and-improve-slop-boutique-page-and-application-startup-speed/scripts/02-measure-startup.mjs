// Starts ONE bounded public Deck tour, then reloads it. No model calls.
// Records only paths/timings/cache metadata, never headers containing grants.
import {createRequire} from 'node:module';
import {writeFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const base='https://slop.boutique';
const label=process.argv[2]||'before';
if(!/^[a-z-]+$/.test(label))throw new Error('Invalid receipt label');
const result={at:new Date().toISOString(),base,artifact:'lattice-deck',readiness:'Hosted frame has visible text and a nonempty canvas; two animation frames elapsed',runs:[]};
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try{
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  const page=await context.newPage();
  let requests=[],inflight=new Set();
  page.on('response',async r=>{
    const pending=(async()=>{
      const url=new URL(r.url());
      const headers=await r.allHeaders();
      await r.finished().catch(()=>{});
      const size=await r.request().sizes().catch(()=>({}));
      requests.push({origin:url.origin,path:url.pathname,status:r.status(),timing:r.request().timing(),bodyBytes:size.responseBodySize,
        cacheControl:headers['cache-control'],encoding:headers['content-encoding'],etag:headers.etag});
    })();
    inflight.add(pending);pending.finally(()=>inflight.delete(pending));
  });
  await page.goto(base+'/apps/lattice-deck');
  for(const pass of ['cold-launch','warm-reload']){
    requests=[];
    const start=performance.now();
    if(pass==='cold-launch'){
      const demoResponse=page.waitForResponse(r=>r.url()===base+'/api/demos'&&r.request().method()==='POST');
      await page.getByRole('button',{name:'Try',exact:true}).click();
      const demo=await demoResponse;
      if(!demo.ok()){
        result.blocked={status:demo.status(),error:(await demo.json()).error};
        console.log('Tour start declined',JSON.stringify(result.blocked));
        process.exitCode=2;break;
      }
      await page.waitForURL(/\/instances\/ins_/);
    }else await page.reload({waitUntil:'domcontentloaded'});
    await page.locator('iframe').waitFor();
    const frame=await(await page.locator('iframe').elementHandle()).contentFrame();
    await frame.waitForFunction(()=>document.querySelectorAll('canvas').length>0&&document.body.innerText.length>100,{},{timeout:45000});
    await frame.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    const readyMs=performance.now()-start;
    // Allow finite resource completions to settle; SSE requests remain excluded.
    await page.waitForTimeout(1200);
    const snapshot=await frame.evaluate(()=>({title:document.title,canvases:document.querySelectorAll('canvas').length,
      resources:performance.getEntriesByType('resource').filter(e=>e.initiatorType!=='fetch').map(e=>({origin:new URL(e.name).origin,path:new URL(e.name).pathname,ms:e.duration,transferBytes:e.transferSize,decodedBytes:e.decodedBodySize}))}));
    result.runs.push({pass,readyMs,...snapshot,requests:[...requests]});
    console.log(pass,'canvas-ready ms',Math.round(readyMs),'finite body bytes',requests.reduce((n,r)=>n+(r.bodyBytes||0),0));
  }
}catch(error){result.error=String(error);process.exitCode=1;}
finally{
  await browser.close();
  writeFileSync(path.join(ticket,`various/startup-${label}.json`),JSON.stringify(result,null,2)+'\n');
}
