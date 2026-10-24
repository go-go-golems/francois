#!/usr/bin/env python3
"""Replay an actual test event; reject bad signatures/wrong mode. No live key accepted."""
import json,subprocess,urllib.request,urllib.error,time,hmac,hashlib
from pathlib import Path
s=json.loads(subprocess.check_output(['vault','kv','get','-format=json','kv/apps/francois/sandbox/stripe']))['data']['data']
assert s['secret-key'].startswith(('rk_test_','sk_test_'))
def stripe(path):
 req=urllib.request.Request('https://api.stripe.com/v1/'+path,headers={'Authorization':'Bearer '+s['secret-key']})
 return json.load(urllib.request.urlopen(req))
events=stripe('events?type=checkout.session.completed&limit=10')['data']
e=next(e for e in events if e['data']['object'].get('client_reference_id') or e['data']['object'].get('metadata',{}).get('user_id')) if events else None
if e is None and events:e=events[0]
assert e and not e['livemode'] and not e['data']['object']['livemode']
def send(event,valid=True):
 body=json.dumps(event,separators=(',',':')).encode();t=str(int(time.time()));sig=hmac.new(s['webhook-secret'].encode(),t.encode()+b'.'+body,hashlib.sha256).hexdigest() if valid else '0'*64
 req=urllib.request.Request('https://sandbox.slop.boutique/webhooks/stripe',data=body,headers={'Content-Type':'application/json','Stripe-Signature':f't={t},v1={sig}'})
 try:
  with urllib.request.urlopen(req) as r:return r.status
 except urllib.error.HTTPError as r:return r.code
results={'event':e['id'],'originalLivemode':e['livemode'],'duplicateReplay':send(e),'invalidSignature':send(e,False)}
e['id']='evt_sandbox_wrong_mode_probe_'+str(int(time.time()));e['livemode']=True
results['wrongMode']=send(e)
assert results['duplicateReplay']==200 and 400<=results['invalidSignature']<500 and 400<=results['wrongMode']<500,results
out=Path(__file__).resolve().parent.parent/'sources/sandbox-webhook-checks.json';out.write_text(json.dumps(results,indent=2)+'\n');print(json.dumps(results))
