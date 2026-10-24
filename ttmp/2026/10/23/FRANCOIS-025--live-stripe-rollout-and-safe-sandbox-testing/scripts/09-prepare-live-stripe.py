#!/usr/bin/env python3
"""Prepare live Stripe config only. Never updates mounted runtime, charges or customers.
Each resource ID is checkpointed to Vault; webhook is kept disabled until cutover.
"""
import json,subprocess,urllib.request,urllib.parse,urllib.error,re
from pathlib import Path
root=Path(__file__).resolve().parent.parent
path='kv/apps/francois/prod/stripe-live'
def vault(p):return json.loads(subprocess.check_output(['vault','kv','get','-format=json',p]))['data']
st=vault(path);s=st['data'];key=s['secret-key'];assert key.startswith('rk_live_')
def save():
 global st
 subprocess.run(['vault','kv','put','-format=json','-cas='+str(st['metadata']['version']),path,'-'],input=json.dumps(s),text=True,check=True,stdout=subprocess.DEVNULL)
 st=vault(path)
def api(method,endpoint,body=None,identity=None,credential=key):
 h={'Authorization':'Bearer '+credential,'Stripe-Version':'2026-06-24.dahlia'}
 if identity:h['Idempotency-Key']='francois-live-v1-'+identity
 req=urllib.request.Request('https://api.stripe.com/v1/'+endpoint,headers=h,method=method,data=urllib.parse.urlencode(body).encode() if body is not None else None)
 try:
  with urllib.request.urlopen(req,timeout=30) as r:return json.load(r)
 except urllib.error.HTTPError as e:
  m=json.load(e).get('error',{}).get('message','request failed')
  raise RuntimeError('Stripe HTTP '+str(e.code)+': '+re.sub(r'(?:rk|sk|pk)_(?:test|live)_\S+','[REDACTED]',m)) from None
account=api('GET','account');assert account['id']==s['account-id']=='acct_1TxEMNLpxiqBMHHi'
assert account['charges_enabled'] and account['payouts_enabled'] and account['capabilities']['card_payments']=='active'
oldkey=vault('kv/apps/francois/prod/runtime')['data']['stripe-secret-key'];assert oldkey.startswith(('sk_test_','rk_test_'))
oldprice=api('GET','prices/price_1UPxdFLeHmcfmvuCg0zsScj5',credential=oldkey)
assert not oldprice['livemode'] and oldprice['unit_amount']==100 and oldprice['currency']=='usd' and oldprice['recurring']['interval']=='month'
if 'product-id' not in s:
 obj=api('POST','products',{'name':'Slop Boutique subscriber','metadata[francois_deployment]':'francois.slop.boutique'},'product');assert obj['livemode'];s['product-id']=obj['id'];save()
if 'price-id' not in s:
 obj=api('POST','prices',{'product':s['product-id'],'unit_amount':str(oldprice['unit_amount']),'currency':oldprice['currency'],'recurring[interval]':'month','recurring[interval_count]':str(oldprice['recurring']['interval_count']),'lookup_key':'francois-live-subscriber-usd-monthly','metadata[environment]':'production'},'price');assert obj['livemode'];s['price-id']=obj['id'];save()
events=[x.strip() for x in (root.parents[4]/'pkg/catalog/payments/webhook-events.txt').read_text().strip().split(',') if x.strip()]
if 'webhook-id' not in s:
 body={'url':'https://slop.boutique/webhooks/stripe','api_version':'2026-06-24.dahlia','description':'Slop Boutique production (staged for live cutover)'}
 body.update({f'enabled_events[{i}]':e for i,e in enumerate(events)})
 obj=api('POST','webhook_endpoints',body,'webhook');assert obj['livemode'];s.update({'webhook-id':obj['id'],'webhook-secret':obj['secret']});save()
wh=api('POST','webhook_endpoints/'+s['webhook-id'],{'disabled':'true'})
if 'portal-id' not in s:
 configs=api('GET','billing_portal/configurations?limit=100')['data']
 if configs:raise RuntimeError('Existing live portal configuration needs explicit reconciliation')
 body={'business_profile[headline]':'slop.boutique','business_profile[privacy_policy_url]':'https://slop.boutique/privacy','business_profile[terms_of_service_url]':'https://slop.boutique/terms','default_return_url':'https://slop.boutique/account','features[subscription_cancel][enabled]':'true','features[subscription_cancel][mode]':'at_period_end','features[payment_method_update][enabled]':'true','features[invoice_history][enabled]':'true','metadata[francois_deployment]':'francois.slop.boutique'}
 obj=api('POST','billing_portal/configurations',body,'portal');assert obj['livemode'];s['portal-id']=obj['id'];save()
price=api('GET','prices/'+s['price-id']);portal=api('GET','billing_portal/configurations/'+s['portal-id']);wh=api('GET','webhook_endpoints/'+s['webhook-id'])
assert price['livemode'] and price['unit_amount']==oldprice['unit_amount'] and price['currency']==oldprice['currency'] and price['recurring']['interval']==oldprice['recurring']['interval']
assert portal['livemode'] and portal['active'] and portal['is_default']
assert wh['livemode'] and wh['status']=='disabled' and sorted(wh['enabled_events'])==sorted(events)
receipt={'account':{k:account[k] for k in ['id','country','default_currency','charges_enabled','payouts_enabled']},'cardPayments':account['capabilities']['card_payments'],'product':s['product-id'],'price':{k:price[k] for k in ['id','livemode','unit_amount','currency','recurring']},'portal':{k:portal[k] for k in ['id','livemode','active','is_default','default_return_url','features']},'webhook':{k:wh[k] for k in ['id','livemode','url','api_version','status','enabled_events']},'runtimeChanged':False,'chargesCreated':False}
(root/'sources/live-stripe-preparation.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt,indent=2))
