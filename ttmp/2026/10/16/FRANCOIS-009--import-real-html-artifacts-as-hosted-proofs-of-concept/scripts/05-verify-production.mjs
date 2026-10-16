// Uses the normal in-memory signed-in browser context supplied by step 04.
import path from 'node:path';
import assert from 'node:assert/strict';
export async function verify(context,base,ticket,releases) {
  const results=[];
  for(const {id,releaseId} of releases) {
    const page=await context.newPage();
    const errors=[],failed=[];
    page.on('pageerror',e=>errors.push(e.message));
    page.on('requestfailed',r=>failed.push({url:r.url().split('?')[0],error:r.failure()?.errorText}));
    try {
      await page.goto(base+'/apps/'+id);
      await page.getByRole('button',{name:'Open',exact:true}).click();
      await page.waitForURL(/\/instances\/ins_/);
      const upgraded=await page.evaluate(async releaseId=>{
        const session=await(await fetch('/api/session')).json();
        const current=await(await fetch('/api'+location.pathname)).json();
        if(current.releaseId===releaseId)return 200;
        const r=await fetch('/api'+location.pathname+'/upgrade',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':session.csrfToken},body:JSON.stringify({releaseId})});
        return r.status;
      },releaseId);
      assert.equal(upgraded,200,'Explicit workspace release upgrade');
      await page.reload();
      await page.locator('iframe').waitFor();
      let frame=await(await page.locator('iframe').elementHandle()).contentFrame();
      await frame.waitForFunction(()=>document.readyState==='complete'&&document.body.innerText.length>50);
      await page.waitForTimeout(2500);
      const result={id,instance:new URL(page.url()).pathname,title:await frame.title(),origin:new URL(frame.url()).origin};
      if(id==='lattice-rift') {
        const b=frame.getByRole('button',{name:/^(Thaw|Freeze)$/});
        const before=await b.textContent();await b.click();const after=await b.textContent();
        assert.notEqual(before,after);await b.click();result.interaction={before,after};
      } else if(id==='lattice-phantasmagoria') {
        const before=await frame.locator('#scName').textContent();
        await frame.locator('#btnA').click({delay:100});
        await frame.waitForFunction(()=>document.querySelector('#scName').textContent!=='Boot');
        result.interaction={before,after:await frame.locator('#scName').textContent()};
      } else if(id==='tabula-workbench') {
        const input=frame.getByRole('spinbutton',{name:'Inputs!B2',exact:true});
        await input.fill('421');await input.press('Tab');
        await frame.waitForFunction(()=>window.__tabula.S.session.get('Inputs!B2')===421);
        await page.waitForTimeout(1000);await page.reload();await page.waitForTimeout(2500);
        frame=await(await page.locator('iframe').elementHandle()).contentFrame();
        const reloaded=await frame.evaluate(()=>window.__tabula.S.session.get('Inputs!B2'));
        assert.equal(reloaded,421);
        result.interaction={edited:421,reloaded,nodes:await frame.evaluate(()=>window.__tabula.S.build.nodes)};
        const restore=frame.getByRole('spinbutton',{name:'Inputs!B2',exact:true});await restore.fill('420');await restore.press('Tab');
      } else if(id==='lattice-deck') {
        await frame.getByRole('button',{name:'Spin faster and make it pulse',exact:true}).click();
        await frame.locator('.ls-msg-result').first().waitFor();
        result.interaction={results:await frame.locator('.ls-msg-result').allTextContents(),errors:await frame.locator('.ls-msg-error').allTextContents()};
        assert.equal(result.interaction.errors.length,0);
        assert.match(result.interaction.results.join(' '),/speed.*2\.4/);
      } else if(id==='rag-workbench') {
        await frame.waitForFunction(()=>!!window.__wb.lastResult);
        const top=await frame.evaluate(()=>window.__wb.lastResult.top[0]);
        assert.equal(top.doc,'Model Limits');
        await frame.locator('#file-in').setInputFiles(path.join(ticket,'various/poc-note.md'));
        await frame.waitForFunction(()=>window.__wb.uploaded.some(x=>x.name==='poc-note.md'));
        await page.waitForTimeout(1500);await page.reload();await page.waitForTimeout(3000);
        frame=await(await page.locator('iframe').elementHandle()).contentFrame();
        const uploaded=await frame.evaluate(()=>window.__wb.uploaded.map(x=>x.name));
        assert.ok(uploaded.includes('poc-note.md'));
        result.interaction={top,uploadedAfterReload:uploaded};
      }
      if(['tabula-workbench','lattice-deck','rag-workbench'].includes(id)) {
        result.sample=await frame.evaluate(async()=>{
          const start=Date.now();const sample=await window.claude.use('sample');
          const r=await sample([{role:'user',content:'Reply with exactly: pong'}],{modelTier:'quick',cache:false,signal:AbortSignal.timeout(45000)});
          return{text:r.text,ms:Date.now()-start,model:r.modelUsed??r.modelApplied,tier:r.modelTierApplied};
        });
        assert.match(result.sample.text,/pong/i);
      }
      await page.screenshot({path:path.join(ticket,'various/screenshots',id+'-production.png'),fullPage:true});
      result.errors=errors;result.failed=failed;
      assert.equal(errors.length,0,'Unexpected application error: '+errors.join(';'));
      results.push(result);console.log('Verified',id,JSON.stringify(result.interaction));
    } finally {await page.close();}
  }
  return results;
}
