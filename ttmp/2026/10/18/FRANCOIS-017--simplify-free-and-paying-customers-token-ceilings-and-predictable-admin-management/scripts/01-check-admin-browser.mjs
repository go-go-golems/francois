// Browser layout/action checks with explicit mocked API fixtures. Real API
// policy, concurrency and migrations are covered by the PostgreSQL Go tests.
import {createRequire} from 'node:module';
import {readFile,mkdir,writeFile} from 'node:fs/promises';
import assert from 'node:assert/strict';
const require=createRequire('/home/manuel/.npm/_npx/420ff84f11983ee5/node_modules/playwright/package.json');
const {chromium}=require('playwright');
const out=new URL('../sources/implementation/',import.meta.url);await mkdir(out,{recursive:true});
const user={id:'usr_fixture',name:'Alex Example',email:'alex@example.test',emailVerified:true,role:'subscriber',disabled:false};
const a={period:'2030-10-18',default:10000,limit:10000,override:null,used:1200,reserved:500,remaining:8300,resetsAt:'2030-10-19T00:00:00Z'};
const c={class:'Free',enabled:true,source:'free',accessUntil:null,asOf:'2030-10-18T12:00:00Z',support:{active:false,kind:'month'},ai:{eligible:true,reason:'Eligible; requests also require an available model lane',maxOutputTokens:8192},allowance:a,workspaces:{owned:1,limit:1,membersPerInstance:1}};
const d={user,summary:c,sessions:1,subscriptions:[],complimentary:[],instances:[],calls:[],audit:[]};
const browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport:{width:1440,height:1000},timezoneId:'UTC'});const errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.route('http://fixture.test/**',async route=>{
 const req=route.request(),path=new URL(req.url()).pathname;
 if(path.startsWith('/static/'))return route.fulfill({body:await readFile('pkg/catalog/web'+path),contentType:path.endsWith('.css')?'text/css':'application/javascript'});
 let data;
 if(path==='/api/session')data={authenticated:true,user:{id:'usr_admin',role:'admin',name:'Admin',email:'admin@example.test'},csrfToken:'fixture',brand:'slop.boutique',plan:{source:'staff',paid:true,instances:10}};
 else if(path.endsWith('/token-ceiling')){const body=req.postDataJSON();a.override=body.limit;a.limit=body.limit??a.default;a.remaining=Math.max(0,a.limit-a.used-a.reserved);data={ok:true};}
 else if(path.endsWith('/access-grants')){c.class='Paying';c.source='complimentary';c.accessUntil='2030-11-17T12:00:00Z';c.workspaces.limit=10;d.complimentary=[{id:'fixture-grant',until:c.accessUntil,reason:'Browser fixture'}];data={ok:true};}
 else if(path==='/api/admin/users/usr_fixture'&&req.method()==='PATCH'){user.disabled=req.postDataJSON().disabled;c.enabled=!user.disabled;c.ai.eligible=!user.disabled;c.ai.reason=user.disabled?'Account disabled':'Eligible; requests also require an available model lane';data={ok:true};}
 else if(path==='/api/admin/users/usr_fixture')data=d;
 else if(path.startsWith('/fonts/'))return route.fulfill({status:404,body:''});
 else return route.fulfill({contentType:'text/html',body:'<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/static/catalog.css"><div id="root"></div><script src="/static/catalog.js"></script>'});
 return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
});
try{
 await page.goto('http://fixture.test/admin/users/usr_fixture');await page.getByText('Free access',{exact:true}).waitFor();
 await page.getByLabel('Daily generated-token ceiling').fill('25000');await page.getByLabel('Ceiling reason').fill('Browser fixture');await page.getByRole('button',{name:'Save ceiling',exact:true}).click();await page.getByText('Custom ceiling · class default 10,000',{exact:true}).waitFor();assert.equal(a.limit,25000);
 await page.getByRole('button',{name:'Reset to default',exact:true}).click();await page.getByText('Class default',{exact:true}).waitFor();assert.equal(a.limit,10000);
 await page.locator('.toast').click().catch(()=>{});
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow ${width}`);await page.screenshot({path:new URL(`customer-free-${width}.png`,out).pathname,fullPage:true});}
 await page.getByRole('button',{name:'Disable account',exact:true}).click();await page.getByText('Account disabled',{exact:true}).first().waitFor();assert.equal(c.enabled,false);
 await page.getByRole('button',{name:'Enable account',exact:true}).click();await page.getByLabel('Access reason').fill('Browser fixture');await page.getByRole('button',{name:'Grant access',exact:true}).click();await page.getByText('Paying access',{exact:true}).waitFor();
 await page.locator('.toast').click().catch(()=>{});await page.setViewportSize({width:1440,height:1000});await page.screenshot({path:new URL('customer-complimentary-1440.png',out).pathname,fullPage:true});
 assert.deepEqual(errors,[]);
 const receipt={mockedAPI:true,realPolicyVerifiedSeparately:true,viewports:[1440,390],actions:['set ceiling','reset default','disable','enable','grant Paying access'],pageErrors:errors};await writeFile(new URL('browser.json',out),JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify(receipt));
}finally{await browser.close();}
