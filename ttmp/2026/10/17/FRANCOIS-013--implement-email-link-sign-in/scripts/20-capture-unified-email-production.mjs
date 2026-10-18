// Read-only production sign-in capture; never submits an email address.
import {createRequire} from 'node:module';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const out=fileURLToPath(new URL('../sources/implementation/',import.meta.url));
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try{
 const context=await browser.newContext();const page=await context.newPage();
 const response=await page.goto('https://slop.boutique/signin');assert.equal(response.status(),200);
 assert.equal(await page.getByLabel('Email',{exact:true}).getAttribute('maxlength'),'200');
 assert.match(await page.locator('main').innerText(),/any method above to create an account/);
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:900});await page.evaluate(()=>document.fonts.ready);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.screenshot({path:out+`unified-email-production-signin-${width}.png`,fullPage:true});
 }
 console.log('Production signup form captured at desktop/mobile; no email requested.');
}finally{await browser.close();}
