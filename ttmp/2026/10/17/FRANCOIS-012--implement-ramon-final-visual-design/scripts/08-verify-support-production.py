#!/usr/bin/env python3
"""Read-only rollout verification. Never submits an address or saves secrets."""
import argparse
import datetime
import json
import pathlib
import subprocess
import urllib.request

KUBECONFIG='/home/manuel/code/wesen/2026-03-27--hetzner-k3s/.cache/kubeconfig-tailnet.yaml'
ROOT=pathlib.Path(__file__).resolve().parent.parent

def kube(namespace,*args):
    return subprocess.check_output(['kubectl','--kubeconfig',KUBECONFIG,'-n',namespace,*args],text=True)

def fetch(origin,path):
    with urllib.request.urlopen(origin+path,timeout=20) as response:
        body=response.read(262144).decode()
        return response.status,dict(response.headers),body

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--expected-image',required=True)
    expected=parser.parse_args().expected_image
    deploy=json.loads(kube('francois','get','deployment','francois','-o','json'))
    container=next(c for c in deploy['spec']['template']['spec']['containers'] if c['name']=='francois')
    assert container['image']==expected
    args=container['args']
    assert '--signup-allowed' in args and '--email-signin-enabled' in args and '--migrate' in args
    argo=json.loads(kube('argocd','get','application','francois','-o','json'))['status']
    assert argo['sync']['status']=='Synced' and argo['health']['status']=='Healthy'
    status=deploy['status'];assert status.get('observedGeneration',0)>=deploy['metadata']['generation']
    assert status.get('readyReplicas')==1 and status.get('updatedReplicas')==1
    migration=kube('postgres','exec','postgres-0','--','sh','-c',
        '''psql -U "$POSTGRES_USER" -d francois -Atc "SELECT name||'|'||applied_at FROM schema_migrations WHERE name='015_monthly_support.sql'"''').strip()
    assert migration.startswith('015_monthly_support.sql|')
    tables=kube('postgres','exec','postgres-0','--','sh','-c',
        'psql -U "$POSTGRES_USER" -d francois -Atc "SELECT to_regclass(\'public.email_links\') IS NOT NULL, to_regclass(\'public.email_signins\') IS NULL, to_regclass(\'public.email_registrations\') IS NULL"').strip()
    assert tables=='t|t|t',tables
    sites=[]
    for origin in ['https://slop.boutique','https://francois.yolo.scapegoat.dev']:
        checks=[]
        for path in ['/healthz','/readyz','/apps','/apps/lattice-deck','/feed.xml']:
            code,_,_=fetch(origin,path);assert code==200;checks.append({'path':path,'status':code})
        code,headers,body=fetch(origin,'/signin')
        # Header lookup is case-insensitive through a normalized projection.
        headers={k.lower():v for k,v in headers.items()}
        assert code==200 and headers.get('cache-control')=='no-store' and headers.get('referrer-policy')=='no-referrer'
        assert 'Continue with Google' in body and 'Continue with GitHub' in body and 'to create an account' in body
        assert 'fifteen minutes' in body
        assert 'maxlength="200"' in body and '<script' not in body
        _,_,pricing=fetch(origin,'/pricing')
        assert 'value="42"' in pricing and 'data-support-monthly' in pricing and '999,999.99 USD' in pricing
        code,_,_=fetch(origin,'/thank-you');assert code==200
        _,_,body=fetch(origin,'/api/session');session=json.loads(body)
        assert session['authenticated'] is False and session['signupAvailable'] is True
        sites.append({'origin':origin,'checks':checks,'pricing42USD':True,'thankYouRoute':True,'signinNoStore':True,'signinNoReferrer':True,
            'allMethodsAndSignupGuidance':True,'emailMaxLength':200,'signupAvailable':session['signupAvailable'],
            'journalEnabled':session.get('journalEnabled'),'authenticated':False})
    receipt={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'image':container['image'],
        'replicas':{'ready':status['readyReplicas'],'updated':status['updatedReplicas']},
        'argo':{'sync':argo['sync']['status'],'health':argo['health']['status'],
            'phase':argo.get('operationState',{}).get('phase'),'revision':argo['sync']['revision']},
        'migration':migration,'unifiedPendingTables':tables,'sites':sites,'productionCheckoutRequested':False,'productionEmailRequested':False,'credentialsSaved':False}
    (ROOT/'sources/implementation/support-production.json').write_text(json.dumps(receipt,indent=2)+'\n')
    print(json.dumps(receipt,indent=2))

if __name__=='__main__':
    main()
