// Read-only cold/warm browser measurements. No credentials or launch/model calls.
import {createRequire} from 'node:module';
import {writeFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const base=process.argv[2]||'https://slop.boutique';
const label=process.argv[3]||'before';
if(!/^[a-z-]+$/.test(label))throw new Error('Invalid receipt label');
const result={at:new Date().toISOString(),base,visits:[]};
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try{
  for(const route of ['/apps','/apps/lattice-deck','/account']){
    const context=await browser.newContext({viewport:{width:1440,height:1000}});
    const page=await context.newPage();
    const cdp=await context.newCDPSession(page);
    await cdp.send('Network.enable');
    let requests=new Map();
    cdp.on('Network.responseReceived',({requestId,response:r})=>{
      const u=new URL(r.url);if(u.origin!==base)return;
      const headers={};
      for(const [k,v] of Object.entries(r.headers))if(['cache-control','etag','last-modified','content-encoding','content-length','vary'].includes(k.toLowerCase()))headers[k.toLowerCase()]=v;
      requests.set(requestId,{path:u.pathname,status:r.status,diskCache:!!r.fromDiskCache,headers});
    });
    cdp.on('Network.loadingFinished',({requestId,encodedDataLength})=>{if(requests.has(requestId))requests.get(requestId).wireBytes=encodedDataLength;});
    for(const pass of ['cold','warm']){
      requests=new Map();
      await page.goto(base+route,{waitUntil:'load'});
      await page.waitForTimeout(1200);
      const timing=await page.evaluate(()=>({
        navigation:performance.getEntriesByType('navigation').map(e=>({ttfbMs:e.responseStart-e.requestStart,loadMs:e.loadEventEnd,startMs:e.startTime,transferBytes:e.transferSize})),
        paint:performance.getEntriesByType('paint').map(e=>({name:e.name,ms:e.startTime})),
        resources:performance.getEntriesByType('resource').filter(e=>new URL(e.name).origin===location.origin).map(e=>({path:new URL(e.name).pathname,ms:e.duration,transferBytes:e.transferSize,decodedBytes:e.decodedBodySize}))
      }));
      const visit={route,pass,...timing,responses:[...requests.values()]};
      result.visits.push(visit);
      console.log(route,pass,JSON.stringify(timing.navigation), 'wireBytes',visit.responses.reduce((n,r)=>n+(r.wireBytes||0),0));
    }
    await context.close();
  }
}finally{await browser.close();}
writeFileSync(path.join(ticket,`various/cache-${label}.json`),JSON.stringify(result,null,2)+'\n');
