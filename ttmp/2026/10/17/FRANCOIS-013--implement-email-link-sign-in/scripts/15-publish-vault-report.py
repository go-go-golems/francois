#!/usr/bin/env python3
"""Copy the completed report and its local-test figures into the parc vault."""
import json
import pathlib

root=pathlib.Path(__file__).resolve().parent.parent
report=root/'reference/02-email-only-signup-production-project-report.md'
body=report.read_text().split('---',2)[2].lstrip()
body=body.replace('../sources/implementation/email-signup-','_assets/francois-email-signup-')
if '@@PRODUCTION_RESULT@@' in body:
    raise SystemExit('Refusing to publish before observed production results are inserted')
receipt=json.loads((root/'sources/implementation/email-signup-production.json').read_text())
folder=pathlib.Path('/home/manuel/code/wesen/go-go-golems/go-go-parc/Projects/2026/10/17')
destination=folder/'PROJ - Francois - Passwordless Email Signup and Production Delivery - A Technical Deep Dive.md'
if destination.exists():
    raise SystemExit('Refusing to overwrite an existing historical vault report')
metadata={
    'title':'Francois: Passwordless Email Signup and Production Delivery',
    'aliases':['FRANCOIS-013 Email Signup Report','Francois Email Registration'],
    'tags':['project','go','authentication','zitadel','kubernetes'],
    'status':'active','type':'project','created':'2026-10-17',
    'repo':'/home/manuel/code/wesen/go-go-golems/francois','ticket':'FRANCOIS-013',
    'pr':'https://github.com/go-go-golems/francois/pull/13',
    'runtime_image':receipt['image'],'gitops_revision':receipt['argo']['revision'],
}
front='---\n'+'\n'.join(key+': '+json.dumps(value) for key,value in metadata.items())+'\n---\n\n'
assets=folder/'_assets';assets.mkdir(parents=True,exist_ok=True)
for name in ['account-1440','account-390','confirm-1440','production-signin-1440','production-signin-390']:
    source=root/('sources/implementation/email-signup-'+name+'.png')
    target=assets/('francois-email-signup-'+name+'.png')
    if target.exists() and target.read_bytes()!=source.read_bytes():
        raise SystemExit('Refusing to replace an existing different figure')
    target.write_bytes(source.read_bytes())
destination.write_text(front+body)
print(destination)
print('Words:',len(body.split()))
