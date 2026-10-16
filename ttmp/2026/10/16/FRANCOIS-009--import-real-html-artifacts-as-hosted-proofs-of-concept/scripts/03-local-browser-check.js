async (page) => {
  const root='/home/manuel/code/wesen/go-go-golems/francois/ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept';
  const results=[];
  for(const id of ['lattice-rift','lattice-phantasmagoria','tabula-workbench','lattice-deck','rag-workbench']) {
    const app=await page.context().newPage();
    const errors=[];const failed=[];
    app.on('pageerror', e=>errors.push(e.message));
    app.on('requestfailed', r=>failed.push({url:r.url().split('?')[0],error:r.failure()?.errorText}));
    try {
      await app.setViewportSize({width:1440,height:1000});
      await app.goto('http://localhost:8090/apps/'+id);
      await app.getByRole('button',{name:'Open',exact:true}).click();
      await app.waitForURL(/\/instances\/ins_/);
      const handle=await app.locator('iframe').elementHandle();
      const frame=await handle.contentFrame();
      await frame.waitForFunction(()=>document.readyState==='complete' && document.body.innerText.length>50);
      await app.waitForTimeout(1500);
      const state=await frame.evaluate(()=>({title:document.title,text:document.body.innerText.slice(0,550),canvases:document.querySelectorAll('canvas').length,buttons:document.querySelectorAll('button').length,claude:typeof window.claude?.use,rag:window.__wb?{preset:window.__wb.preset?.id,files:window.__wb.bundled.length,hasWorkspace:!!window.__wb.ws,caps:Object.fromEntries(Object.entries(window.__wb.caps).map(([k,v])=>[k,!!v]))}:null}));
      await app.screenshot({path:root+'/various/screenshots/'+id+'-local.png',fullPage:true});
      results.push({id,url:app.url(),...state,errors,failed});
    } finally { await app.close(); }
  }
  return results;
}
