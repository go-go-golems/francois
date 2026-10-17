// Read-only page/provider-entry verification; no credentials or email submission.
const {chromium}=require('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
(async()=>{
 const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
 const checks=[];const entries=[];let active='';
 try {
  for(const base of ['https://slop.boutique','https://francois.yolo.scapegoat.dev']) {
   active=base+' page';const context=await browser.newContext();const page=await context.newPage();
   const response=await page.goto(base+'/signin?return=/account',{waitUntil:'networkidle'});assert.equal(response.status(),200);
   for(const label of ['Continue with Google','Continue with GitHub','More sign-in options'])assert.equal(await page.getByRole('link',{name:label,exact:false}).count(),1);
   assert.equal(await page.locator('form[action="/auth/email/request"]').count(),1);
   assert.equal(await page.getByText(/creator.*administrator/i).count(),0);
   assert.equal(await page.getByText('New here? Continue above to create an account.',{exact:true}).count(),1);
   checks.push({base,google:true,github:true,email:true,nativeFallback:true,roleSpecificLink:false,signupGuidance:true});
   if(base==='https://slop.boutique')for(const width of [1440,390]){
    await page.setViewportSize({width,height:1000});await page.evaluate(()=>document.fonts.ready);
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    await page.screenshot({path:process.argv[2]+`/social-production-${width}.png`,fullPage:true});
   }
   await context.close();
  }
  for(const provider of ['google','github']) {
   active=provider+' entry';const context=await browser.newContext();const page=await context.newPage();let auth;
   page.on('response',r=>{if(r.url().startsWith('https://slop.boutique/auth/login?')){const location=r.headers()['location'];if(location)auth=new URL(location);}});
   await page.goto('https://slop.boutique/auth/login?provider='+provider+'&return=/account',{waitUntil:'domcontentloaded'});
   const expected=provider==='google'?'accounts.google.com':'github.com';
   await page.waitForURL(u=>u.hostname===expected,{timeout:30000});
   active=provider+' login field';
   const input=provider==='google'?page.getByRole('textbox',{name:'Email or phone',exact:true}):page.locator('input[name=login]');
   await input.waitFor({state:'visible',timeout:20000});
   assert(auth);assert.equal(auth.origin,'https://zitadel.yolo.scapegoat.dev');
   const q=auth.searchParams;assert(q.get('scope').includes('urn:zitadel:iam:org:id:394966170567966836'));
   const id=provider==='google'?'395005753389219858':'395005753372442642';assert(q.get('scope').includes('urn:zitadel:iam:org:idp:id:'+id));
   assert(q.get('state'));assert(q.get('nonce'));assert(q.get('code_challenge'));assert.equal(q.get('code_challenge_method'),'S256');
   const at=new URL(page.url());entries.push({provider,origin:at.origin,path:at.pathname,loginFormVisible:true,organizationScope:true,providerScope:true,stateNoncePKCE:true});
   await page.screenshot({path:process.argv[2]+`/social-${provider}-entry.png`,fullPage:true});await context.close();
  }
  const result={at:new Date().toISOString(),checks,entries,credentialsSubmitted:false,emailsRequested:0,externalConsentCompleted:false};
  await fs.writeFile(process.argv[2]+'/social-production-browser.json',JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result));
 } catch(e) {
  // Keep authorization query strings and temporary codes out of diagnostics.
  console.error('Production social verification failed at '+active+' ('+e.name+'); no credentials were submitted.');process.exitCode=1;
 } finally {await browser.close();}
})();
