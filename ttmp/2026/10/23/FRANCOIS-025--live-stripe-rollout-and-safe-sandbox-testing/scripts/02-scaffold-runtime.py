#!/usr/bin/env python3
"""Create a standalone sandbox package, never patch production manifests."""
from pathlib import Path
import yaml,copy,json
repo=Path('/home/manuel/code/wesen/2026-03-27--hetzner-k3s-zitadel-upgrade')
src=repo/'gitops/kustomize/francois';dst=repo/'gitops/kustomize/francois-sandbox';dst.mkdir(exist_ok=True)
files=['namespace.yaml','serviceaccount.yaml','db-bootstrap-serviceaccount.yaml','vault-connection.yaml','vault-auth.yaml','db-bootstrap-vault-auth.yaml','runtime-secret.yaml','fonts-secret.yaml','postgres-admin-secret.yaml','db-bootstrap-script-configmap.yaml','db-bootstrap-job.yaml','persistentvolumeclaim.yaml','deployment.yaml','service.yaml','network-policy.yaml']
def save(name,d): (dst/name).write_text(yaml.safe_dump(d,sort_keys=False))
for name in files:
 d=yaml.safe_load((src/name).read_text())
 if name=='namespace.yaml':d['metadata']['name']='francois-sandbox'
 if name in ['vault-auth.yaml','db-bootstrap-vault-auth.yaml']:
  d['spec']['kubernetes']['role']='francois-sandbox'+('-db-bootstrap' if name.startswith('db-') else '')
  d['spec']['kubernetes']['audiences']=['vault']
 if name=='runtime-secret.yaml':d['spec']['path']='apps/francois/sandbox/runtime'
 if name=='persistentvolumeclaim.yaml':d['spec']['resources']['requests']['storage']='2Gi'
 if name=='deployment.yaml':
  pod=d['spec']['template']['spec'];c=pod['containers'][0]
  c['image']='ghcr.io/go-go-golems/francois:sha-dff295d'
  c.pop('command',None)
  c['args']=['catalog','serve','--listen=0.0.0.0:8080','--public-url=https://sandbox.slop.boutique','--app-domain=sandbox.yolo.scapegoat.dev','--legal-contact=wesen3000@googlemail.com','--trust-proxy-headers','--hsts-max-age=24h','--font-dir=/etc/francois/fonts','--migrate','--database-url-file=/run/secrets/francois/database-url','--release-dir=/var/lib/francois/releases','--worker-dir=/var/lib/francois/worker','--oidc-issuer=https://zitadel.yolo.scapegoat.dev','--oidc-client-id-file=/run/secrets/francois/oidc-client-id','--oidc-extra-scopes=urn:zitadel:iam:org:id:$(FRANCOIS_ZITADEL_ORG_ID)','--stripe-secret-key-file=/run/secrets/francois/stripe-secret-key','--stripe-webhook-secret-files=/run/secrets/francois/stripe-webhook-secret','--stripe-live-mode=false','--backend=fake']
  for probe in ['startupProbe','readinessProbe','livenessProbe']:c[probe]['httpGet']['httpHeaders'][0]['value']='sandbox.slop.boutique'
  c['resources']={'requests':{'cpu':'100m','memory':'128Mi'},'limits':{'cpu':'500m','memory':'512Mi'}}
  c['volumeMounts']=[v for v in c['volumeMounts'] if v['name']!='pi']
  pod['volumes']=[v for v in pod['volumes'] if v['name']!='pi']
  for v in pod['volumes']:
   if v['name']=='runtime':v['secret']['items']=[x for x in v['secret']['items'] if x['key'] not in ['zitadel-login-token','stripe-webhook-secret-previous']]
 save(name,d)
for name,issuer,hosts in [('portal-tls','letsencrypt-prod',['sandbox.slop.boutique']),('apps-tls','letsencrypt-prod-dns01-digitalocean',['*.sandbox.yolo.scapegoat.dev'])]:
 save(name+'.yaml',{'apiVersion':'cert-manager.io/v1','kind':'Certificate','metadata':{'name':name,'annotations':{'argocd.argoproj.io/sync-wave':'2'}},'spec':{'secretName':name,'issuerRef':{'name':issuer,'kind':'ClusterIssuer'},'dnsNames':hosts}});files.append(name+'.yaml')
