// Local-only admin UI check. Temporary post/session are removed in finally.
import {createRequire} from 'node:module';
import {readFile,mkdir,writeFile} from 'node:fs/promises';
import {execFileSync} from 'node:child_process';
import {randomBytes,createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const base='http://localhost:18792';
const db=(await readFile('/tmp/francois-012/database-url','utf8')).trim();
const sql=q=>execFileSync('psql',[db,'-At','-v','ON_ERROR_STOP=1','-c',q],{encoding:'utf8'}).trim();
const secret=randomBytes(32).toString('hex'),csrf=randomBytes(24).toString('hex');
const digest=createHash('sha256').update(secret).digest('hex');
const admin=sql(`INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at,id_token) SELECT decode('${digest}','hex'),id,'${csrf}',now()+interval '15 minutes',now()+interval '15 minutes','' FROM users WHERE role='admin' AND NOT disabled ORDER BY created_at LIMIT 1 RETURNING user_id`);
assert(admin,'local preview needs an existing administrator');
const out=fileURLToPath(new URL('../sources/',import.meta.url));
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
const staff=await browser.newContext();
await staff.addCookies([{name:'francois_session_dev',value:secret,url:base,httpOnly:true,sameSite:'Lax'}]);
const page=await staff.newPage();const checks=[];
try {
 await page.goto(base+'/instances');
 await page.locator('.site-footer').getByRole('button',{name:'Sign out',exact:true}).waitFor();
 const initial=await page.title();
 for(const route of ['/studio','/admin']) {
  await page.locator('.site-footer a[href="/account"]').click();
  await page.waitForFunction(()=>document.title.startsWith('Account · '));
  await page.locator(`.site-footer a[href="${route}"]`).click();
  await page.waitForFunction(value=>document.title===value,initial);
  assert.equal(await page.locator('.site-footer').getByRole('button',{name:'Sign out',exact:true}).count(),1);
  checks.push('Account → '+route+' restores title and retains footer sign-out');
 }
 let reject=true;
 await page.route('**/auth/logout',async route=>{
  if(reject)await route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:{code:'test_failure',message:'Temporary logout failure'}})});
  else await route.continue();
 });
 await page.locator('.site-footer').getByRole('button',{name:'Sign out',exact:true}).click();
 await page.getByRole('status').filter({hasText:'Temporary logout failure'}).waitFor();
 assert.equal((await staff.request.get(base+'/api/session').then(r=>r.json())).authenticated,true);
 checks.push('failed logout shows error and preserves session');reject=false;
 await page.locator('.site-footer').getByRole('button',{name:'Sign out',exact:true}).click();
 await page.waitForURL(base+'/');
 assert.equal((await staff.request.get(base+'/api/session').then(r=>r.json())).authenticated,false);
 checks.push('footer logout revokes session and returns home');
 await page.goto(base+'/instances');
 await page.locator('.site-footer').waitFor();
 assert.equal(await page.locator('.site-footer').getByRole('button',{name:'Sign out',exact:true}).count(),0);
 checks.push('anonymous shell hides sign-out');
 await writeFile(out+'review-browser-validation.json',JSON.stringify({base,at:new Date().toISOString(),checks,credentialsSaved:false},null,2)+'\n');console.log(JSON.stringify({checks}));
} finally {
 sql(`UPDATE sessions SET revoked_at=now() WHERE token_hash=decode('${digest}','hex')`);
 await browser.close();
}
