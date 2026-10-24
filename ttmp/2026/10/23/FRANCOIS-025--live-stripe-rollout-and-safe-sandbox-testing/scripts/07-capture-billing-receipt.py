#!/usr/bin/env python3
"""Read-only sanitized end-of-test DB receipt; never exports session URLs or PII."""
import subprocess,json,datetime
from pathlib import Path
sql="""SELECT json_build_object('subscriptions',(SELECT json_agg(row_to_json(x)) FROM (SELECT status,livemode,support_amount_cents,support_tier,cancel_at_period_end FROM billing_subscriptions ORDER BY support_amount_cents)x),'events',(SELECT json_agg(row_to_json(x)) FROM (SELECT type,state,error_code,count(*) FROM stripe_events GROUP BY 1,2,3 ORDER BY 1)x),'checkouts',(SELECT json_agg(row_to_json(x)) FROM (SELECT price,state,payment_state FROM checkout_attempts ORDER BY created_at)x),'wrongModeRows',(SELECT count(*) FROM stripe_events WHERE event_id LIKE 'evt_sandbox_wrong_mode_probe_%'),'replayedEventRows',(SELECT count(*) FROM stripe_events WHERE event_id='evt_1UTzspLpxiqBMHHihrsh4kJ4'));"""
r=subprocess.run(['kubectl','-n','postgres','exec','-i','postgres-0','--','sh','-ec','export PGPASSWORD="$POSTGRES_PASSWORD"; exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d francois_sandbox'],input=sql,text=True,capture_output=True)
if r.returncode:raise SystemExit(r.stderr)
d=json.loads(r.stdout)
assert all(x['status']=='canceled' and not x['livemode'] for x in d['subscriptions'])
assert all(x['state']=='processed' and not x['error_code'] for x in d['events'])
assert d['wrongModeRows']==0 and d['replayedEventRows']==1
assert any(x['payment_state']=='refunded' for x in d['checkouts'])
receipt={'capturedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'image':'ghcr.io/go-go-golems/francois:sha-dff295d','database':d,'livePayments':False}
p=Path(__file__).resolve().parent.parent/'sources/sandbox-billing-qualification.json';p.write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(d,indent=2))
