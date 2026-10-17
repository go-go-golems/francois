#!/usr/bin/env python3
"""Provision ONLY the isolated 03-local-stack.yaml instance. No production mode."""
import datetime,json,pathlib,urllib.request,urllib.error
root=pathlib.Path('/tmp/francois-013-private')
issuer='http://localhost:18083'
bootstrap=(root/'bootstrap.pat').read_text().strip()
statefile=root/'fixture.json'
state=json.loads(statefile.read_text()) if statefile.exists() else {}
def save():
 statefile.write_text(json.dumps(state,indent=2)+'\n');statefile.chmod(0o600)
def api(method,path,body=None,token=bootstrap):
 req=urllib.request.Request(issuer+path,data=json.dumps(body).encode() if body is not None else None,method=method,headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'})
 try:
  with urllib.request.urlopen(req,timeout=10) as r:return json.load(r)
 except urllib.error.HTTPError as e:
  # This fixed-loopback fixture has no real customer data. Do not print tokens.
  detail=json.loads(e.read());raise RuntimeError(f'{method} {path}: HTTP {e.code}: '+str(detail.get('message','request failed'))) from None
if 'organization' not in state:
 state['organization']=api('GET','/management/v1/orgs/me')['org']['id'];save()
if 'smtp' not in state:
 state['smtp']=api('POST','/admin/v1/email/smtp',{'senderAddress':'signin@slop.test','senderName':'Slop Boutique local test','host':'mail:1025','tls':False,'description':'Isolated FRANCOIS-013 mail sink'})['id'];save()
 api('POST','/admin/v1/email/'+state['smtp']+'/_activate',{})
if 'service' not in state:
 state['service']=api('POST','/management/v1/users/machine',{'userName':'francois-email-login','name':'Francois email login'})['userId'];save()
 api('POST','/admin/v1/members',{'userId':state['service'],'roles':['IAM_LOGIN_CLIENT']})
if not (root/'login.pat').exists():
 response=api('POST','/v2/users/'+state['service']+'/pats',{'expirationDate':(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1)).isoformat()})
 p=root/'login.pat';p.write_text(response['token']);p.chmod(0o600)
if 'subject' not in state:
 response=api('POST','/v2/users/human',{'username':'supporter@slop.test','organization':{'orgId':state['organization']},'profile':{'givenName':'Local','familyName':'Supporter'},'email':{'email':'supporter@slop.test','isVerified':True},'password':{'password':'LocalEmailProof1!','changeRequired':False}})
 state['subject']=response['userId'];save()
 api('POST','/v2/users/'+state['subject']+'/otp_email',{})
print('Provisioned isolated ZITADEL v4.16.1, SMTP mail sink, dedicated IAM_LOGIN_CLIENT and verified supporter; private fixture state saved.')
