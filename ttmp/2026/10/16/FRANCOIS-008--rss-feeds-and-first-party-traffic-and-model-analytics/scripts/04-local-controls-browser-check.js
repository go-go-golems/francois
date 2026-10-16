// Real local deployment check. Temporarily pauses AI/saving, then restores settings.
// It makes no provider call and persists no application document.
// prettier-ignore
async (page) => {
  const checks = [];
  const check = (ok, label) => { if (!ok) throw new Error(label); checks.push(label); };
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('http://localhost:8090/admin/settings');
  await page.getByRole('heading', {name:'Settings', exact:true}).waitFor();
  check(await page.getByText('Service controls', {exact:true}).count() === 1, 'existing settings editor contains service controls');
  check(await page.getByLabel('Enable AI', {exact:true}).isChecked(), 'AI enabled on migrated deployment');
  check(await page.getByLabel('Storage mode').inputValue() === 'read-write', 'storage initially read-write');
  const original = await page.evaluate(async () => (await fetch('/api/admin/settings').then(r=>r.json())).active.document);
  const save = async (doc) => page.evaluate(async doc => {
    const session = await fetch('/api/session').then(r=>r.json());
    const current = await fetch('/api/admin/settings').then(r=>r.json());
    const r = await fetch('/api/admin/settings', {method:'PATCH', headers:{'Content-Type':'application/json','X-CSRF-Token':session.csrfToken,'If-Match':JSON.stringify(String(current.active.revision))}, body:JSON.stringify(doc)});
    if(!r.ok) throw new Error('settings restore failed: '+r.status);
    return r.status;
  }, doc);
  let app;
  try {
    await page.getByLabel('Enable AI', {exact:true}).uncheck();
    await page.getByLabel('Storage mode').selectOption('read-only');
    await page.getByLabel('Service notice').fill('Local verification pause <plain text>.');
    const saved = page.waitForResponse(r=>r.url().endsWith('/api/admin/settings') && r.request().method()==='PATCH');
    await page.getByRole('button', {name:'Save & activate'}).click();
    check((await saved).status() === 200, 'controls save through existing revision-checked editor');
    const configured = await page.evaluate(async () => (await fetch('/api/admin/settings').then(r=>r.json())).active.document);
    check(configured.allowance.globalConcurrency<=6 && configured.allowance.perUserConcurrency===1, 'six global lanes and one per person');
    app = await page.context().newPage();
    app.on('pageerror', error => errors.push(error.message));
    await app.goto('http://localhost:8090/apps/capabilities');
    await app.getByRole('button', {name:'Open',exact:true}).waitFor();
    await app.getByRole('button', {name:'Open',exact:true}).click();
    await app.waitForURL(/\/instances\/ins_/);
    const openedURL=app.url();
    check(!await app.getByText('Share & info',{exact:true}).count(), 'launcher hides instance management');
    check(await app.getByRole('link',{name:'Close application'}).count()===1, 'launcher keeps simple Close action');
    const iframe=app.locator('iframe');await iframe.waitFor();
    const frame=await (await iframe.elementHandle()).contentFrame();
    await frame.waitForFunction(()=>!!window.claude?.use);
    const sampling=await frame.evaluate(async()=>{try{await (await window.claude.use('sample'))('Local pause verification');return 'unexpected success'}catch(e){return e.code}});
    check(sampling==='sampling_paused','real hosted AI request is rejected before provider use');
    let writes=0;
    app.on('request', request=>{if(request.url().endsWith('/rpc') && request.postData()?.includes('"method":"db.set"')) writes++;});
    const write=async()=>frame.evaluate(async()=>{try{await (await window.claude.use('db')).doc('checks/local-controls').set({text:'must not persist'});return 'unexpected success'}catch(e){return e.code}});
    check(await write()==='storage_read_only','real hosted write obeys read-only mode');
    const afterFirst=writes;
    check(await write()==='storage_read_only' && writes===afterFirst, 'subsequent autosave attempt is refused locally');
    check((await frame.locator('#francois-service-notice').innerText()).includes('Changes cannot be saved'), 'unsaved state is explicit');
    check((await frame.locator('#francois-service-notice').innerText()).includes('<plain text>') && !await frame.locator('#francois-service-notice img').count(), 'operator notice is rendered as plain text');
    const exists=await frame.evaluate(async()=> (await (await window.claude.use('db')).doc('checks/local-controls').get()).exists);
    check(!exists,'rejected write was not persisted and reads still work');
    const paused={...configured, operations:{...configured.operations,acceptNewVisits:false}};
    await save(paused);
    await app.goto('http://localhost:8090/apps/capabilities');
    await app.getByRole('button',{name:'Open',exact:true}).click();await app.waitForURL(/\/instances\/ins_/);
    check(app.url()===openedURL,'Open resumes the same workspace while new visits are paused');
  } finally {
    if(app)await app.close();
    await save(original);
  }
  await page.goto('http://localhost:8090/account');
  await page.getByRole('heading',{name:'Account',exact:true}).waitFor();
  check(!await page.getByText('Model allowance',{exact:true}).count() && !await page.locator('.meter,.stat-tiles').count(), 'Account has no public usage meter');
  check(await page.getByRole('button',{name:'Sign out',exact:true}).count()>=1, 'sign out remains available');
  await page.goto('http://localhost:8090/');
  check(!await page.locator('.hero,.creator-grid').count(), 'home omits hero and creator presentation');
  check(!await page.getByRole('link',{name:'Instances',exact:true}).count() && !await page.getByRole('link',{name:'Pricing',exact:true}).count(), 'normal navigation omits instances and pricing');
  const feed=await page.evaluate(async()=>{const r=await fetch('/feed.xml');const xml=await r.text();return{status:r.status,type:r.headers.get('content-type'),valid:!new DOMParser().parseFromString(xml,'text/xml').querySelector('parsererror'),items:new DOMParser().parseFromString(xml,'text/xml').querySelectorAll('item').length}});
  check(feed.status===200 && feed.type.includes('application/rss+xml') && feed.valid, 'combined RSS is valid XML');
  await page.goto('http://localhost:8090/admin/settings');
  await page.getByRole('heading',{name:'Settings',exact:true}).waitFor();
  check(await page.getByLabel('Enable AI',{exact:true}).isChecked() && await page.getByLabel('Storage mode').inputValue()==='read-write', 'temporary operation changes restored');
  check(!errors.length,'no browser page errors');
  return {passed:checks.length,checks,feedItems:feed.items,providerCallsMade:0};
}
