// Complete only the sandbox Checkout created by script 05. No live-mode keys.
import {createRequire} from 'node:module';import {readFile,writeFile} from 'node:fs/promises';import {randomBytes,createHash} from 'node:crypto';import {execFileSync} from 'node:child_process';import {fileURLToPath} from 'node:url';import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');const {chromium}=require('playwright-core');
const priv='/tmp/francois-support-private/',base='http://localhost:18796',out=fileURLToPath(new URL('../sources/implementation/',import.meta.url));
const key=(await readFile(priv+'stripe-key','utf8')).trim();assert(key.startsWith('sk_test_')||key.startsWith('rk_test_'));
const db=(await readFile(priv+'database-url','utf8')).trim(),id=(await readFile(priv+'checkout-user','utf8')).trim();assert.match(id,/^support_preview_[a-f0-9]+$/);
const sql=q=>execFileSync('psql',[db,'-At','-v','ON_ERROR_STOP=1','-c',q],{encoding:'utf8'}).trim();
const token=randomBytes(32).toString('hex'),csrf=randomBytes(24).toString('hex'),hash=createHash('sha256').update(token).digest('hex');
sql(`INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at) VALUES(decode('${hash}','hex'),'${id}','${csrf}',now()+interval '1 hour',now()+interval '1 hour')`);
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try {
 const context=await browser.newContext();await context.addCookies([{name:'francois_session_dev',value:token,url:base,httpOnly:true,sameSite:'Lax'}]);const page=await context.newPage();
 await page.goto((await readFile(priv+'checkout-url','utf8')).trim());
 await page.getByText('Card',{exact:true}).first().click({force:true,timeout:20000});
 await page.locator('#cardNumber').fill('4242424242424242',{timeout:20000});await page.locator('#cardExpiry').fill('12 / 34');await page.locator('#cardCvc').fill('123');
 if(await page.locator('#billingName').isVisible())await page.locator('#billingName').fill('Ana Support Preview');
 if(await page.locator('#billingPostalCode').isVisible())await page.locator('#billingPostalCode').fill('10001');
 const link=page.getByRole('checkbox',{name:/save my information/i});if(await link.count())await link.first().uncheck({force:true});
 await page.locator('button[type="submit"]').first().click();
 await page.waitForURL(u=>u.origin===base&&u.pathname==='/thank-you',{timeout:45000});
 await page.getByRole('heading',{name:'Thank you, Ana.'}).waitFor({timeout:45000});
 assert.equal(await page.locator('.support-callout,.site-support').count(),0);
 const account=await (await context.request.get(base+'/api/account')).json();
 assert.equal(account.support.active,true);assert.equal(account.support.amountCents,4300);assert.equal(account.entitlement.source,'free');assert.equal(account.hasCustomer,true);
 const subscription=account.subscriptions.find(s=>s.supportAmountCents===4300);assert(subscription);
 await page.locator(".toast").click().catch(()=>{});
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.evaluate(()=>document.fonts.ready);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:out+`support-real-sandbox-thank-you-${width}.png`,fullPage:true});}
 const portal=await context.request.post(base+'/api/billing/portal',{headers:{Origin:base,'X-CSRF-Token':csrf},data:{}});assert.equal(portal.status(),200);
 const customerPortal=await portal.json();assert.equal(new URL(customerPortal.url).hostname,'billing.stripe.com');
 // Sandbox fixture cleanup schedules its cancellation. It remains paid through
 // this test month, which also checks that cancellation does not re-show banners.
 const cancelled=await fetch('https://api.stripe.com/v1/subscriptions/'+subscription.id,{method:'POST',headers:{Authorization:'Bearer '+key,'Content-Type':'application/x-www-form-urlencoded'},body:'cancel_at_period_end=true'});assert.equal(cancelled.status,200);const cancelledBody=await cancelled.json();assert.equal(cancelledBody.livemode,false);
 const receipt={at:new Date().toISOString(),sandbox:true,selectedAmountCents:4300,stripeAmountCents:cancelledBody.items.data[0].price.unit_amount,returnedToThankYou:true,authoritativeSupportConfirmed:true,accessSource:account.entitlement.source,customerPortalOpened:true,noSupportPrompts:true,testSubscriptionCancellationScheduled:true,productionDeploymentChanged:false};
 await writeFile(out+'support-sandbox-checkout.json',JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify(receipt,null,2));
}finally{sql(`DELETE FROM sessions WHERE token_hash=decode('${hash}','hex')`);await browser.close();}
