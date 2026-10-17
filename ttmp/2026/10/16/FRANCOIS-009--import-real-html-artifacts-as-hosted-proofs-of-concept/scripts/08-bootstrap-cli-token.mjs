// Bootstrap a one-day Studio token; --admin requires explicit operator approval.
// Normal sign-in; no credentials/session cookies exported.
import {createRequire} from 'node:module';
import {mkdirSync,writeFileSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright-core/package.json');
const {chromium}=require('playwright-core');
const base='https://slop.boutique';
const admin=process.argv.includes('--admin');
const browser=await chromium.launch({executablePath:'/home/manuel/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',headless:true});
try {
  const context=await browser.newContext();const page=await context.newPage();
  let creds;
  try {creds=JSON.parse(execFileSync('vault',['kv','get','-address=https://vault.yolo.scapegoat.dev','-format=json','kv/apps/francois/prod/e2e-users'],{stdio:['ignore','pipe','ignore']})).data.data;}
  catch {throw new Error('Vault admin credential lookup failed');}
  try {
    await page.goto(base+'/auth/login?return=/account');
    await page.waitForURL(/zitadel\.yolo\.scapegoat\.dev/);
    await page.getByRole('textbox',{name:'Username or email'}).fill(creds.admin_username);
    await page.getByRole('button',{name:'Continue',exact:true}).click();
    await page.locator('input[type=password]').fill(creds.admin_password);
    await page.getByRole('button',{name:'Continue',exact:true}).click();
    for(let i=0;i<3&&!page.url().startsWith(base);i++) {
      const skip=page.getByRole('button',{name:/skip/i});
      await Promise.race([page.waitForURL(u=>u.href.startsWith(base),{timeout:15000}).catch(()=>{}),skip.waitFor({timeout:15000}).catch(()=>{})]);
      if(!page.url().startsWith(base)&&await skip.isVisible().catch(()=>false))await skip.click();
    }
    await page.waitForURL(u=>u.href.startsWith(base),{timeout:30000});
  }catch {throw new Error('Admin browser sign-in failed; credentials omitted');}
  creds=null;

  const result=await page.evaluate(async admin=>{
    const s=await(await fetch('/api/session')).json();
    if(!s.authenticated||s.user?.role!=='admin')throw new Error('Expected administrator');
    const r=await fetch('/api/tokens',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':s.csrfToken},body:JSON.stringify({name:admin?'FRANCOIS-009 approved tour setting':'FRANCOIS-009 CLI gallery publishing',days:1,scopes:admin?['admin:read','admin:write']:['studio']})});
    if(!r.ok)throw new Error('Token creation HTTP '+r.status());return r.json();
  },admin);
  const dir='/tmp/francois-009-private';mkdirSync(dir,{recursive:true,mode:0o700});
  const tokenFile=dir+(admin?'/admin-token':'/cli-token');
  writeFileSync(tokenFile,result.token+'\n',{mode:0o600,flag:'wx'});
  console.log(JSON.stringify({tokenFile,expiresAt:result.info.expiresAt,scopes:result.info.scopes}));
}finally{await browser.close();}
