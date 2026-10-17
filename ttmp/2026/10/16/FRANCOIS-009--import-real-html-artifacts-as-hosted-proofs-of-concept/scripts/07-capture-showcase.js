// Run with the browser MCP against the existing local administrator session.
// These are curated views of real apps; published release bytes are unchanged.
async (page) => {
  const root='/home/manuel/code/wesen/go-go-golems/francois/ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/various/showcase/';
  const shots=[];
  const only=null; // Set to ['tabula','rag'], etc. to resume a subset.
  async function open(id) {
    const p=await page.context().newPage();
    await p.setViewportSize({width:1800,height:1125});
    await p.goto('http://localhost:8090/instances/'+id);
    await p.locator('iframe').waitFor();
    const f=await(await p.locator('iframe').elementHandle()).contentFrame();
    await f.waitForFunction(()=>document.readyState==='complete'&&document.body.innerText.length>100);
    await f.evaluate(async()=>{await document.fonts.ready;});
    await p.waitForTimeout(1000);return{p,f};
  }
  async function shot(p,name,subject) {
    await p.mouse.move(2,2);
    await (subject??p).screenshot({path:root+name+'.png'});
    shots.push(name);
  }
  if(!only||only.includes('rift')) {
    const{p,f}=await open('ins_a8fbd875f986beb842b6');
    try {
      if(await f.getByRole('button',{name:'Thaw',exact:true}).count())await f.getByRole('button',{name:'Thaw',exact:true}).click();
      for(const [index,name]of [[0,'rift-title'],[2,'rift-arcade'],[4,'rift-world']]) {
        await f.locator('.rs').nth(index).scrollIntoViewIfNeeded();await p.waitForTimeout(1000);
        await f.getByRole('button',{name:'Freeze',exact:true}).click();
        await shot(p,name);
        await f.getByRole('button',{name:'Thaw',exact:true}).click();
      }
    }finally{await p.close();}
  }
  if(!only||only.includes('phantasmagoria')) {
    const{p,f}=await open('ins_d6c846b4fb77486c45ad');
    try {
      const ids=await f.locator('#chips button').evaluateAll(es=>es.slice(1,4).map(e=>e.id));
      for(let i=0;i<ids.length;i++) {
        await f.locator('#'+ids[i]).click();await f.evaluate(()=>window.scrollTo(0,0));await p.waitForTimeout(1100);
        await shot(p,['phantasmagoria-menu','phantasmagoria-cartridge','phantasmagoria-kipple'][i]);
      }
    }finally{await p.close();}
  }
  if(!only||only.includes('deck')) {
    const{p,f}=await open('ins_9b1a6c16280179a82fc3');
    try {
      for(const [preset,name]of [['Animated 3D','lattice-deck-motion'],['Charts','lattice-deck-charts'],['Procedural 3D','lattice-deck-procedural']]) {
        const leave=f.getByTitle('Leave present mode (Esc)',{exact:true});if(await leave.count())await leave.click();
        if(!await f.locator('.preset-btn').isVisible())await f.getByTitle('Open the IDE',{exact:true}).click();
        await f.locator('.preset-btn').click();
        await f.locator('.preset-menu button').filter({hasText:preset}).first().click();
        await p.waitForTimeout(800);await f.getByTitle('Present: hide the panels',{exact:true}).click();await p.waitForTimeout(1300);
        await shot(p,name,f.locator('.ls-slide'));
        if(preset==='Procedural 3D')await shot(p,'lattice-deck-cover',f.locator('.ls-slide canvas'));
      }
    }finally{await p.close();}
  }
  if(!only||only.includes('tabula')) {
    const{p,f}=await open('ins_e0f18c608c25a8a083bd');
    try {
      await f.waitForFunction(()=>!!window.__tabula?.S?.session);
      await f.getByRole('button',{name:'Panels ▾',exact:true}).click();await f.getByRole('button',{name:'Reset layout',exact:true}).click();
      for(const side of ['right','left','bottom']){await f.getByRole('button',{name:'Panels ▾',exact:true}).click();await f.getByRole('button',{name:'Collapse '+side,exact:true}).click();}
      await f.locator('button').filter({hasText:/^Stage$/}).click();
      await f.getByRole('button').filter({hasText:'T-01'}).click();
      const cell=f.getByRole('spinbutton',{name:'Inputs!B2',exact:true});await cell.fill('420');await cell.press('Tab');await p.waitForTimeout(700);
      await shot(p,'tabula-overview');
      await f.getByRole('button').filter({hasText:'T-02'}).click();await p.waitForTimeout(600);
      await shot(p,'tabula-story',f.locator('.ls-slide'));
      await f.getByRole('button').filter({hasText:'S-02'}).click();await p.waitForTimeout(500);
      await shot(p,'tabula-sensitivity',f.locator('.ls-slide'));
      await f.getByRole('button').filter({hasText:'S-03'}).click();await p.waitForTimeout(600);
      await shot(p,'tabula-landscape',f.locator('.ls-slide'));
    }finally{await p.close();}
  }
  if(!only||only.includes('rag')) {
    const{p,f}=await open('ins_fb70d88f0d7d6a2a659f');
    try {
      await f.getByPlaceholder('Search the reports…',{exact:true}).fill('model concurrency');
      await f.waitForFunction(()=>window.__wb?.lastResult?.top(1)[0]?.doc==='Model Limits');
      await f.getByText('Concurrency',{exact:true}).first().click();await p.waitForTimeout(500);
      await shot(p,'rag-search');
      await f.locator('button').filter({hasText:/^Trace$/}).click();await p.waitForTimeout(400);
      await shot(p,'rag-trace');
      await f.locator('[data-ide=files]').click();await f.locator('button').filter({hasText:/^Corpus$/}).click();await p.waitForTimeout(400);
      await shot(p,'rag-corpus');
    }finally{await p.close();}
  }
  return{viewport:[1800,1125],source:'local hosted instances, same HTML hashes as production',shots};
}
