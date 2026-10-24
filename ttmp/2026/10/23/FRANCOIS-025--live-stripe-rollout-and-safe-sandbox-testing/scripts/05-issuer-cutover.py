#!/usr/bin/env python3
"""Pre-launch checkpoint and exact issuer migration. Explicit --execute required.
Leaves applications held/stopped for the separate GitOps domain cutover.
"""
import argparse,json,subprocess,time,datetime
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--execute',action='store_true');a=p.parse_args()
if not a.execute:raise SystemExit('--execute required')
root=Path(__file__).resolve().parent.parent
stamp=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%d%H%M%S')
receipt={'startedAt':stamp,'phase':'holding','sqlite':[],'postgres':[],'backupJobs':[]}
def k(*args):return subprocess.check_output(['kubectl',*args],text=True)
def save(): (root/'sources/issuer-cutover.json').write_text(json.dumps(receipt,indent=2)+'\n')
save()
for name in ['francois','francois-sandbox','agentlogic','datadrop','zitadel','zitadel-prereqs']:
 k('-n','argocd','patch','application',name,'--type=merge','-p','{"spec":{"syncPolicy":{"automated":null}}}')
k('-n','zitadel','patch','cronjob','zitadel-branding-reconciler','--type=merge','-p','{"spec":{"suspend":true}}')
for ns,name in [('francois','francois'),('francois-sandbox','francois'),('agentlogic','agentlogic'),('datadrop','datadrop'),('zitadel','zitadel-login'),('zitadel','zitadel')]:
 k('-n',ns,'scale','deployment',name,'--replicas=0')
for ns in ['francois','francois-sandbox','agentlogic','datadrop']:
 k('-n',ns,'wait','--for=delete','pod','-l','app.kubernetes.io/component=app','--timeout=90s')
k('-n','zitadel','wait','--for=delete','pod','-l','app.kubernetes.io/component in (start,login)','--timeout=90s')
for ns,cron in [('postgres','postgres-backup'),('vault','vault-backup')]:
 job='auth-domain-'+ns+'-'+stamp;k('-n',ns,'create','job','--from=cronjob/'+cron,job);receipt['backupJobs'].append({'namespace':ns,'job':job});save()
for item in receipt['backupJobs']:
 ns,job=item['namespace'],item['job'];k('-n',ns,'wait','--for=condition=complete','--timeout=180s','job/'+job)
 logs=k('-n',ns,'logs','job/'+job)
 import re
 found=re.findall(r'(?:postgres/postgres-[0-9TZ]+\.sql\.gz|vault/vault-[0-9TZ]+\.snap\.gz)',logs)
 if not found:raise RuntimeError('Backup succeeded but object receipt not found')
 item['object']=found[-1];save()
old='https://zitadel.yolo.scapegoat.dev';new='https://auth.slop.boutique'
for db in ['francois','francois_sandbox']:
 sql=f'''BEGIN;
DO $$ BEGIN IF EXISTS(SELECT 1 FROM users o JOIN users n ON o.subject=n.subject WHERE o.issuer='{old}' AND n.issuer='{new}') THEN RAISE EXCEPTION 'issuer collision'; END IF; END $$;
SELECT count(*) AS users_before FROM users;
UPDATE users SET issuer='{new}' WHERE issuer='{old}';
SELECT count(*) AS users_after FROM users;
COMMIT;'''
 cmd=['kubectl','-n','postgres','exec','-i','postgres-0','--','sh','-ec','export PGPASSWORD="$POSTGRES_PASSWORD"; exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d '+db]
 r=subprocess.run(cmd,input=sql,text=True,capture_output=True,check=True);receipt['postgres'].append({'database':db,'result':r.stdout.strip()});save()
for ns,db in [('agentlogic','agentlogic.db'),('datadrop','datadrop.db')]:
 code=f'''import sqlite3,json,os
p='/data/{db}';b='/data/{db}.before-auth-domain-{stamp}'
c=sqlite3.connect(p);out=sqlite3.connect(b);c.backup(out);assert out.execute('PRAGMA integrity_check').fetchone()[0]=='ok';out.close();os.chmod(b,0o600)
c.execute('BEGIN IMMEDIATE')
old={old!r};new={new!r}
assert c.execute('SELECT count(*) FROM users o JOIN users n ON o.subject=n.subject WHERE o.issuer=? AND n.issuer=?',(old,new)).fetchone()[0]==0
before=c.execute('SELECT id,subject FROM users ORDER BY id').fetchall();n=c.execute('UPDATE users SET issuer=? WHERE issuer=?',(new,old)).rowcount
assert before==c.execute('SELECT id,subject FROM users ORDER BY id').fetchall();c.commit();assert c.execute('PRAGMA integrity_check').fetchone()[0]=='ok';c.close()
print(json.dumps({{'database':p,'backup':b,'users':len(before),'updated':n,'identitiesPreserved':True}}))'''
 job='issuer-mapping-'+stamp
 manifest={'apiVersion':'batch/v1','kind':'Job','metadata':{'name':job,'namespace':ns},'spec':{'backoffLimit':0,'template':{'spec':{'restartPolicy':'Never','automountServiceAccountToken':False,'containers':[{'name':'migrate','image':'python:3.13.2-alpine3.21','command':['python','-c',code],'volumeMounts':[{'name':'data','mountPath':'/data'}]}],'volumes':[{'name':'data','persistentVolumeClaim':{'claimName':ns+'-data'}}]}}}}
 subprocess.run(['kubectl','apply','-f','-'],input=json.dumps(manifest),text=True,check=True,stdout=subprocess.DEVNULL)
 k('-n',ns,'wait','--for=condition=complete','--timeout=90s','job/'+job)
 receipt['sqlite'].append(json.loads(k('-n',ns,'logs','job/'+job)));save();k('-n',ns,'delete','job',job)
receipt['phase']='backed-up-mappings-migrated-awaiting-gitops';save();print(json.dumps(receipt,indent=2))
