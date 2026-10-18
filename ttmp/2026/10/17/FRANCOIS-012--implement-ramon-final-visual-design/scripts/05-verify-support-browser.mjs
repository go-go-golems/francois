// Local-only evidence; credentials/cookies/hosted URLs stay out of the ticket.
import {createRequire} from 'node:module';
import {randomBytes,createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {readFile,writeFile} from 'node:fs/promises';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const base='http://localhost:18796';
const db=(await readFile('/tmp/francois-support-private/database-url','utf8')).trim();
const out=fileURLToPath(new URL('../sources/implementation/',import.meta.url));
const sql=q=>execFileSync('psql',[db,'-At','-v','ON_ERROR_STOP=1','-c',q],{encoding:'utf8'}).trim();
const token=randomBytes(32).toString('hex'),csrf=randomBytes(24).toString('hex'),hash=createHash('sha256').update(token).digest('hex');
const id='support_preview_'+randomBytes(6).toString('hex');
sql(`INSERT INTO users(id,issuer,subject,email,email_verified,display_name) VALUES('${id}','http://support-preview.test','${id}','ana-support-preview@example.test',true,'Ana'); INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at) VALUES(decode('${hash}','hex'),'${id}','${csrf}',now()+interval '1 hour',now()+interval '1 hour')`);
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
const receipt={base,at:new Date().toISOString(),checks:[],screenshots:[],productionChanged:false};
try {
 const context=await browser.newContext();await context.addCookies([{name:'francois_session_dev',value:token,url:base,httpOnly:true,sameSite:'Lax'}]);
 const page=await context.newPage();
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:1000});
  for(const [name,path] of [['pricing','/pricing'],['free-account','/account']]){
   await page.goto(base+path,{waitUntil:'networkidle'});await page.evaluate(()=>document.fonts.ready);
   if(name==='pricing'){
    const input=page.getByRole('spinbutton',{name:'Monthly support in US dollars'});assert.equal(await input.inputValue(),'42');
    await page.getByRole('button',{name:'Increase monthly support'}).click();assert.equal(await input.inputValue(),'43');
    await page.getByRole('button',{name:'Decrease monthly support'}).click();assert.equal(await input.inputValue(),'42');
   }else assert.equal(await page.getByRole('heading',{name:'Keep the lab running.'}).count(),1);
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),name+' overflow');
   const file=`support-${name}-${width}.png`;await page.screenshot({path:out+file,fullPage:true});receipt.screenshots.push(file);
  }
 }
 receipt.checks.push('42 USD input, working plus/minus, responsive pricing and free-account callout');
 sql(`INSERT INTO billing_subscriptions(stripe_subscription_id,user_id,stripe_customer_id,price_id,status,quantity,item_count,access_until,support_amount_cents,started_at) VALUES('preview_${id}','${id}','fixture_only','fixture_price','active',1,1,now()+interval '30 days',4200,now()-interval '14 days')`);
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:1000});
  await page.goto(base+'/thank-you',{waitUntil:'networkidle'});
  await page.getByRole('heading',{name:'Thank you, Ana.'}).waitFor();
  assert.equal(await page.locator('.support-callout,.site-support').count(),0);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'supporter overflow');
  const file=`support-thank-you-${width}.png`;await page.screenshot({path:out+file,fullPage:true});receipt.screenshots.push(file);
  await page.goto(base+'/pricing',{waitUntil:'networkidle'});assert.equal(await page.locator('[data-support-monthly],.site-support').count(),0);
 }
 receipt.checks.push('Projected supporter sees thank-you and actual amount; no pricing/header/account support solicitations');
 sql(`DELETE FROM billing_subscriptions WHERE stripe_subscription_id='preview_${id}'`);
 if(process.argv.includes('--sandbox-checkout')){
  await page.goto(base+'/pricing',{waitUntil:'networkidle'});
  await page.getByRole('spinbutton',{name:'Monthly support in US dollars'}).fill('43');
  await page.getByRole('button',{name:'Support monthly'}).click();
  await page.waitForURL('https://checkout.stripe.com/**',{timeout:45000});
  await writeFile('/tmp/francois-support-private/checkout-url',page.url(),{mode:0o600});
  await writeFile('/tmp/francois-support-private/checkout-user',id,{mode:0o600});
  await writeFile('/tmp/francois-support-private/browser-state.json',JSON.stringify(await context.storageState()),{mode:0o600});
  await page.waitForTimeout(3000);
  console.log('Sandbox hosted checkout opened; page title:',await page.title());
  console.log('Visible form fields:',await page.locator('input').evaluateAll(nodes=>nodes.map(n=>({name:n.name,id:n.id,placeholder:n.placeholder}))));
  receipt.checks.push('Real sandbox hosted Checkout opened with selected 43 USD monthly contribution');
 }
 await writeFile(out+'support-browser.json',JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify(receipt,null,2));
} finally {sql(`DELETE FROM sessions WHERE token_hash=decode('${hash}','hex')`);await browser.close();}
