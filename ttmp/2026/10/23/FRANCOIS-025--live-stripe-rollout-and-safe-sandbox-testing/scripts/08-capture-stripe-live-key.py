#!/usr/bin/env python3
"""One-request loopback bridge: browser live staging credential directly to Vault.
No stdout/body logging or persistent key files. Run only while provisioning.
"""
import http.server,json,subprocess
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_POST(self):
  if self.path!='/custody-17d99503-6f74-44d7-85e5-d64ca369a7d6':self.send_error(404);return
  size=int(self.headers.get('Content-Length','0'))
  if not 0<size<4096:self.send_error(400);return
  d=json.loads(self.rfile.read(size))
  if not d.get('secret-key','').startswith('rk_live_'):self.send_error(400);return
  r=subprocess.run(['vault','kv','put','-cas=0','-format=json','kv/apps/francois/prod/stripe-live','-'],input=json.dumps(d),text=True,capture_output=True)
  self.send_response(200 if r.returncode==0 else 500);self.end_headers();self.wfile.write(b'stored' if r.returncode==0 else b'Vault write failed')
  print('Vault custody '+('succeeded' if r.returncode==0 else 'failed'),flush=True)
s=http.server.HTTPServer(('127.0.0.1',18771),Handler);s.timeout=90
try:s.handle_request()
finally:s.server_close()
