#!/usr/bin/env python3
"""Provision only sandbox Stripe objects and Vault runtime; no live API key accepted."""
import json,subprocess,urllib.request,urllib.parse,urllib.error,secrets
from pathlib import Path
root=Path(__file__).resolve().parent.parent
path='kv/apps/francois/sandbox/stripe'
def get(p):return json.loads(subprocess.check_output(['vault','kv','get','-format=json',p]))['data']
def put(p,v,data):subprocess.run(['vault','kv','put','-format=json','-cas='+str(v),p,'-'],input=json.dumps(data),text=True,check=True,stdout=subprocess.DEVNULL)
st=get(path);s=st['data'];key=s['secret-key'];assert key.startswith('rk_test_')
def stripe(method,url,body=None,identity=None):
 headers={'Authorization':'Bearer '+key,'Stripe-Version':'2026-06-24.dahlia'}
 if identity:headers['Idempotency-Key']='francois-sandbox-v1-'+identity
 req=urllib.request.Request('https://api.stripe.com/v1/'+url,data=urllib.parse.urlencode(body).encode() if body else None,headers=headers,method=method)
 try:
  with urllib.request.urlopen(req,timeout=30) as r:return json.load(r)
 except urllib.error.HTTPError as e:
  d=json.load(e);raise RuntimeError('Stripe HTTP '+str(e.code)+': '+d.get('error',{}).get('message','request failed')) from None
product=stripe('POST','products',{'name':'Slop sandbox subscriber','metadata[francois_deployment]':'francois-sandbox.slop.boutique'},'product');assert not product['livemode']
price=stripe('POST','prices',{'product':product['id'],'unit_amount':'100','currency':'usd','recurring[interval]':'month','lookup_key':'francois-sandbox-subscriber-usd-monthly','metadata[environment]':'sandbox'},'price');assert not price['livemode']
if 'webhook-secret' not in s:
 events=Path('/home/manuel/code/wesen/go-go-golems/francois-sandbox/pkg/catalog/payments/webhook-events.txt')
 body={'url':'https://sandbox.slop.boutique/webhooks/stripe','api_version':'2026-06-24.dahlia','description':'Isolated Slop sandbox webhook'}
 for i,event in enumerate(x.strip() for x in events.read_text().strip().split(',') if x.strip()):body[f'enabled_events[{i}]']=event
 wh=stripe('POST','webhook_endpoints',body,'webhook');assert not wh['livemode']
 s.update({'webhook-secret':wh['secret'],'webhook-id':wh['id'],'price-id':price['id'],'product-id':product['id']});put(path,st['metadata']['version'],s)
# Reuse already-custodied resources on retry; never replace database credentials.
runtime='kv/apps/francois/sandbox/runtime'
r=subprocess.run(['vault','kv','get','-format=json',runtime],capture_output=True,text=True)
if r.returncode!=0:
 password=secrets.token_hex(24)
 d={'database':'francois_sandbox','username':'francois_sandbox','password':password,'database-url':'postgres://francois_sandbox:'+password+'@postgres.postgres.svc.cluster.local:5432/francois_sandbox?sslmode=disable','oidc-client-id':'396448802724184162','oidc-org-id':'394966170567966836','stripe-secret-key':key,'stripe-webhook-secret':s['webhook-secret']}
 put(runtime,0,d)
ingress='kv/apps/francois/sandbox/ingress';r=subprocess.run(['vault','kv','get','-format=json',ingress],capture_output=True,text=True)
if r.returncode!=0:
 password=secrets.token_urlsafe(24);hashed=subprocess.check_output(['openssl','passwd','-apr1','-stdin'],input=password,text=True).strip()
 put(ingress,0,{'username':'sandbox','password':password,'users':'sandbox:'+hashed+'\n'})
receipt={'testOnly':True,'account':s['account-id'],'product':product['id'],'price':price['id'],'amount':100,'currency':'usd','interval':'month','webhook':s['webhook-id'],'portal':'https://sandbox.slop.boutique','database':'francois_sandbox','sharedProductionData':False}
(root/'sources').mkdir(exist_ok=True);(root/'sources/runtime-provisioning.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt,indent=2))
