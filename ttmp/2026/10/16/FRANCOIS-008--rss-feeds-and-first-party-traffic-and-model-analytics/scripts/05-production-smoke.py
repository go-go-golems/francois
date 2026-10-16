import json,urllib.request,urllib.error,xml.etree.ElementTree as ET
checks=[]
def get(url):
 req=urllib.request.Request(url,headers={'User-Agent':'curl/francois-deployment-check'})
 try:
  with urllib.request.urlopen(req,timeout=20) as r: return r.status,r.read()
 except urllib.error.HTTPError as r: return r.code,r.read()
for origin in ['https://francois.yolo.scapegoat.dev','https://slop.boutique']:
 for path in ['/','/apps','/journal','/privacy','/static/catalog.js']:
  code,body=get(origin+path)
  assert code==200,(origin,path,code)
  checks.append({'url':origin+path,'status':code})
 code,body=get(origin+'/readyz')
 ready=json.loads(body)
 assert code==200 and ready['status']=='ready' and ready['models'] is True,(origin,code,ready)
 checks.append({'url':origin+'/readyz','status':code,'models':True})
 code,body=get(origin+'/healthz')
 assert code==200,(origin,code)
 checks.append({'url':origin+'/healthz','status':code})
 for path in ['/feed.xml','/apps/feed.xml','/journal/feed.xml']:
  code,body=get(origin+path)
  assert code==200,(origin,path,code)
  xml=ET.fromstring(body)
  assert xml.tag=='rss',(origin,path,xml.tag)
  for item in xml.findall('./channel/item'):
   assert item.findtext('link').startswith(origin+'/'),(origin,path,'wrong site link')
  if path=='/feed.xml':
   assert xml.findtext('./channel/link')==origin+'/'
   for item in xml.findall('./channel/item'):
    guid=item.find('guid')
    assert guid is not None and guid.attrib.get('isPermaLink')=='false'
  checks.append({'url':origin+path,'status':code,'items':len(xml.findall('./channel/item'))})
 code,body=get(origin+'/api/admin/analytics')
 assert code in [401,403],(origin,'anonymous analytics access',code)
 checks.append({'url':origin+'/api/admin/analytics','anonymous_status':code})
print(json.dumps({'checks_passed':len(checks),'checks':checks},indent=2))
