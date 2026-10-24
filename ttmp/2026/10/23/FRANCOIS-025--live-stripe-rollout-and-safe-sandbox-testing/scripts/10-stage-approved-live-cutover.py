#!/usr/bin/env python3
"""One-shot approved production test-billing archive + live runtime staging.
Leaves deployment stopped/reconciliation held for merging the live-mode manifest.
Never replay after production is live; rejects non-test current credentials.
"""
import json,subprocess,urllib.request,urllib.parse,datetime,base64,time
from pathlib import Path
root=Path(__file__).resolve().parent.parent
stamp=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%d%H%M%S')
archive='billing_test_archive_'+stamp
receipt={'startedAt':stamp,'archiveSchema':archive,'phase':'preflight'}
def save(): (root/'sources/live-cutover.json').write_text(json.dumps(receipt,indent=2)+'\n')
def k(*a):return subprocess.check_output(['kubectl',*a],text=True)
def vget(p):return json.loads(subprocess.check_output(['vault','kv','get','-format=json',p]))['data']
def vput(p,version,data):subprocess.run(['vault','kv','put','-format=json','-cas='+str(version),p,'-'],input=json.dumps(data),text=True,check=True,stdout=subprocess.DEVNULL)
def sql(q):
 r=subprocess.run(['kubectl','-n','postgres','exec','-i','postgres-0','--','sh','-ec','export PGPASSWORD="$POSTGRES_PASSWORD"; exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d francois'],input=q,text=True,capture_output=True)
 if r.returncode:raise RuntimeError(r.stderr)
 return r.stdout.strip()
def stripe(key,method,path,body=None):
 req=urllib.request.Request('https://api.stripe.com/v1/'+path,headers={'Authorization':'Bearer '+key,'Stripe-Version':'2026-06-24.dahlia'},method=method,data=urllib.parse.urlencode(body).encode() if body is not None else None)
 with urllib.request.urlopen(req,timeout=30) as r:return json.load(r)
old=vget('kv/apps/francois/prod/runtime');live=vget('kv/apps/francois/prod/stripe-live')['data']
assert old['data']['stripe-secret-key'].startswith(('sk_test_','rk_test_')) and live['secret-key'].startswith('rk_live_')
assert live['account-id']=='acct_1TxEMNLpxiqBMHHi'
price=stripe(live['secret-key'],'GET','prices/'+live['price-id']);assert price['livemode'] and price['unit_amount']==100 and price['currency']=='usd'
wh=stripe(live['secret-key'],'GET','webhook_endpoints/'+live['webhook-id']);assert wh['livemode'] and wh['status']=='disabled'
receipt['runtimeRollbackVaultPath']='kv/apps/francois/prod/pre-live-'+stamp
vput(receipt['runtimeRollbackVaultPath'],0,old['data']);save()
k('-n','argocd','patch','application','francois','--type=merge','-p','{"spec":{"syncPolicy":{"automated":null}}}')
k('-n','francois','scale','deployment','francois','--replicas=0')
k('-n','francois','wait','--for=delete','pod','-l','app.kubernetes.io/component=app','--timeout=90s')
receipt['phase']='stopped';save()
job='live-cutover-'+stamp;k('-n','postgres','create','job','--from=cronjob/postgres-backup',job);k('-n','postgres','wait','--for=condition=complete','job/'+job,'--timeout=180s')
import re
objects=re.findall(r'postgres/postgres-[0-9TZ]+\.sql\.gz',k('-n','postgres','logs','job/'+job));assert objects
receipt['postgresCheckpoint']=objects[-1];save()
protected="SELECT json_build_object('users',(SELECT count(*) FROM users),'artifacts',(SELECT count(*) FROM catalog_artifacts),'releases',(SELECT count(*) FROM releases),'modelCalls',(SELECT count(*) FROM model_calls),'dailyUsage',(SELECT count(*) FROM daily_token_usage));"
receipt['protectedBefore']=json.loads(sql(protected));save()
q=f"""BEGIN;
LOCK TABLE billing_customers,billing_subscriptions,checkout_attempts,stripe_events,platform_settings IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF (SELECT count(*) FROM billing_customers)<>4 OR (SELECT count(*) FROM billing_subscriptions)<>2 OR (SELECT count(*) FROM checkout_attempts)<>7 OR (SELECT count(*) FROM stripe_events)<>229 THEN RAISE EXCEPTION 'approved scope changed'; END IF;
 IF EXISTS(SELECT 1 FROM billing_subscriptions WHERE livemode) OR EXISTS(SELECT 1 FROM stripe_events WHERE livemode) THEN RAISE EXCEPTION 'live rows present'; END IF;
 IF (SELECT document#>>'{{plans,1,prices,0,priceId}}' FROM platform_settings ORDER BY revision DESC LIMIT 1)<>'price_1UPxdFLeHmcfmvuCg0zsScj5' THEN RAISE EXCEPTION 'price mapping changed'; END IF;
END $$;
CREATE SCHEMA {archive};
REVOKE ALL ON SCHEMA {archive} FROM PUBLIC;
CREATE TABLE {archive}.billing_customers AS TABLE billing_customers;
CREATE TABLE {archive}.billing_subscriptions AS TABLE billing_subscriptions;
CREATE TABLE {archive}.checkout_attempts AS TABLE checkout_attempts;
CREATE TABLE {archive}.stripe_events AS TABLE stripe_events;
CREATE TABLE {archive}.platform_settings AS TABLE platform_settings;
DELETE FROM stripe_events;
DELETE FROM checkout_attempts;
DELETE FROM billing_subscriptions;
DELETE FROM billing_customers;
INSERT INTO platform_settings(document,actor_id)
SELECT jsonb_set(document,'{{plans,1,prices,0,priceId}}',to_jsonb('{live['price-id']}'::text)),NULL FROM platform_settings ORDER BY revision DESC LIMIT 1;
COMMIT;
"""
receipt['archiveResult']=sql(q);receipt['protectedAfter']=json.loads(sql(protected));assert receipt['protectedBefore']==receipt['protectedAfter'];receipt['phase']='archived';save()
# Retire only this application's old test webhook destinations, never other apps.
oldhooks=stripe(old['data']['stripe-secret-key'],'GET','webhook_endpoints?limit=100')
assert not oldhooks.get('has_more'),'paginate old hooks before continuing'
receipt['disabledOldTestWebhooks']=[]
for item in oldhooks['data']:
 if item['url'] in ['https://slop.boutique/webhooks/stripe','https://francois.yolo.scapegoat.dev/webhooks/stripe']:
  assert not item['livemode'];stripe(old['data']['stripe-secret-key'],'POST','webhook_endpoints/'+item['id'],{'disabled':'true'});receipt['disabledOldTestWebhooks'].append(item['id']);save()
new=dict(old['data']);new.update({'stripe-secret-key':live['secret-key'],'stripe-webhook-secret':live['webhook-secret'],'stripe-webhook-secret-previous':'','stripe-price-id':live['price-id']})
vput('kv/apps/francois/prod/runtime',old['metadata']['version'],new)
for _ in range(45):
 data=json.loads(k('-n','francois','get','secret','francois-runtime','-o','json'))['data']
 if base64.b64decode(data['stripe-secret-key']).decode()==live['secret-key'] and base64.b64decode(data['stripe-webhook-secret']).decode()==live['webhook-secret']:
  break
 time.sleep(2)
else:raise RuntimeError('VSO runtime sync timeout; deployment remains stopped')
receipt['phase']='live-runtime-synced-awaiting-gitops';save();print(json.dumps(receipt,indent=2))
