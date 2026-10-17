// Fixed local real-mail browser journey. No tokens/codes/URLs are persisted.
import { createRequire } from 'node:module';
import { mkdir,writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const base='http://localhost:18793';
const out=fileURLToPath(new URL('../sources/implementation/',import.meta.url));
await mkdir(out,{recursive:true});
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
const context=await browser.newContext({viewport:{width:1440,height:900}});
const page=await context.newPage();
const checks=[];const screenshots=[];const navigationHeaders=[];
page.on('request',async request=>{
 if(request.method()==='POST' && new URL(request.url()).pathname.startsWith('/auth/email/')){
  const h=await request.allHeaders();navigationHeaders.push({path:new URL(request.url()).pathname,origin:h.origin,site:h['sec-fetch-site'],mode:h['sec-fetch-mode']});
 }
});
async function capture(state){
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:900});await page.evaluate(()=>document.fonts.ready);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'horizontal overflow');
  const file=`signin-${state}-${width}.png`;await page.screenshot({path:out+file,fullPage:true});screenshots.push(file);
 }
 await page.setViewportSize({width:1440,height:900});
}
try{
 let ready=false;
 for(let attempt=0;attempt<60&&!ready;attempt++){
  try {ready=(await fetch(base+'/healthz')).ok;}catch{}
  if(!ready)await new Promise(r=>setTimeout(r,250));
 }
 assert(ready,'local preview did not become healthy');
 const res=await page.goto(base+'/signin?return=/account');
 assert.equal(res.status(),200);assert.equal(res.headers()['cache-control'],'no-store');assert.equal(res.headers()['referrer-policy'],'no-referrer');
 assert.equal(await page.locator('script').count(),0);checks.push('form no-store/no-referrer; works without client scripts');
 await capture('form');
 await page.getByLabel('Email',{exact:true}).fill('supporter@slop.test');
 await Promise.all([page.waitForURL('**/auth/email/request'),page.getByRole('button',{name:'Send link'}).click()]);
 assert.match(await page.locator('h1').textContent(),/Check your email/,await page.locator('main').innerText());await capture('sent');
 let link;
 for(let attempt=0;attempt<40&&!link;attempt++){
  const listing=await(await fetch('http://localhost:18025/api/v1/messages')).json();
  for(const message of listing.messages??[]){
   const mail=await(await fetch('http://localhost:18025/api/v1/message/'+encodeURIComponent(message.ID))).json();
   const match=mail.Text?.match(/http:\/\/localhost:18793\/auth\/email\/verify\?[^\s<>"`]+/);
   if(match){link=match[0];break;}
  }
  if(!link)await new Promise(r=>setTimeout(r,250));
 }
 assert(link,'no matching delivered test email');checks.push('actual ZITADEL SMTP email delivered');
 const confirmation=await page.goto(link);assert.equal(confirmation.status(),200);assert.equal(confirmation.headers()['referrer-policy'],'no-referrer');
 assert.equal(await page.locator('script').count(),0);await capture('confirm');
 const before=await(await page.request.get(base+'/api/session')).json();assert.equal(before.authenticated,false);checks.push('GET confirmation did not sign in');
 await Promise.all([page.waitForURL('**/account'),page.getByRole('button',{name:'Continue'}).click()]);
 const account=await(await page.request.get(base+'/api/session')).json();assert.equal(account.authenticated,true);assert.equal(account.user.email,'supporter@slop.test');
 checks.push('same supporter signed in through actual browser POST');
 await page.waitForLoadState('networkidle');await capture('account');
 const replay=await page.goto(link);assert.equal(replay.status(),400);await capture('used');checks.push('link replay denied');
 const logout=await page.request.post(base+'/auth/logout',{headers:{Origin:base,'X-CSRF-Token':account.csrfToken}});assert.equal(logout.status(),200);assert.equal((await logout.json()).redirect,undefined);
 assert.equal((await(await page.request.get(base+'/api/session')).json()).authenticated,false);checks.push('local logout revoked session');
 await writeFile(out+'browser-validation.json',JSON.stringify({base,at:new Date().toISOString(),checks,screenshots,credentialsSaved:false},null,2)+'\n');
 console.log(JSON.stringify({checks,screenshots:screenshots.length}));
}catch(error){console.log(JSON.stringify({navigationHeaders}));throw error;}finally{await browser.close();}
