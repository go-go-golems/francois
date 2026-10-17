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
const out=fileURLToPath(new URL('../sources/',import.meta.url));await mkdir(out,{recursive:true});
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
const staff=await browser.newContext({viewport:{width:1440,height:900}});
await staff.addCookies([{name:'francois_session_dev',value:secret,url:base,httpOnly:true,sameSite:'Lax'}]);
const anonymous=await browser.newContext();const page=await staff.newPage();
const headers={Origin:base,'X-CSRF-Token':csrf};let post,original;
try {
 let ready=false;for(let n=0;n<60&&!ready;n++){try{ready=(await fetch(base+'/healthz')).ok;}catch{}if(!ready)await new Promise(r=>setTimeout(r,250));}assert(ready);
 const current=await(await staff.request.get(base+'/api/admin/settings')).json();original=structuredClone(current.active.document);
 assert.equal(original.storefront.journalEnabled,false);
 const created=await staff.request.post(base+'/api/studio/posts',{headers,data:{slug:'journal-visibility-check-'+randomBytes(4).toString('hex'),title:'Temporary journal visibility check',body:'Local browser test only.'}});assert.equal(created.status(),201);post=await created.json();
 assert.equal((await staff.request.post(base+'/api/studio/posts/'+post.id+'/publish',{headers,data:{}})).status(),200);
 await page.goto(base+'/admin/settings');const toggle=page.getByRole('checkbox',{name:'Show the blog publicly'});await toggle.waitFor();assert.equal(await toggle.isChecked(),false);
 assert.equal(await page.locator('.site-footer a[href="/journal"]').count(),0);
 const checks=['admin checkbox defaults off; SPA footer hides Journal'];
 for(const enabled of [true,false]) {
  await toggle.setChecked(enabled);
  const saved=page.waitForResponse(r=>r.url().endsWith('/api/admin/settings')&&r.request().method()==='PATCH');
  await page.getByRole('button',{name:'Save & activate',exact:true}).click();assert.equal((await saved).status(),200);
  await page.waitForFunction(value=>Boolean(document.querySelector('.site-footer a[href="/journal"]'))===value,enabled);
  assert.equal((await anonymous.request.get(base+'/journal/'+post.slug)).status(),enabled?200:404);
  const feed=await(await anonymous.request.get(base+'/feed.xml')).text();assert.equal(feed.includes(post.title),enabled);
  await toggle.scrollIntoViewIfNeeded();await page.screenshot({path:out+`admin-blog-${enabled?'enabled':'hidden'}.png`});
  checks.push(`admin save ${enabled?'enables':'hides'} journal page, RSS posts and SPA footer`);
 }
 const home=await anonymous.request.get(base+'/');assert.equal(home.status(),200);assert(!(await home.text()).includes('href="/journal"'));checks.push('artifact archive remains available; default-off state retained');
 await writeFile(out+'browser-validation.json',JSON.stringify({base,at:new Date().toISOString(),checks,credentialsSaved:false},null,2)+'\n');console.log(JSON.stringify({checks}));
} finally {
 if(post)await staff.request.delete(base+'/api/studio/posts/'+post.id,{headers});
 if(original){const state=await staff.request.get(base+'/api/admin/settings');original.storefront.journalEnabled=false;await staff.request.patch(base+'/api/admin/settings',{headers:{...headers,'If-Match':state.headers().etag},data:original});}
 sql(`UPDATE sessions SET revoked_at=now() WHERE token_hash=decode('${digest}','hex')`);
 await browser.close();
}