base=yaml.safe_load((src/'ingress.yaml').read_text())
base['spec']['tls']=[{'hosts':['sandbox.slop.boutique'],'secretName':'portal-tls'},{'hosts':['*.sandbox.yolo.scapegoat.dev'],'secretName':'apps-tls'}]
rules=[]
for host in ['sandbox.slop.boutique','*.sandbox.yolo.scapegoat.dev']:
 r=copy.deepcopy(base['spec']['rules'][0]);r['host']=host;rules.append(r)
base['spec']['rules']=rules
base['metadata']['annotations']['traefik.ingress.kubernetes.io/router.priority']='100'
base['metadata']['annotations']['traefik.ingress.kubernetes.io/router.middlewares']='francois-sandbox-operator-gate@kubernetescrd,francois-sandbox-sandbox-headers@kubernetescrd'
save('ingress.yaml',base);files.append('ingress.yaml')
w=copy.deepcopy(base);w['metadata']['name']='stripe-webhook';w['metadata']['annotations'].pop('traefik.ingress.kubernetes.io/router.middlewares');w['metadata']['annotations']['traefik.ingress.kubernetes.io/router.priority']='200';w['spec']['rules']=[copy.deepcopy(rules[0])];w['spec']['rules'][0]['http']['paths'][0].update(path='/webhooks/stripe',pathType='Exact');w['spec']['tls']=w['spec']['tls'][:1];save('webhook-ingress.yaml',w);files.append('webhook-ingress.yaml')
for name,spec in [('operator-gate',{'basicAuth':{'secret':'sandbox-ingress-auth','realm':'Slop SANDBOX - test payments only','removeHeader':True}}),('sandbox-headers',{'headers':{'customResponseHeaders':{'X-Robots-Tag':'noindex, nofollow','X-Francois-Environment':'sandbox'}}})]:
 save(name+'.yaml',{'apiVersion':'traefik.io/v1alpha1','kind':'Middleware','metadata':{'name':name,'annotations':{'argocd.argoproj.io/sync-wave':'2'}},'spec':spec});files.append(name+'.yaml')
v=yaml.safe_load((src/'runtime-secret.yaml').read_text());v['metadata']['name']='sandbox-ingress-auth';v['spec'].pop('rolloutRestartTargets',None);v['spec']['path']='apps/francois/sandbox/ingress';v['spec']['destination']['name']='sandbox-ingress-auth';v['spec']['destination']['transformation']={'excludeRaw':True,'excludes':['.*'],'templates':{'users':{'text':'{{ .Secrets.users }}'}}};save('ingress-secret.yaml',v);files.append('ingress-secret.yaml')
save('kustomization.yaml',{'apiVersion':'kustomize.config.k8s.io/v1beta1','kind':'Kustomization','namespace':'francois-sandbox','resources':files,'labels':[{'pairs':{'app.kubernetes.io/part-of':'francois-sandbox'}}]})
app=yaml.safe_load((repo/'gitops/applications/francois.yaml').read_text());app['metadata']['name']='francois-sandbox';app['metadata']['annotations']['scapegoat.dev/description']='Operator-only Slop sandbox: isolated data, test Stripe, fake models';app['metadata']['labels']['app.kubernetes.io/part-of']='demo-apps';app['spec']['project']='demo-apps';app['spec']['destination']['namespace']='francois-sandbox';app['spec']['source']['path']='gitops/kustomize/francois-sandbox';(repo/'gitops/applications/francois-sandbox.yaml').write_text(yaml.safe_dump(app,sort_keys=False))
for suffix,paths,sa in [('', ['apps/francois/sandbox/runtime','apps/francois/sandbox/ingress','apps/francois/prod/fonts'],'francois'),('-db-bootstrap',['apps/francois/sandbox/runtime','infra/postgres/cluster'],'francois-db-bootstrap')]:
 name='francois-sandbox'+suffix
 policy='\n'.join('path "kv/data/'+path+'" { capabilities = ["read"] }' for path in paths)+'\n'
 (repo/'vault/policies/kubernetes'/ (name+'.hcl')).write_text(policy)
 role={'bound_service_account_names':[sa],'bound_service_account_namespaces':['francois-sandbox'],'policies':[name],'ttl':'1h','audience':'vault'}
 (repo/'vault/roles/kubernetes'/(name+'.json')).write_text(json.dumps(role,indent=2)+'\n')
print('Created standalone sandbox package and scoped Vault policies; production untouched.')
