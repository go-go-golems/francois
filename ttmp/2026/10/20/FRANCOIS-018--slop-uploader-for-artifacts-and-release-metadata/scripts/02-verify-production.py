#!/usr/bin/env python3
"""Read-only rollout receipt: exact image, Argo state and public asset checks."""
import datetime, hashlib, json, subprocess
from pathlib import Path
from urllib.request import urlopen
from urllib.error import HTTPError
KUBE = '/home/manuel/code/wesen/2026-03-27--hetzner-k3s/.cache/kubeconfig-tailnet.yaml'
def resource(namespace, kind, name):
    return json.loads(subprocess.check_output(['kubectl','--kubeconfig',KUBE,'-n',namespace,'get',kind,name,'-o','json']))
def fetch(base, path, expected=200):
    try:
        with urlopen(base+path, timeout=30) as response:
            assert response.status == expected, (base,path,response.status)
            return response.read()
    except HTTPError as error:
        assert error.code == expected, (base,path,error.code)
        return b''
expected='ghcr.io/go-go-golems/francois:sha-21b8458'
deployment=resource('francois','deployment','francois')
image=next(c['image'] for c in deployment['spec']['template']['spec']['containers'] if c['name']=='francois')
assert image==expected,(image,expected)
assert deployment['status'].get('updatedReplicas')==1 and deployment['status'].get('readyReplicas')==1
app=resource('argocd','application','francois')
assert app['status']['sync']['status']=='Synced'
assert app['status']['health']['status']=='Healthy'
checks=[]
for base in ['https://slop.boutique','https://francois.yolo.scapegoat.dev']:
    for path in ['/healthz','/readyz','/','/pricing','/studio/upload','/signin']:
        fetch(base,path)
    assert b'<rss' in fetch(base,'/feed.xml')
    session=json.loads(fetch(base,'/api/session'))
    assert not session['authenticated']
    assets={}
    for filename in ['catalog.js','catalog.css']:
        deployed=fetch(base,'/static/'+filename)
        local=Path('pkg/catalog/web/static',filename).read_bytes()
        assert deployed==local, (base, filename, 'served asset differs from merged source')
        assets[filename]=hashlib.sha256(deployed).hexdigest()
    assert b'francoisRoutePosition' in fetch(base,'/static/catalog.js')
    checks.append({'site':base,'health':True,'uploaderShell':True,'exactAssets':assets,'authenticated':False})
receipt={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceCommit':'21b8458cbe75cb34e2219d75bd6536a5f38df80d','image':image,'readyReplicas':1,'argoRevision':app['status']['sync']['revision'],'argoSync':'Synced','argoHealth':'Healthy','checks':checks,'productionMutationsInSmokeTest':False}
out=Path(__file__).resolve().parent.parent/'sources/production-rollout.json'
out.write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps(receipt,indent=2))
