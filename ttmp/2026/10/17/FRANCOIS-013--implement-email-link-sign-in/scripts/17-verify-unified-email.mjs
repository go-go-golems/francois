// One isolated real-browser gate: new signup plus verified provider-only identity.
// No production mode. No authentication URL/code/token is saved or printed.
import {createRequire} from 'node:module';
import {execFileSync} from 'node:child_process';
import {writeFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const fixtureScript=fileURLToPath(new URL('./18-seed-provider-only.py',import.meta.url));
const fixture=JSON.parse(execFileSync('python3',[fixtureScript],{encoding:'utf8'}));
const base='http://localhost:18795';
const out=fileURLToPath(new URL('../sources/implementation/',import.meta.url));
const database='postgres://emailtest:emailtest@127.0.0.1:5437/francois_email_signup_preview?sslmode=disable';
const count=email=>Number(execFileSync('psql',[database,'-Atc',`SELECT count(*) FROM users WHERE email='${email}'`],{encoding:'utf8'}).trim());
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
const checks=[];let stage='start';
async function receive(email,previous){
 for(let i=0;i<40;i++){
  const listing=await(await fetch('http://localhost:18025/api/v1/messages')).json();
  for(const message of listing.messages??[]){
   if(!(message.To??[]).some(to=>to.Address===email))continue;
   const mail=await(await fetch('http://localhost:18025/api/v1/message/'+encodeURIComponent(message.ID))).json();
   const link=mail.Text?.match(/http:\/\/localhost:18795\/auth\/email\/verify\?[^\s<>"`]+/)?.[0];
   if(link&&link!==previous)return link;
  }
  await new Promise(r=>setTimeout(r,250));
 }
 throw new Error('No new local email');
}
try{
 stage='local preview readiness';
 let ready=false;for(let i=0;i<60&&!ready;i++){try{ready=(await fetch(base+'/healthz')).ok;}catch{}if(!ready)await new Promise(r=>setTimeout(r,250));}
 assert(ready,'local preview not ready');
 for(const [kind,email] of [['new address',`unified-${Date.now()}@slop.test`],['existing verified provider',fixture.email]]){
  const context=await browser.newContext({viewport:{width:1440,height:900}});const page=await context.newPage();let previous;
  assert.equal(count(email),0);
  for(const pass of ['first confirmation','returning login']){
   stage=kind+' '+pass+' send';
   await page.goto(base+'/signin?return=/account');
   assert.match(await page.locator('main').innerText(),/fifteen minutes/);
   await page.getByLabel('Email',{exact:true}).fill(email);
   await Promise.all([page.waitForURL('**/auth/email/request'),page.getByRole('button',{name:'Send link'}).click()]);
   assert.match(await page.locator('h1').innerText(),/Check your email/);
   const link=await receive(email,previous);previous=link;
   stage=kind+' '+pass+' confirm';
   for(let i=0;i<2;i++){const response=await page.goto(link);assert.equal(response.status(),200);assert.equal(response.headers()['cache-control'],'no-store');}
   if(pass==='first confirmation')assert.equal(count(email),0);
   await Promise.all([page.waitForURL('**/account'),page.getByRole('button',{name:'Continue'}).click()]);
   const session=await(await page.request.get(base+'/api/session')).json();assert.equal(session.authenticated,true);assert.equal(session.user.email,email);assert.equal(count(email),1);
   assert.equal((await page.goto(link)).status(),400);
   await page.request.post(base+'/auth/logout',{headers:{Origin:base,'X-CSRF-Token':session.csrfToken}});
   checks.push(kind+': '+pass+' delivered, authenticated same identity, replay denied');
  }
  await context.close();
 }
 assert.equal(JSON.parse(execFileSync('python3',[fixtureScript,fixture.subject],{encoding:'utf8'})).emailOTPEnrolled,false);
 checks.push('existing verified identity required no OTP enrollment or provider session');
 await writeFile(out+'unified-email-browser.json',JSON.stringify({at:new Date().toISOString(),base,checks,productionMailRequested:false,credentialsSaved:false},null,2)+'\n');
 console.log(JSON.stringify({checks}));
}catch(error){console.error(`Unified email verification failed at ${stage} (${error.name}); no credentials or link printed.`);process.exitCode=1;}finally{await browser.close();}
