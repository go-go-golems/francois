// Local rollout check against the real localhost:8090 deployment.
// Receipt expects the recorded pre-deployment baseline of five model calls.
// Pass as a function expression to Playwright browser_run_code; do not add a trailing semicolon.
// prettier-ignore
async(page)=>{
const checks=[],errors=[];page.on('pageerror',e=>errors.push(String(e)));
const check=(ok,name)=>{if(!ok)throw Error(name);checks.push(name)};
for(const path of ['/','/apps','/journal','/pricing']) { const response=await page.goto('http://localhost:8090'+path);check(response.status()===200,'Public document '+path+' responds'); }
await page.goto('http://localhost:8090/admin/analytics');
await page.getByRole('heading',{name:'Daily page views',exact:true}).waitFor();
const live=await page.evaluate(async()=>{
const read=async(path)=>{const r=await fetch(path);return {status:r.status,body:await r.json()}};
const [report,filtered,session,anon,...feeds]=await Promise.all([
read('/api/admin/analytics'),read('/api/admin/analytics?path=%2Fapps&referrer='),read('/api/session'),
fetch('/api/admin/analytics',{credentials:'omit'}).then(async r=>({status:r.status})),
...['/apps/feed.xml','/journal/feed.xml'].map(async path=>{const r=await fetch(path);const xml=new DOMParser().parseFromString(await r.text(),'application/xml');return {path,status:r.status,type:r.headers.get('content-type'),valid:!xml.querySelector('parsererror')&&xml.documentElement.tagName==='rss'};})
]);
return {report,filtered,session:{status:session.status,authenticated:session.body.authenticated,role:session.body.user?.role},anon,feeds};
});
check(live.session.authenticated&&live.session.role==='admin','Existing admin session survives service recreation');
check(live.report.status===200&&live.report.body.traffic.views>=5,'Authenticated analytics reads real collected traffic');
check(live.filtered.status===200&&live.filtered.body.traffic.views>=1&&live.filtered.body.groups.pages.every(r=>r.key==='/apps'),'Exact page and direct-referrer filters work against real database');
for(const dim of ['pages','referrers'])check(live.report.body.series[dim].reduce((n,r)=>n+r.views,0)===live.report.body.traffic.views,'Real '+dim+' stack reconciles with total');
check(live.anon.status===401,'Unauthenticated report access is denied');
for(const feed of live.feeds)check(feed.status===200&&feed.valid&&feed.type.startsWith('application/rss+xml'),'Live RSS '+feed.path+' is valid');
await page.getByRole('button',{name:'Stack by page',exact:true}).click();check(await page.getByRole('button',{name:'Stack by page',exact:true}).getAttribute('aria-pressed')==='true','Stack-by-page mode selects');
await page.getByRole('button',{name:'Stack by referrer',exact:true}).click();check(await page.getByRole('button',{name:'Stack by referrer',exact:true}).getAttribute('aria-pressed')==='true','Stack-by-referrer mode selects');
await page.locator('.analytics-date-trigger').click();
await page.getByRole('dialog').getByRole('button',{name:'Last 7 days',exact:true}).click();
await page.getByRole('dialog').getByRole('button',{name:'Cancel',exact:true}).click();
check(!await page.getByRole('dialog').isVisible(),'Date presets and Cancel work in deployed UI');
await page.getByRole('button',{name:'Model usage',exact:true}).click();
await page.getByRole('heading',{name:'Usage by artifact',exact:true}).waitFor();
check(await page.locator('svg[role=img]').count()===2,'Live model token and cost graphs render');
const calls=live.report.body.models.daily.reduce((n,r)=>n+r.calls,0);
const artifactCalls=live.report.body.models.artifacts.reduce((n,r)=>n+r.calls,0);
check(calls===5&&artifactCalls===5&&live.report.body.models.artifacts.every(r=>r.key!==''),'All existing calls appear in per-artifact reporting without provider calls');
check(errors.length===0,'No browser page errors');
return {checks,report:{trafficViews:live.report.body.traffic.views,modelCalls:calls,artifactCalls,artifacts:live.report.body.models.artifacts.map(r=>({id:r.key,label:r.label,calls:r.calls}))},errors};
}
