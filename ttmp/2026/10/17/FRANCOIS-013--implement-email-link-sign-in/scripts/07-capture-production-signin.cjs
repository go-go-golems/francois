const {chromium}=require('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
 try {
  for(const width of [1440,390]) {
   const page=await browser.newPage({viewport:{width,height:1000}});
   const response=await page.goto('https://slop.boutique/signin?return=/account',{waitUntil:'networkidle'});
   assert.equal(response.status(),200);
   await page.evaluate(()=>document.fonts.ready);
   assert.equal(await page.locator('form[action="/auth/email/request"]').count(),1);
   assert.equal(await page.getByRole('link',{name:'Creator / administrator sign-in',exact:false}).count(),1);
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.screenshot({path:process.argv[2]+`/production-signin-${width}.png`,fullPage:true});
   await page.close();
  }
  console.log('Production email form and staff fallback verified at desktop/mobile sizes; no login email requested.');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
