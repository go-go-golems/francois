// Assert the live cache contract after scripts/01 writes cache-after.json.
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import assert from 'node:assert/strict';
const ticket=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const measurement=JSON.parse(readFileSync(path.join(ticket,'various/cache-after.json'),'utf8'));
const base=measurement.base;
const result={at:new Date().toISOString(),base,visits:[],assets:[]};
for(const visit of measurement.visits){
  const assets=visit.responses.filter(r=>r.path.startsWith('/static/'));
  assert.ok(assets.length>0);
  for(const r of assets){
    assert.match(r.headers['cache-control'],/immutable/);
    assert.ok(r.headers.etag);
    if(visit.pass==='warm')assert.equal(r.wireBytes,0,'Warm static response must transfer no bytes: '+r.path);
    if(visit.pass==='cold'&&r.path.endsWith('.js'))assert.equal(r.headers['content-encoding'],'gzip');
  }
  result.visits.push({route:visit.route,pass:visit.pass,staticWireBytes:assets.reduce((n,r)=>n+(r.wireBytes||0),0),totalWireBytes:visit.responses.reduce((n,r)=>n+(r.wireBytes||0),0)});
}
const html=await(await fetch(base+'/account')).text();
const url=html.match(/src="(\/static\/catalog\.js\?v=[a-f0-9]{64})"/)[1];
const identity=await fetch(base+url,{headers:{'Accept-Encoding':'identity'}});
assert.equal(identity.status,200);
const bytes=Buffer.from(await identity.arrayBuffer());
const gzip=await fetch(base+url,{headers:{'Accept-Encoding':'gzip'}});
assert.equal(gzip.status,200);assert.equal(gzip.headers.get('content-encoding'),'gzip');
assert.deepEqual(Buffer.from(await gzip.arrayBuffer()),bytes);
assert.notEqual(identity.headers.get('etag'),gzip.headers.get('etag'));
const unchanged=await fetch(base+url,{headers:{'Accept-Encoding':'gzip','If-None-Match':gzip.headers.get('etag')}});
assert.equal(unchanged.status,304);assert.equal((await unchanged.arrayBuffer()).byteLength,0);
const head=await fetch(base+url,{method:'HEAD',headers:{'Accept-Encoding':'identity'}});
assert.equal(head.status,200);assert.equal((await head.arrayBuffer()).byteLength,0);
const bare=await fetch(base+'/static/catalog.js',{headers:{'Accept-Encoding':'identity'}});
assert.equal(bare.headers.get('cache-control'),'public, no-cache');await bare.arrayBuffer();
const session=await fetch(base+'/api/session');
assert.equal(session.headers.get('cache-control'),'no-store');await session.arrayBuffer();
const page=await(await fetch(base+'/apps/lattice-deck')).text();
assert.match(page,/<img[^>]+loading="eager"[^>]+fetchpriority="high"/);
result.assets.push({path:url,identityBytes:bytes.length,gzipBytes:Number(gzip.headers.get('content-length')),identityETag:identity.headers.get('etag'),gzipETag:gzip.headers.get('etag'),sha256:createHash('sha256').update(bytes).digest('hex'),conditionalStatus:304,headStatus:200});
result.sessionNoStore=true;result.eagerCover=true;
writeFileSync(path.join(ticket,'various/cache-contract-verification.json'),JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result));
