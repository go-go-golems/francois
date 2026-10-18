#!/usr/bin/env python3
"""Fixed loopback fixture: an active verified identity with no email OTP."""
import json,pathlib,sys,time,urllib.request
root=pathlib.Path('/tmp/francois-013-private')
token=(root/'bootstrap.pat').read_text().strip()
org=json.loads((root/'fixture.json').read_text())['organization']
def api(method,path,body=None):
    request=urllib.request.Request('http://localhost:18083'+path,method=method,
        data=None if body is None else json.dumps(body).encode(),
        headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'})
    with urllib.request.urlopen(request,timeout=10) as response:return json.load(response)
if len(sys.argv)==2:
    subject=sys.argv[1]
    methods=api('GET','/v2/users/'+subject+'/authentication_methods')['authMethodTypes']
    assert 'AUTHENTICATION_METHOD_TYPE_OTP_EMAIL' not in methods
    print(json.dumps({'emailOTPEnrolled':False}))
else:
    email='provider-only-'+str(time.time_ns())+'@slop.test'
    subject=api('POST','/v2/users/human',{'username':email,'organization':{'orgId':org},
        'profile':{'givenName':'Existing','familyName':'Local'},'email':{'email':email,'isVerified':True},
        'password':{'password':'LocalMailboxProof1!','changeRequired':False}})['userId']
    methods=api('GET','/v2/users/'+subject+'/authentication_methods')['authMethodTypes']
    assert 'AUTHENTICATION_METHOD_TYPE_OTP_EMAIL' not in methods
    print(json.dumps({'subject':subject,'email':email,'emailOTPEnrolled':False}))
