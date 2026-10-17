#!/usr/bin/env python3
"""Save primary-source snapshots; run from any directory."""
from pathlib import Path
import urllib.request, hashlib, json, datetime
root=Path(__file__).resolve().parent.parent/'sources'/'zitadel-email-login'
root.mkdir(parents=True,exist_ok=True)
urls={
 '01-magic-links-discussion.html':'https://github.com/zitadel/zitadel/discussions/2075',
 '02-custom-login-oidc.html':'https://zitadel.com/docs/guides/integrate/login-ui/oidc-standard',
 '03-email-otp-and-mfa.html':'https://zitadel.com/docs/guides/integrate/login-ui/mfa',
 '04-create-session.html':'https://zitadel.com/docs/reference/api/session/zitadel.session.v2.SessionService.CreateSession',
 '05-hosted-login.html':'https://zitadel.com/docs/guides/integrate/login/hosted-login',
}
records=[]
for name,url in urls.items():
 req=urllib.request.Request(url,headers={'User-Agent':'Mozilla/5.0'})
 with urllib.request.urlopen(req,timeout=30) as response:
  data=response.read(); final=response.url
 (root/name).write_bytes(data)
 records.append(dict(file=name,url=url,resolved_url=final,retrieved_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),bytes=len(data),sha256=hashlib.sha256(data).hexdigest()))
 print(name,len(data))
(root/'manifest.json').write_text(json.dumps(records,indent=2)+'\n')
