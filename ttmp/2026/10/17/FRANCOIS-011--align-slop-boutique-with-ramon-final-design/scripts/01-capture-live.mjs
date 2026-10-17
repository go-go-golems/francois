import { createRequire } from 'node:module';
import { mkdir, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
const require = createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const { chromium } = require('playwright-core');
const out = fileURLToPath(new URL('../sources/', import.meta.url));
await mkdir(out, { recursive: true });
const browser = await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome', headless:true});
const results = [];
try {
  for (const width of [1440, 390]) {
    const context = await browser.newContext({viewport:{width,height:1000}, deviceScaleFactor:1});
    const page = await context.newPage();
    for (const [name,path] of [['archive','/'],['experiment','/apps/lattice-deck'],['support','/pricing'],['account-signed-out','/account']]) {
      const response = await page.goto('https://slop.boutique'+path,{waitUntil:'networkidle'});
      await page.evaluate(()=>document.fonts.ready);
      await page.screenshot({path:out+'live-'+name+'-'+width+'.png',fullPage:true});
      results.push({name,width,url:page.url(),status:response.status(),...await page.evaluate(()=>({
        title:document.title, text:document.body.innerText, overflow:document.documentElement.scrollWidth>innerWidth,
        headings:[...document.querySelectorAll('h1,h2')].map(e=>({text:e.textContent,fontSize:getComputedStyle(e).fontSize,lineHeight:getComputedStyle(e).lineHeight})),
        links:[...document.querySelectorAll('header a,footer a')].map(e=>({text:e.textContent,href:e.getAttribute('href')}))
      }))});
    }
    await context.close();
  }
  for (const path of ['/llms.txt','/feed.json']) {
    const c = await browser.newContext();
    const r=await c.request.get('https://slop.boutique'+path);
    results.push({path,status:r.status(),contentType:r.headers()['content-type']});
    await c.close();
  }
  await writeFile(out+'live-observations.json',JSON.stringify({capturedAt:new Date().toISOString(),results},null,2)+'\n');
  console.log(JSON.stringify(results.map(({text,links,...r})=>r),null,2));
} finally { await browser.close(); }
