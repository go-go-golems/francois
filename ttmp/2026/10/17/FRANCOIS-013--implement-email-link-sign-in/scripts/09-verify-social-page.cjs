// Local visual/DOM checks only; real OIDC routing has integration coverage.
const {chromium}=require('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
(async()=>{
 const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
 const base='http://localhost:18794';const checks=[];
 try {
  for(const width of [1440,390]) {
   const page=await browser.newPage({viewport:{width,height:1000}});
   const response=await page.goto(base+'/signin?return='+encodeURIComponent('/studio?tab=posts&sort=new'),{waitUntil:'networkidle'});
   assert.equal(response.status(),200);await page.evaluate(()=>document.fonts.ready);
   for(const provider of ['Google','GitHub']) {
    const link=page.getByRole('link',{name:'Continue with '+provider,exact:false});
    assert.equal(await link.count(),1);const target=new URL(await link.getAttribute('href'),base);
    assert.equal(target.searchParams.get('provider'),provider.toLowerCase());
    assert.equal(target.searchParams.get('return'),'/studio?tab=posts&sort=new');
   }
   assert.equal(await page.locator('form[action="/auth/email/request"]').count(),1);
   assert.equal(await page.getByRole('link',{name:'More sign-in options',exact:false}).count(),1);
   assert.equal(await page.getByText(/creator.*administrator/i).count(),0);
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.screenshot({path:process.argv[2]+`/social-local-${width}.png`,fullPage:true});
   checks.push({width,google:true,github:true,email:true,roleSpecificLink:false,returnPreserved:true,horizontalOverflow:false});await page.close();
  }
  await fs.writeFile(process.argv[2]+'/social-local-browser.json',JSON.stringify({base,checks,externalAuthenticationAttempted:false,mailSent:false},null,2)+'\n');
  console.log(JSON.stringify({checks}));
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
