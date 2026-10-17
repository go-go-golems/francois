// Fixed loopback integration: no production mode and no saved codes/live links.
import {createRequire} from 'node:module';
import {writeFile} from 'node:fs/promises';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const base='http://localhost:18795';
const out=fileURLToPath(new URL('../sources/implementation/',import.meta.url));
const address=`signup-${Date.now()}@slop.test`;
const database='postgres://emailtest:emailtest@127.0.0.1:5437/francois_email_signup_preview?sslmode=disable';
const userCount=()=>Number(execFileSync('psql',[database,'-Atc',`SELECT count(*) FROM users WHERE email='${address}'`],{encoding:'utf8'}).trim());
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
const context=await browser.newContext({viewport:{width:1440,height:900}});
const page=await context.newPage();
const checks=[];let stage='start';let previous;
async function receiveLink(){
 for(let attempt=0;attempt<40;attempt++){
  const list=await(await fetch('http://localhost:18025/api/v1/messages')).json();
  for(const message of list.messages??[]){
   if(!(message.To??[]).some(to=>to.Address===address))continue;
   const mail=await(await fetch('http://localhost:18025/api/v1/message/'+encodeURIComponent(message.ID))).json();
   const link=mail.Text?.match(/http:\/\/localhost:18795\/auth\/email\/verify\?[^\s<>"`]+/)?.[0];
   if(link&&link!==previous){previous=link;return link;}
  }
  await new Promise(r=>setTimeout(r,250));
 }
 throw new Error('No new matching email in isolated mail sink');
}
async function request(){
 await page.goto(base+'/signin?return=/account');
 await page.getByLabel('Email',{exact:true}).fill(address);
 await Promise.all([page.waitForURL('**/auth/email/request'),page.getByRole('button',{name:'Send link'}).click()]);
 assert.match(await page.locator('h1').innerText(),/Check your email/);
 return receiveLink();
}
async function capture(state){
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:900});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'horizontal overflow');
  await page.screenshot({path:out+`email-signup-${state}-${width}.png`,fullPage:true});
 }
}
try{
 stage='first email';
 const link=await request();assert.equal(userCount(),0);checks.push('real ZITADEL verification email delivered; no local account before proof');
 stage='confirmation GET';
 for(let i=0;i<2;i++){const response=await page.goto(link);assert.equal(response.status(),200);assert.equal(response.headers()['cache-control'],'no-store');assert.equal(response.headers()['referrer-policy'],'no-referrer');}
 assert.equal(userCount(),0);assert.equal((await(await page.request.get(base+'/api/session')).json()).authenticated,false);
 await capture('confirm');checks.push('scanner GET is read-only and authentication material is not cached');
 stage='first confirmation POST';
 await Promise.all([page.waitForURL('**/account'),page.getByRole('button',{name:'Continue'}).click()]);
 const session=await(await page.request.get(base+'/api/session')).json();
 assert.equal(session.authenticated,true);assert.equal(session.user.email,address);assert.equal(session.user.role,'subscriber');assert.equal(userCount(),1);
 await capture('account');checks.push('verification creates exactly one ordinary local account and signs in');
 stage='replay';assert.equal((await page.goto(link)).status(),400);checks.push('used signup link is rejected');
 await page.request.post(base+'/auth/logout',{headers:{Origin:base,'X-CSRF-Token':session.csrfToken}});
 stage='subsequent email OTP';const next=await request();
 assert.equal((await page.goto(next)).status(),200);
 await Promise.all([page.waitForURL('**/account'),page.getByRole('button',{name:'Continue'}).click()]);
 assert.equal((await(await page.request.get(base+'/api/session')).json()).authenticated,true);assert.equal(userCount(),1);
 checks.push('subsequent real session-OTP email signs into the same account');
 await writeFile(out+'email-signup-browser.json',JSON.stringify({base,at:new Date().toISOString(),checks,credentialsSaved:false},null,2)+'\n');
 console.log(JSON.stringify({checks}));
}catch(error){console.error(`Local email-signup verification failed at ${stage} (${error.name}); no code, token or link printed.`);process.exitCode=1;}finally{await browser.close();}
