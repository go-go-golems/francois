const {chromium}=require('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
 try {
  for(const width of [1440,390]) {
   const page=await browser.newPage({viewport:{width,height:1000}});
   for(const [name,path] of [['archive','/'],['lattice-deck','/apps/lattice-deck']]) {
    const response=await page.goto('https://slop.boutique'+path,{waitUntil:'networkidle'});
    assert.equal(response.status(),200);
    await page.evaluate(()=>document.fonts.ready);
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'horizontal overflow');
    assert.equal(await page.locator('.site-footer a[href="/journal"]').count(),0);
    await page.screenshot({path:process.argv[2]+`/production-${name}-${width}.png`,fullPage:true});
   }
   await page.close();
  }
  console.log('Four production screenshots captured; desktop/mobile have no horizontal overflow or Journal footer link.');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
